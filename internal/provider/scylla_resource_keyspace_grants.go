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

var _ resource.Resource = &keyspaceGrantsResource{}
var _ resource.ResourceWithConfigure = &keyspaceGrantsResource{}
var _ resource.ResourceWithModifyPlan = &keyspaceGrantsResource{}

func NewKeyspaceGrantsResource() resource.Resource {
	return &keyspaceGrantsResource{}
}

type keyspaceGrantsResource struct {
	client *scylladb.Cluster
}

type keyspaceGrantsResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Keyspace    types.String `tfsdk:"keyspace"`
	Grants      []grantModel `tfsdk:"grant"`
	Permissions types.List   `tfsdk:"permissions"`
}

// grantModel is shared by keyspaceGrantsResource and tableGrantsResource.
type grantModel struct {
	Role       types.String `tfsdk:"role"`
	Privileges types.List   `tfsdk:"privileges"`
}

func (r *keyspaceGrantsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_keyspace_grants"
}

func (r *keyspaceGrantsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Authoritatively manages all grants on a keyspace. Any existing grants not specified in `grant` blocks are revoked on apply.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The keyspace name.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"keyspace": schema.StringAttribute{
				Description: "The keyspace to manage grants for.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"permissions": schema.ListAttribute{
				Description: "Effective grants currently applied as `keyspace:role:PRIVILEGE` strings. Used for drift detection.",
				Computed:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"grant": schema.ListNestedBlock{
				Description: "Privileges to grant to a role on the keyspace.",
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
										"CREATE",
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

func (r *keyspaceGrantsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *keyspaceGrantsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan keyspaceGrantsResourceModel
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

func (r *keyspaceGrantsResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
	// Refreshing the state based on the current state of db should not occur. Any change in the db warrants
	// a update. Refer to ModifyPlan for reading and resolving the diff.
}

func (r *keyspaceGrantsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan keyspaceGrantsResourceModel
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

func (r *keyspaceGrantsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state keyspaceGrantsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := scylladb.ParseIdentifier(state.Keyspace.ValueString())
	if err := r.client.RevokeAllGrantsOnIdentifier(id); err != nil {
		resp.Diagnostics.AddError("Error Revoking Keyspace Grants", err.Error())
	}
}

func (r *keyspaceGrantsResource) ImportState(_ context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.AddError("Not supported", "Import is not supported for keyspaceGrantsResource")
}

// ModifyPlan detects cases requiring an Update:
// - The grant config is changed.
// - Permissions drifted in the DB.
func (r *keyspaceGrantsResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Skip if resource is being created or destroyed
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var state keyspaceGrantsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan keyspaceGrantsResourceModel
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
	id := scylladb.ParseIdentifier(state.Keyspace.ValueString())
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

	tflog.Debug(ctx, "keyspace grants permissions check", map[string]any{"db": dbPerms, "state": statePerms})
	if slices.Compare(dbPerms, statePerms) == 0 {
		return
	}

	resp.Diagnostics.AddWarning(
		"Keyspace grants are changed unexpectedly",
		fmt.Sprintf("Permissions on %q were modified externally and will be reconciled on next apply.\nExpected: %v\nActual:   %v",
			id.Original, statePerms, dbPerms),
	)
	plan.Permissions = types.ListUnknown(types.StringType)
	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
}

// grantListsEqual returns true if two grant lists have identical roles and privileges in the same order.
func grantListsEqual(a, b []grantModel) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Role.Equal(b[i].Role) || !a[i].Privileges.Equal(b[i].Privileges) {
			return false
		}
	}
	return true
}

func (r *keyspaceGrantsResource) applyPlanData(ctx context.Context, plan *keyspaceGrantsResourceModel) (diags diag.Diagnostics) {
	id, bindings, extractDiags := r.extractPlanData(ctx, *plan)
	diags.Append(extractDiags...)
	if diags.HasError() {
		return
	}

	if err := r.client.ApplyAuthoritativeGrant(id, bindings); err != nil {
		diags.AddError("Error Applying Keyspace Grants", err.Error())
		return
	}

	// populate computed attributes
	diags.Append(r.populateComputed(ctx, plan, id)...)
	return
}

func (r *keyspaceGrantsResource) extractPlanData(ctx context.Context, plan keyspaceGrantsResourceModel) (
	id scylladb.ParsedIdentifier,
	bindings []scylladb.AuthoritativeBinding,
	diags diag.Diagnostics,
) {
	id = scylladb.ParseIdentifier(plan.Keyspace.ValueString())

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

func (r *keyspaceGrantsResource) populateComputed(ctx context.Context, plan *keyspaceGrantsResourceModel, id scylladb.ParsedIdentifier) diag.Diagnostics {
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
