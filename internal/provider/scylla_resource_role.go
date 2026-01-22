// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/i1snow/terraform-provider-scylladb/scylladb"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &roleResource{}
var _ resource.ResourceWithConfigure = &roleResource{}
var _ resource.ResourceWithImportState = &roleResource{}

func NewRoleResource() resource.Resource {
	return &roleResource{}
}

// roleResource defines the resource implementation.
type roleResource struct {
	client *scylladb.Cluster
}

// roleResourceModel maps the resource source schema data.
type roleResourceModel struct {
	ID          types.String   `tfsdk:"id"`
	LastUpdated types.String   `tfsdk:"last_updated"`
	Role        types.String   `tfsdk:"role"`
	CanLogin    types.Bool     `tfsdk:"can_login"`
	IsSuperuser types.Bool     `tfsdk:"is_superuser"`
	MemberOf    []types.String `tfsdk:"member_of"`
}

// Metadata returns the resource type name.
func (r *roleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

// The resource uses the `Schema` method to define the supported configuration, plan, and state attribute names and types.
func (r *roleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Role resource",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The name of the role to look up.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(), // the attribute is not configurable and should not show updates from the existing state
				},
			},
			"last_updated": schema.StringAttribute{
				Computed:    true,
				Description: "The time of the last time the resource was updated",
			},
			"role": schema.StringAttribute{
				Description: "The name of the role",
				Required:    true,
			},
			"can_login": schema.BoolAttribute{
				Description: "whether a user can login as a role",
				Optional:    true,
			},
			"is_superuser": schema.BoolAttribute{
				Description: "whether the role is a superuser",
				Optional:    true,
			},
			"member_of": schema.ListAttribute{
				Optional:    true,
				Description: "a list of members of the role",
				ElementType: types.StringType,
			},
		},
	}
}

// Resources use the optional `Configure` method to fetch configured clients from the provider.
func (r *roleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*scylladb.Cluster)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *scylladb.Cluster, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

// The provider uses the `Create` method to create a new resource based on the schemadata.
func (r *roleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan roleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get role from plan
	role := planToRole(plan)

	// Create a role
	err := r.client.CreateRole(role)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create the role",
			err.Error(),
		)
		return
	}

	// Populate computed attribute values
	plan.ID = types.StringValue(role.Role)
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	// Set state to fully populate data
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// The provider uses the `Read` method to retrieve the resource's information and update the state
// The provider invokes this function before every plan.
func (r *roleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state roleResourceModel

	// Read state.
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	curRole, err := r.client.GetRole(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read the role",
			err.Error(),
		)
		return
	}

	// Overwrite with refreshed state.
	state = roleResourceModel{
		ID:          types.StringValue(curRole.Role),
		Role:        types.StringValue(curRole.Role),
		CanLogin:    types.BoolValue(curRole.CanLogin),
		IsSuperuser: types.BoolValue(curRole.IsSuperuser),
		LastUpdated: types.StringValue(time.Now().Format(time.RFC850)),
	}
	for _, member := range curRole.MemberOf {
		state.MemberOf = append(state.MemberOf, types.StringValue(member))
	}

	// Set state.
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// The provider uses the `Update` method to update an existing resource based on the schema data.
func (r *roleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan roleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get role from plan
	role := planToRole(plan)

	// Update the role
	err := r.client.UpdateRole(role)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update the role",
			err.Error(),
		)
		return
	}
	// Populate Compuated attribute values
	plan.ID = types.StringValue(role.Role)
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// The provider uses the `Delete` method to attempt to retrieve the values from state and delete the resource.
func (r *roleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state roleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the role from state
	role := scylladb.Role{
		Role: state.Role.ValueString(),
	}

	// Delete the role
	err := r.client.DeleteRole(role)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to delete the role",
			err.Error(),
		)
		return
	}
}

// The provider users the `ImportState` method to import an existing source.
func (r *roleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func planToRole(plan roleResourceModel) scylladb.Role {
	role := scylladb.Role{
		Role:        plan.Role.ValueString(),
		CanLogin:    plan.CanLogin.ValueBool(),
		IsSuperuser: plan.IsSuperuser.ValueBool(),
	}
	for _, member := range plan.MemberOf {
		role.MemberOf = append(role.MemberOf, member.ValueString())
	}
	return role
}
