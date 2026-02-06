// Copyright RetailNext, Inc. 2026

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
	"github.com/retailnext/terraform-provider-scylladb/scylladb"
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
	Host                 types.String            `tfsdk:"host"`
	DnsAddress           types.String            `tfsdk:"dns_address"`
	SystemAuthKeyspace   types.String            `tfsdk:"system_auth_keyspace"`
	SkipHostVerification types.Bool              `tfsdk:"skip_host_verification"`
	CAcertFile           types.String            `tfsdk:"ca_cert_file"`
	AuthLoginUserPass    *authLoginUserPassModel `tfsdk:"auth_login_userpass"`
	AuthTLS              *authTLSModel           `tfsdk:"auth_tls"`
}

type authLoginUserPassModel struct {
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
}

type authTLSModel struct {
	CertFile types.String `tfsdk:"cert_file"`
	KeyFile  types.String `tfsdk:"key_file"`
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
			"dns_address": schema.StringAttribute{
				MarkdownDescription: "DNS server IP address for resolving hostnames through the proxy. This is relevant when you use a proxy and the hostname must be resolved through the network of the proxy",
				Optional:            true,
			},
			"system_auth_keyspace": schema.StringAttribute{
				MarkdownDescription: "The keyspace where ScyllaDB stores authentication and authorization information. Default is `system_auth`.",
				Optional:            true,
			},
			"ca_cert_file": schema.StringAttribute{
				MarkdownDescription: "Path to the CA certificate file for TLS connections.",
				Optional:            true,
			},
			"skip_host_verification": schema.BoolAttribute{
				MarkdownDescription: "Skip TLS host verification. Default is `false`.",
				Optional:            true,
			},
		},
		Blocks: map[string]schema.Block{
			"auth_login_userpass": schema.SingleNestedBlock{
				Description: "Login to ScyllaDB using the userpass method",
				Attributes: map[string]schema.Attribute{
					"username": schema.StringAttribute{
						Description: "Login with username",
						Optional:    true,
					},
					"password": schema.StringAttribute{
						Description: "Login with password",
						Optional:    true,
						Sensitive:   true,
					},
				},
			},
			"auth_tls": schema.SingleNestedBlock{
				Description: "Login to ScyllaDB using TLS",
				Attributes: map[string]schema.Attribute{
					"cert_file": schema.StringAttribute{
						Description: "Path to the client certificate file for TLS connections",
						Optional:    true,
					},
					"key_file": schema.StringAttribute{
						Description: "Path to the client key file for TLS connections",
						Optional:    true,
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
	var dnsAddress string
	if !data.DnsAddress.IsNull() {
		dnsAddress = data.DnsAddress.ValueString()
	}
	client, err := scylladb.NewClusterConfigWithDns([]string{host}, dnsAddress)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create a Cluster Configuration",
			"An unexpected error was encountered trying to configure the cluster. "+
				"Please verify the setup and the values of host and dnsAddress and try again.\n\n"+
				err.Error(),
		)
	}

	// Set system auth keyspace
	if !data.SystemAuthKeyspace.IsNull() {
		client.SetSystemAuthKeyspace(data.SystemAuthKeyspace.ValueString())
	} else {
		client.SetSystemAuthKeyspace("system_auth")
	}

	// Set Username/Password authentication if configured
	if data.AuthLoginUserPass != nil {
		tflog.Debug(ctx, "Configuring Username/Password authentication for ScyllaDB client")
		password := os.Getenv("SCYLLADB_PASSWORD")
		if !data.AuthLoginUserPass.Password.IsNull() {
			password = data.AuthLoginUserPass.Password.ValueString()
		}

		if password == "" {
			resp.Diagnostics.AddAttributeError(
				path.Root("auth_login_userpass").AtName("password"),
				"Missing ScyllaDB Password",
				"The provider cannot create the ScyllaDB client as there is a missing or empty value for the ScyllaDB password. "+
					"Set the password value in the configuration or use the SCYLLADB_PASSWORD environment variable. "+
					"If either is already set, ensure the value is not empty.",
			)
		}

		client.SetUserPasswordAuth(data.AuthLoginUserPass.Username.ValueString(), password)
	}

	// Set TLS authentication if configured

	// Default
	caCert := []byte(os.Getenv("SCYLLADB_CA_CERT"))
	clientCert := []byte(os.Getenv("SCYLLADB_CLIENT_CERT"))
	clientKey := []byte(os.Getenv("SCYLLADB_CLIENT_KEY"))
	skipHostVerification := false

	tflog.Debug(ctx, "TLS env vars", map[string]any{
		"ca_cert_len":     len(os.Getenv("SCYLLADB_CA_CERT")),
		"client_cert_len": len(os.Getenv("SCYLLADB_CLIENT_CERT")),
		"client_key_len":  len(os.Getenv("SCYLLADB_CLIENT_KEY")),
	})

	// Read the CA cert file if provided
	if !data.CAcertFile.IsNull() {
		if !data.CAcertFile.IsNull() {
			caCertFile := data.CAcertFile.ValueString()
			caCert, err = os.ReadFile(caCertFile)
			if err != nil {
				resp.Diagnostics.AddAttributeError(
					path.Root("ca_cert_file"),
					"Unable to Read CA Certificate File",
					"An unexpected error was encountered trying to read the CA certificate file. "+
						"Please verify the file path is correct and try again.\n\n"+
						err.Error(),
				)
			}
		}
	}

	// Read the skip host verification flag
	if !data.SkipHostVerification.IsNull() {
		skipHostVerification = data.SkipHostVerification.ValueBool()
	}

	// Read the client cert and key files if provided
	if data.AuthTLS != nil {
		if !data.AuthTLS.CertFile.IsNull() {
			certFile := data.AuthTLS.CertFile.ValueString()
			clientCert, err = os.ReadFile(certFile)
			if err != nil {
				resp.Diagnostics.AddAttributeError(
					path.Root("auth_tls").AtName("cert_file"),
					"Unable to Read Client Certificate File",
					"An unexpected error was encountered trying to read the client certificate file. "+
						"Please verify the file path is correct and try again.\n\n"+
						err.Error(),
				)
			}
		}

		if !data.AuthTLS.KeyFile.IsNull() {
			keyFile := data.AuthTLS.KeyFile.ValueString()
			clientKey, err = os.ReadFile(keyFile)
			if err != nil {
				resp.Diagnostics.AddAttributeError(
					path.Root("auth_tls").AtName("key_file"),
					"Unable to Read Client Key File",
					"An unexpected error was encountered trying to read the client key file. "+
						"Please verify the file path is correct and try again.\n\n"+
						err.Error(),
				)
			}
		}
	}

	if len(caCert) > 0 || len(clientCert) > 0 || len(clientKey) > 0 {
		tflog.Debug(ctx, "Configuring TLS for ScyllaDB client")
		err = client.SetTLS(caCert, clientCert, clientKey, !skipHostVerification)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to Configure TLS for ScyllaDB Client",
				"An unexpected error was encountered trying to configure TLS for the ScyllaDB client. "+
					"Please verify the certificate and key files are correct and try again.\n\n"+
					err.Error(),
			)
		}
	}

	if resp.Diagnostics.HasError() {
		return
	}

	err = client.CreateSession()
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
	resp.DataSourceData = client
	resp.ResourceData = client

	tflog.Info(ctx, "Configured ScyllaDB client", map[string]any{"success": true})
}

func (p *scylladbProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewRoleResource,
		NewGrantResource,
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
