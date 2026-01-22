// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/i1snow/terraform-provider-scylladb/scylladb"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &roleDataSource{}
	_ datasource.DataSourceWithConfigure = &roleDataSource{}
)

// NewRoleDataSource is a helper function to simplify the provider implementation.
func NewRoleDataSource() datasource.DataSource {
	return &roleDataSource{}
}

// roleDataSource is the data source implementation.
type roleDataSource struct {
	client *scylladb.Cluster
}

// roleDataSourceModel maps the data source schema data.
type roleDataSourceModel struct {
	ID          types.String   `tfsdk:"id"`
	Role        types.String   `tfsdk:"role"`
	CanLogin    types.Bool     `tfsdk:"can_login"`
	IsSuperuser types.Bool     `tfsdk:"is_superuser"`
	MemberOf    []types.String `tfsdk:"member_of"`
}

// Metadata returns the data source type name.
func (d *roleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

// Schema defines the schema for the data source.
func (d *roleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The name of the role to look up.",
				Required:    true,
			},
			"role": schema.StringAttribute{
				Computed:    true,
				Description: "The name of the role",
			},
			"can_login": schema.BoolAttribute{
				Computed:    true,
				Description: "whether a user can login as a role",
			},
			"is_superuser": schema.BoolAttribute{
				Computed:    true,
				Description: "whether the role is a superuser",
			},
			"member_of": schema.ListAttribute{
				Computed:    true,
				Description: "a list of members of the role",
				ElementType: types.StringType,
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *roleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config roleDataSourceModel

	// Read config.
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	curRole, err := d.client.GetRole(config.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read the role",
			err.Error(),
		)
		return
	}

	// Map response body to model.
	state := roleDataSourceModel{
		ID:          config.ID,
		Role:        types.StringValue(curRole.Role),
		CanLogin:    types.BoolValue(curRole.CanLogin),
		IsSuperuser: types.BoolValue(curRole.IsSuperuser),
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

// Configure adds the provider configured client to the data source.
func (d *roleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Add a nil check when handling ProviderData because Terraform
	// sets that data after it calls the ConfigureProvider RPC.
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

	d.client = client
}
