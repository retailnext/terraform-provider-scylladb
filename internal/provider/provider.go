// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/i1snow/terraform-provider-scylladb/internal/consts"
	"github.com/i1snow/terraform-provider-scylladb/scylladb"
)

// Ensure ScylladbProvider satisfies various provider interfaces.
var _ provider.Provider = &scylladbProvider{}
var _ provider.ProviderWithFunctions = &scylladbProvider{}
var _ provider.ProviderWithEphemeralResources = &scylladbProvider{}
var _ provider.ProviderWithActions = &scylladbProvider{}

// ScylladbProvider defines the provider implementation.
type scylladbProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// scylladbProviderModel describes the provider data model.
type scylladbProviderModel struct {
	Host               types.String            `tfsdk:"host"`
	SystemAuthKeyspace types.String            `tfsdk:"system_auth_keyspace"`
	AuthLoginUserPass  *authLoginUserPassModel `tfsdk:"auth_login_userpass"`
}

type authLoginUserPassModel struct {
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
}

func (p *scylladbProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "scylladb"
	resp.Version = p.version
}

func (p *scylladbProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Configure access to ScyllaDB.",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				MarkdownDescription: "Hostname or IP address of the ScyllaDB instance with a port if necessary. e.g. localhost:9042",
				Optional:            true,
			},
			"system_auth_keyspace": schema.StringAttribute{
				MarkdownDescription: "The keyspace where ScyllaDB stores authentication and authorization information. Default is `system_auth`.",
				Optional:            true,
			},
		},
		Blocks: map[string]schema.Block{
			consts.FieldAuthLoginUserpass: schema.SingleNestedBlock{
				Description: "Login to ScyllaDB using the userpass method",
				Attributes: map[string]schema.Attribute{
					"username": schema.StringAttribute{
						Description: "Login with username",
						Required:    true,
					},
					"password": schema.StringAttribute{
						Description: "Login with password",
						Required:    true,
						Sensitive:   true,
					},
				},
			},
		},
	}
}

func (p *scylladbProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring Scylla client")

	// Retrieve provider data from configuration.
	var data scylladbProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Provider configuration", map[string]interface{}{
		"host": data.Host.ValueString(),
		//"username": data.Username.ValueString(),
		// Do not log sensitive values such as passwords.
	})

	if data.Host.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Unknown ScyllaDB API Host",
			"The provider cannot create the ScyllaDB client as there is an unknown configuration value for. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the SCYLLADB_HOST environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Default values to environment variables, but override
	// with Terraform configuration value if set.

	host := os.Getenv("SCYLLADB_HOST")

	if !data.Host.IsNull() {
		host = data.Host.ValueString()
	}

	// If any of the expected configurations are missing, return
	// errors with provider-specific guidance.

	if host == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Missing ScyllaDB Host",
			"The provider cannot create the ScyllaDB client as there is a missing or empty value for the ScyllaDB host. "+
				"Set the host value in the configuration or use the SCYLLADB_HOST environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "scylladb_host", host)
	tflog.Debug(ctx, "Creating scylladb client")

	// Create a new scylladb client using the config
	client := scylladb.NewClusterConfig([]string{host})

	// Set system auth keyspace
	if !data.SystemAuthKeyspace.IsNull() {
		client.SetSystemAuthKeyspace(data.SystemAuthKeyspace.ValueString())
	} else {
		client.SetSystemAuthKeyspace("system_auth")
	}

	if data.AuthLoginUserPass != nil {
		client.SetUserPasswordAuth(data.AuthLoginUserPass.Username.ValueString(), data.AuthLoginUserPass.Password.ValueString())
	}

	err := client.CreateSession()
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create ScyllaDB Client",
			"An unexpected error was encountered trying to create the ScyllaDB client. "+
				"Please verify the provider configuration values are correct and try again.\n\n"+
				err.Error(),
		)
		return
	}

	// Make the HashiCups client available during DataSource and Resource
	// type Configure methods.
	resp.DataSourceData = &client
	resp.ResourceData = &client

	tflog.Info(ctx, "Configured ScyllaDB client", map[string]any{"success": true})
}

func (p *scylladbProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewRoleResource,
	}
}

func (p *scylladbProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		//NewExampleEphemeralResource,
	}
}

func (p *scylladbProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewRoleDataSource,
	}
}

func (p *scylladbProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{
		//NewExampleFunction,
	}
}

func (p *scylladbProvider) Actions(ctx context.Context) []func() action.Action {
	return []func() action.Action{
		//NewExampleAction,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &scylladbProvider{
			version: version,
		}
	}
}
