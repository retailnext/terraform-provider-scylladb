// Copyright RetailNext, Inc. 2026

package provider

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/retailnext/terraform-provider-scylladb/scylladb"
)

var _ resource.Resource = &grantResource{}
var _ resource.ResourceWithConfigure = &grantResource{}
var _ resource.ResourceWithImportState = &grantResource{}
var _ resource.ResourceWithModifyPlan = &grantResource{}

func NewGrantResource() resource.Resource {
	return &grantResource{}
}

type grantResource struct {
	client *scylladb.Cluster
}

type grantResourceModel struct {
	ID           types.String `tfsdk:"id"`
	LastUpdated  types.String `tfsdk:"last_updated"`
	RoleName     types.String `tfsdk:"role_name"`
	Privilege    types.String `tfsdk:"privilege"`
	ResourceType types.String `tfsdk:"resource_type"`
	Keyspace     types.String `tfsdk:"keyspace"`
	Identifier   types.String `tfsdk:"identifier"`
	Permissions  types.List   `tfsdk:"permissions"`
}

func (g *grantResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_grant"
}

func (g *grantResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Grant resource",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The ID of the grant.",
				Computed:    true,
			},
			"last_updated": schema.StringAttribute{
				Description: "The timestamp of the last update to the grant.",
				Computed:    true,
			},
			"role_name": schema.StringAttribute{
				Description: "The role to which the privilege is granted.",
				Required:    true,
			},
			"privilege": schema.StringAttribute{
				Description: "The privilege to grant.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOfCaseInsensitive(
						// Refer to https://docs.scylladb.com/manual/stable/operating-scylla/security/authorization.html#grant-permission
						"ALL PERMISSIONS",
						"ALTER",
						"AUTHORIZE",
						"CREATE",
						"DESCRIBE",
						"DROP",
						"MODIFY",
						"SELECT",
					),
				},
			},
			"resource_type": schema.StringAttribute{
				Description: "The type of resource (e.g., ALL KEYSPACES, KEYSPACE, TABLE).",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOfCaseInsensitive(
						// Refer to https://docs.scylladb.com/manual/stable/operating-scylla/security/authorization.html#grant-permission
						"ALL KEYSPACES",
						"KEYSPACE",
						"TABLE",
						"ALL ROLES",
						"ROLE",
					),
				},
			},
			"keyspace": schema.StringAttribute{
				Description: "The keyspace of the resource.",
				Optional:    true,
			},
			"identifier": schema.StringAttribute{
				Description: "The identifier of the resource (e.g., table name).",
				Optional:    true,
			},
			"permissions": schema.ListAttribute{
				Computed:    true,
				Description: "The recorded permission for the grant",
				ElementType: types.StringType,
			},
		},
	}
}

func (g *grantResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	g.client = client
}

func (g *grantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan grantResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	grant := scylladb.Grant{
		RoleName:     plan.RoleName.ValueString(),
		Privilege:    plan.Privilege.ValueString(),
		ResourceType: plan.ResourceType.ValueString(),
		Keyspace:     plan.Keyspace.ValueString(),
		Identifier:   plan.Identifier.ValueString(),
	}
	err := g.client.CreateGrant(grant)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating Grant",
			err.Error(),
		)
		return
	}

	permissions, err := g.client.GetGrantPermissions(grant)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Getting Grant Permissions",
			err.Error(),
		)
		return
	}
	tflog.Debug(ctx, fmt.Sprintf("Adding permissions: %v", permissions))
	permissionsList, diags := types.ListValueFrom(ctx, types.StringType, permissions)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Permissions = permissionsList

	plan.ID = types.StringValue(fmt.Sprintf("%s|%s|%s|%s|%s", grant.RoleName, grant.Privilege, grant.ResourceType, grant.Keyspace, grant.Identifier))
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (g *grantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Refreshing the state based on the current state of db should not occur. Any change in the db warrants
	// a replace. Refer to ModifyPlan for reading and resolving the diff.
}

