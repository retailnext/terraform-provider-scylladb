// Copyright RetailNext, Inc. 2026

package provider

import (
	"context"
	"fmt"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/retailnext/terraform-provider-scylladb/scylladb"
)

var _ resource.Resource = &tableGrantsResource{}
var _ resource.ResourceWithConfigure = &tableGrantsResource{}
var _ resource.ResourceWithModifyPlan = &tableGrantsResource{}

func NewTableGrantsResource() resource.Resource {
	return &tableGrantsResource{}
}

type tableGrantsResource struct {
	client *scylladb.Cluster
}

type tableGrantsResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Keyspace    types.String `tfsdk:"keyspace"`
	Table       types.String `tfsdk:"table"`
	Grants      []grantModel `tfsdk:"grant"`
	Permissions types.List   `tfsdk:"permissions"`
}

func (r *tableGrantsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_table_grants"
}

func (r *tableGrantsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Authoritatively manages all grants on a table. Any existing grants not specified in `grant` blocks are revoked on apply.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The table identifier as `keyspace.table`.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"keyspace": schema.StringAttribute{
				Description: "The keyspace containing the table.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"table": schema.StringAttribute{
				Description: "The table to manage grants for.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"permissions": schema.ListAttribute{
				Description: "Effective grants currently applied as `keyspace.table:role:PRIVILEGE` strings. Used for drift detection.",
				Computed:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"grant": schema.ListNestedBlock{
				Description: "Privileges to grant to a role on the table.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"role": schema.StringAttribute{
							Description: "Role to receive the privileges.",
							Required:    true,
						},
						"privileges": schema.ListAttribute{
							Description: "Privileges to grant (e.g. ALTER, SELECT, MODIFY).",
							Required:    true,
							ElementType: types.StringType,
							Validators: []validator.List{
								listvalidator.UniqueValues(),
								listvalidator.ValueStringsAre(
									stringvalidator.OneOfCaseInsensitive(
										"ALTER",
										"AUTHORIZE",
										"DROP",
										"MODIFY",
										"SELECT",
									),
								),
							},
						},
					},
				},
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
		},
	}
}

func (r *tableGrantsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*scylladb.Cluster)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *scylladb.Cluster, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = client
}

func (r *tableGrantsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tableGrantsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags := r.applyPlanData(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tableGrantsResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
	// Refreshing the state based on the current state of db should not occur. Any change in the db warrants
	// a update. Refer to ModifyPlan for reading and resolving the diff.
}

func (r *tableGrantsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan tableGrantsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags := r.applyPlanData(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tableGrantsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state tableGrantsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := scylladb.ParseIdentifier(state.Keyspace.ValueString() + "." + state.Table.ValueString())
	if err := r.client.RevokeAllGrantsOnIdentifier(id); err != nil {
		resp.Diagnostics.AddError("Error Revoking Table Grants", err.Error())
	}
}

func (r *tableGrantsResource) ImportState(_ context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.AddError("Not supported", "Import is not supported for tableGrantsResource")
}

// ModifyPlan detects cases requiring an Update:
// - The grant config is changed.
// - Permissions drifted in the DB.
func (r *tableGrantsResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Skip if resource is being created or destroyed
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var state tableGrantsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan tableGrantsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// grant config is changed
	if !grantListsEqual(plan.Grants, state.Grants) {
		plan.Permissions = types.ListUnknown(types.StringType)
		resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
		return
	}

	// external drift
	id := scylladb.ParseIdentifier(state.Keyspace.ValueString() + "." + state.Table.ValueString())
	dbPerms, err := r.client.GetEffectivePermissions(id)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Effective Permissions", err.Error())
		return
	}

	var statePerms []string
	resp.Diagnostics.Append(state.Permissions.ElementsAs(ctx, &statePerms, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "table grants permissions check", map[string]any{"db": dbPerms, "state": statePerms})
	if slices.Compare(dbPerms, statePerms) == 0 {
		return
	}

	resp.Diagnostics.AddWarning(
		"Table grants are changed unexpectedly",
		fmt.Sprintf("Permissions on %q were modified externally and will be reconciled on next apply.\nExpected: %v\nActual:   %v",
			id.Original, statePerms, dbPerms),
	)
	plan.Permissions = types.ListUnknown(types.StringType)
	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
}

func (r *tableGrantsResource) applyPlanData(ctx context.Context, plan *tableGrantsResourceModel) (diags diag.Diagnostics) {
	id, bindings, extractDiags := r.extractPlanData(ctx, *plan)
	diags.Append(extractDiags...)
	if diags.HasError() {
		return
	}

	if err := r.client.ApplyAuthoritativeGrant(id, bindings); err != nil {
		diags.AddError("Error Applying Table Grants", err.Error())
		return
	}

	// populate computed attributes
	diags.Append(r.populateComputed(ctx, plan, id)...)
	return
}

func (r *tableGrantsResource) extractPlanData(ctx context.Context, plan tableGrantsResourceModel) (
	id scylladb.ParsedIdentifier,
	bindings []scylladb.AuthoritativeBinding,
	diags diag.Diagnostics,
) {
	id = scylladb.ParseIdentifier(plan.Keyspace.ValueString() + "." + plan.Table.ValueString())

	bindings = make([]scylladb.AuthoritativeBinding, 0, len(plan.Grants))
	for _, g := range plan.Grants {
		var privs []string
		diags.Append(g.Privileges.ElementsAs(ctx, &privs, false)...)
		if diags.HasError() {
			return
		}
		bindings = append(bindings, scylladb.AuthoritativeBinding{
			Privileges: privs,
			Role:       g.Role.ValueString(),
		})
	}
	return
}

func (r *tableGrantsResource) populateComputed(ctx context.Context, plan *tableGrantsResourceModel, id scylladb.ParsedIdentifier) diag.Diagnostics {
	var diags diag.Diagnostics

	perms, err := r.client.GetEffectivePermissions(id)
	if err != nil {
		diags.AddError("Error Reading Effective Permissions", err.Error())
		return diags
	}

	permsList, d := types.ListValueFrom(ctx, types.StringType, perms)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	plan.Permissions = permsList
	plan.ID = types.StringValue(id.Original)
	return diags
}
