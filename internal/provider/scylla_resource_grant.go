// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/retailnext/terraform-provider-scylladb/scylladb"
)

var _ resource.Resource = &grantResource{}
var _ resource.ResourceWithConfigure = &grantResource{}
var _ resource.ResourceWithImportState = &grantResource{}

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
				Description: "The privilege to grant (e.g. ALL, CREATE, ALTER).",
				Required:    true,
			},
			"resource_type": schema.StringAttribute{
				Description: "The type of resource (e.g., ALL KEYSPACES, KEYSPACE, TABLE).",
				Required:    true,
			},
			"keyspace": schema.StringAttribute{
				Description: "The keyspace of the resource.",
				Optional:    true,
			},
			"identifier": schema.StringAttribute{
				Description: "The identifier of the resource (e.g., table name).",
				Optional:    true,
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
	plan.ID = types.StringValue(fmt.Sprintf("%s|%s|%s|%s|%s", grant.RoleName, grant.Privilege, grant.ResourceType, grant.Keyspace, grant.Identifier))
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (g *grantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state grantResourceModel

	// Read state.
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	curPerms, found, err := g.client.GetGrant(scylladb.Grant{
		RoleName:     state.RoleName.ValueString(),
		Privilege:    state.Privilege.ValueString(),
		ResourceType: state.ResourceType.ValueString(),
		Keyspace:     state.Keyspace.ValueString(),
		Identifier:   state.Identifier.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Grant",
			err.Error(),
		)
		return
	}

	if !found || len(curPerms) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	// No changes to state as all attributes are known from the ID and plan.
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
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
	// ID format: "RoleName|Privilege|ResourceType|Keyspace|Identifier"
	parts := strings.Split(req.ID, "|")
	if len(parts) != 5 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			fmt.Sprintf("Expected ID format: RoleName|Privilege|ResourceType|Keyspace|Identifier, got: %s", req.ID),
		)
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
}