func (g *grantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from state
	var state grantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the role from state
	fromGrant := scylladb.Grant{
		RoleName:     state.RoleName.ValueString(),
		Privilege:    state.Privilege.ValueString(),
		ResourceType: state.ResourceType.ValueString(),
		Keyspace:     state.Keyspace.ValueString(),
		Identifier:   state.Identifier.ValueString(),
	}
	fromPermissions := []string{}
	diags := state.Permissions.ElementsAs(ctx, &fromPermissions, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve values from plan
	var plan grantResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get role from plan
	toGrant := scylladb.Grant{
		RoleName:     plan.RoleName.ValueString(),
		Privilege:    plan.Privilege.ValueString(),
		ResourceType: plan.ResourceType.ValueString(),
		Keyspace:     plan.Keyspace.ValueString(),
		Identifier:   plan.Identifier.ValueString(),
	}

	err := g.client.UpdateGrant(fromGrant, toGrant)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Grant",
			err.Error(),
		)
		return
	}

	newPermissions, err := g.client.GetGrantPermissions(toGrant)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Getting Grant Permissions",
			err.Error(),
		)
		return
	}
	permissionsList, diags := types.ListValueFrom(ctx, types.StringType, newPermissions)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Permissions = permissionsList
	// Populate Compuated attribute values
	plan.ID = types.StringValue(fmt.Sprintf("%s|%s|%s|%s|%s", toGrant.RoleName, toGrant.Privilege, toGrant.ResourceType, toGrant.Keyspace, toGrant.Identifier))
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (g *grantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state grantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the role from state
	grant := scylladb.Grant{
		RoleName:     state.RoleName.ValueString(),
		Privilege:    state.Privilege.ValueString(),
		ResourceType: state.ResourceType.ValueString(),
		Keyspace:     state.Keyspace.ValueString(),
		Identifier:   state.Identifier.ValueString(),
	}
	err := g.client.DeleteGrant(grant)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Grant",
			err.Error(),
		)
		return
	}
}

func (g *grantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// grant command is idempodent. Therefore, applying the same grant does not fail. Therefore, import has no meaning.
	// ID format: "RoleName|Privilege|ResourceType|Keyspace|Identifier"
	parts := strings.Split(req.ID, "|")
	if len(parts) != 5 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected ID format: RoleName|Privilege|ResourceType|Keyspace|Identifier, got: %s", req.ID),
		)
		return
	}

	grant := scylladb.Grant{
		RoleName:     parts[0],
		Privilege:    parts[1],
		ResourceType: parts[2],
		Keyspace:     parts[3],
		Identifier:   parts[4],
	}

	dbPermissions, err := g.client.GetGrantPermissions(grant)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Grant",
			err.Error(),
		)
		return
	}
	permissionsList, diags := types.ListValueFrom(ctx, types.StringType, dbPermissions)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("role_name"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("privilege"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("resource_type"), parts[2])...)
	if parts[3] != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("keyspace"), parts[3])...)
		if parts[4] != "" {
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("identifier"), parts[4])...)
		}
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("permissions"), permissionsList)...)

}

// ModifyPlan checks if the grant remains the same by checking the current permissions with the state permissions
// If the permissions was modified externally, the resource is marked for replacement.
func (g *grantResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Skip if resource is being created or destroyed
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var state grantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the permissions of the grant in db
	grant := scylladb.Grant{
		RoleName:     state.RoleName.ValueString(),
		Privilege:    state.Privilege.ValueString(),
		ResourceType: state.ResourceType.ValueString(),
		Keyspace:     state.Keyspace.ValueString(),
		Identifier:   state.Identifier.ValueString(),
	}
	dbPermissions, err := g.client.GetGrantPermissions(grant)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Grant",
			err.Error(),
		)
		return
	}

	// Get the permissions of the grant in the state
	statePermissions := []string{}
	diags := state.Permissions.ElementsAs(ctx, &statePermissions, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, fmt.Sprintf("got permissions: from db = %v | from state = %v", dbPermissions, statePermissions))
	// Compare the permissions. If not the same, update the plan's permission, which causes it to replace
	if slices.Compare(dbPermissions, statePermissions) == 0 {
		return
	}

	tflog.Warn(ctx, "permissions have changed outside of Terraform. The resource needs to be recreated",
		map[string]any{
			"state":  statePermissions,
			"actual": dbPermissions,
		})

	// Update the plan's permissions in response to reflect actual database state to trigger a diff
	var plan grantResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	actualPermissionsList, diags := types.ListValueFrom(ctx, types.StringType, dbPermissions)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Permissions = actualPermissionsList
	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
	resp.RequiresReplace = append(resp.RequiresReplace, path.Root("permissions"))
}
