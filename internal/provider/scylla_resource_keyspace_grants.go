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

var _ resource.Resource = &keyspaceGrantsResource{}
var _ resource.ResourceWithConfigure = &keyspaceGrantsResource{}
var _ resource.ResourceWithImportState = &keyspaceGrantsResource{}

func NewKeyspaceGrantsResource() resource.Resource {
	return &keyspaceGrantsResource{}
}

type keyspaceGrantsResource struct {
	client *scylladb.Cluster
}

type keyspaceGrantsResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Keyspace types.String `tfsdk:"keyspace"`
	Grants   Grants       `tfsdk:"grant"`
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
		},
		Blocks: map[string]schema.Block{
			"grant": schema.SetNestedBlock{
				Description: "Privileges to grant to a role on the keyspace.",
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
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					uniqueGrantRolesValidator{},
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

func (r *keyspaceGrantsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state keyspaceGrantsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := scylladb.ParseIdentifier(state.Keyspace.ValueString())
	roleBindings, err := r.client.GetAllRoleBindingsPerId(id)
	if err != nil {
		resp.Diagnostics.AddError("Error Reading Keyspace Grants", err.Error())
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

func (r *keyspaceGrantsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := scylladb.ParseIdentifier(req.ID)
	if id.ResourceType != "KEYSPACE" {
		resp.Diagnostics.AddError("Invalid Import ID", fmt.Sprintf("Expected a keyspace identifier, got %q", req.ID))
		return
	}
	permissionMap, err := r.client.GetAllRolePermissionsPerId(id)
	if err != nil {
		resp.Diagnostics.AddError("Error Importing Keyspace Grants", err.Error())
		return
	}
	if len(permissionMap) == 0 {
		resp.Diagnostics.AddError("No Grants found to Import",
			fmt.Sprintf("No grants were found for keyspace %q. Only identifiers with existing grants can be imported.", id.Original))
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

	state := keyspaceGrantsResourceModel{
		ID:       types.StringValue(id.Original),
		Keyspace: types.StringValue(id.Keyspace),
		Grants:   grants,
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
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

	plan.ID = types.StringValue(id.Original)
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
