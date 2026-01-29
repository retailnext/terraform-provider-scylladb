// Copyright RetailNext, Inc. 2026

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/retailnext/terraform-provider-scylladb/internal/testutil"
)

const (
	providerConfigFmt = `
provider "scylladb" {
  host = "%s"
  system_auth_keyspace = "system"
  auth_login_userpass {
    username = "cassandra"
    password = "cassandra"
  }
}
`
)

// testAccProtoV6ProviderFactories is used to instantiate a provider during acceptance testing.
// The factory function is called for each Terraform CLI command to create a provider
// server that the CLI can connect to and interact with.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"scylladb": providerserver.NewProtocol6WithError(New("test")()),
}

func TestAccProviderConfigmTLS(t *testing.T) {
	caCert, err := testutil.GenerateTestCACert()
	if err != nil {
		t.Fatalf("failed to generate CA certificate: %s", err)
	}
	serverCert, err := testutil.GenerateCert(caCert, testutil.CertSubject{
		CommonName:      "scylla-server",
		Organization:    []string{"My Org, Inc."},
		Country:         []string{"US"},
		DNSNames:        []string{"scylla-server"},
		DurationInYears: 1,
	})
	if err != nil {
		t.Fatalf("failed to generate server certificate: %s", err)
	}
	clientCert, err := testutil.GenerateCert(caCert, testutil.CertSubject{
		CommonName:      "cassandra",
		Organization:    []string{"My Org, Inc."},
		Country:         []string{"US"},
		DNSNames:        []string{"scylla-client"},
		DurationInYears: 1,
	})
	if err != nil {
		t.Fatalf("failed to generate client certificate: %s", err)
	}
	caCertPEM, _, err := caCert.PEMEncodedCert()
	if err != nil {
		t.Fatalf("failed to get CA PEM encoded cert: %s", err.Error())
	}
	clientCertPEM, clientKeyPEM, err := clientCert.PEMEncodedCert()
	if err != nil {
		t.Fatalf("failed to get client PEM encoded cert: %s", err.Error())
	}
	host := testutil.NewTestScyllaContainerMTLS(t, caCert, serverCert)

	providermTLSConfigFmt := `
provider "scylladb" {
  host = "%s"
  system_auth_keyspace = "system"
  skip_host_verification = true
}
data "scylladb_role" "cassandra" {
  id = "cassandra"
}
`
	t.Setenv("SCYLLADB_CA_CERT", string(caCertPEM))
	t.Setenv("SCYLLADB_CLIENT_CERT", string(clientCertPEM))
	t.Setenv("SCYLLADB_CLIENT_KEY", string(clientKeyPEM))

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Checking data is skipped since it is tested in data source tests
			{
				Config: fmt.Sprintf(providermTLSConfigFmt, host),
			},
		},
	})
}
