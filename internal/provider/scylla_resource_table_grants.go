// Copyright RetailNext, Inc. 2026

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/retailnext/terraform-provider-scylladb/scylladb"
)

var _ resource.Resource = &tableGrantsResource{}
var _ resource.ResourceWithConfigure = &tableGrantsResource{}
var _ resource.ResourceWithImportState = &tableGrantsResource{}

func NewTableGrantsResource() resource.Resource {
	return &tableGrantsResource{}
}

type tableGrantsResource struct {
	client *scylladb.Cluster
}

type tableGrantsResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Keyspace types.String `tfsdk:"keyspace"`
	Table    types.String `tfsdk:"table"`
	Grants   Grants       `tfsdk:"grant"`
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
		},
		Blocks: map[string]schema.Block{
			"grant": schema.SetNestedBlock{
				Description: "Privileges to grant to a role on the table.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"role": schema.StringAttribute{
							Description: "Role to receive the privileges.",
							Required:    true,
						},
						"privileges": schema.SetAttribute{
							Description: "Privileges to grant (e.g. ALTER, SELECT, MODIFY).",
							Required:    true,
							ElementType: types.StringType,
							Validators: []validator.Set{
								setvalidator.ValueStringsAre(
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
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					uniqueGrantRolesValidator{},
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

func (r *tableGrantsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tableGrantsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := scylladb.ParseIdentifier(state.ID.ValueString())
	roleBindings, err := r.client.GetAllRoleBindingsPerId(id)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Table Grants", err.Error())
		return
	}
	grants, diags := GetGrantsFromRoleBindings(ctx, roleBindings)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Grants = grants
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
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
	id := scylladb.ParseIdentifier(state.ID.ValueString())
	if err := r.client.RevokeAllGrantsOnIdentifier(id); err != nil {
		resp.Diagnostics.AddError("Error Revoking Table Grants", err.Error())
	}
}

func (r *tableGrantsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := scylladb.ParseIdentifier(req.ID)
	if id.ResourceType != "TABLE" {
		resp.Diagnostics.AddError("Invalid Import ID", fmt.Sprintf("Expected a table identifier in the format 'keyspace.table', got %q", req.ID))
		return
	}
	permissionMap, err := r.client.GetAllRolePermissionsPerId(id)
	if err != nil {
		resp.Diagnostics.AddError("Error Importing Table Grants", err.Error())
		return
	}
	if len(permissionMap) == 0 {
		resp.Diagnostics.AddError("No Grants Found to Import", fmt.Sprintf("No grants were found on %q. Only identifiers with existing grants can be imported.", id.Original))
		return
	}
	var grants Grants
	for role, privs := range permissionMap {
		privSet, diags := types.SetValueFrom(ctx, types.StringType, privs)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		grants = append(grants, grantModel{
			Role:       types.StringValue(role),
			Privileges: privSet,
		})
	}
	state := tableGrantsResourceModel{
		ID:       types.StringValue(id.Original),
		Keyspace: types.StringValue(id.Keyspace),
		Table:    types.StringValue(id.Table),
		Grants:   grants,
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
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

	plan.ID = types.StringValue(id.Original)
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
