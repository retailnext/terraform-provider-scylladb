// Copyright RetailNext, Inc. 2026

package scylladb

import (
	"testing"

	"github.com/retailnext/terraform-provider-scylladb/internal/testutil"
	"github.com/stretchr/testify/assert"
)

var (
	caCert        *testutil.Cert
	clientCert    *testutil.Cert
	serverCert    *testutil.Cert
	caCertPEM     []byte
	clientCertPEM []byte
	clientKeyPEM  []byte
)

func TestMain(m *testing.M) {
	var err error
	caCert, err = testutil.GenerateTestCACert()
	if err != nil {
		panic("failed to generate CA certificate: " + err.Error())
	}
	serverCert, err = testutil.GenerateCert(caCert, testutil.CertSubject{
		CommonName:      "scylla-server",
		Organization:    []string{"My Org, Inc."},
		Country:         []string{"US"},
		DNSNames:        []string{"scylla-server"},
		DurationInYears: 1,
	})
	if err != nil {
		panic("failed to generate server certificate: " + err.Error())
	}
	clientCert, err = testutil.GenerateCert(caCert, testutil.CertSubject{
		CommonName:      "cassandra",
		Organization:    []string{"My Org, Inc."},
		Country:         []string{"US"},
		DNSNames:        []string{"scylla-client"},
		DurationInYears: 1,
	})
	if err != nil {
		panic("failed to generate client certificate: " + err.Error())
	}
	caCertPEM, _, err = caCert.PEMEncodedCert()
	if err != nil {
		panic("failed to get CA PEM encoded cert: " + err.Error())
	}
	clientCertPEM, clientKeyPEM, err = clientCert.PEMEncodedCert()
	if err != nil {
		panic("failed to get client PEM encoded cert: " + err.Error())
	}
	m.Run()
}
func TestSetmTLS(t *testing.T) {
	if caCert == nil || clientCert == nil {
		t.Fatal("certificates are not initialized")
	}
	host := testutil.NewTestScyllaContainerMTLS(t, caCert, serverCert)
	cluster, err := NewClusterConfig([]string{host})
	if err != nil {
		t.Fatalf("failed to create a new cluster config: %s", err)
	}
	cluster.SetSystemAuthKeyspace("system")
	err = cluster.SetTLS(caCertPEM, clientCertPEM, clientKeyPEM, false)
	if err != nil {
		t.Fatalf("failed to set TLS: %s", err)
	}
	if err := cluster.CreateSession(); err != nil {
		t.Fatalf("failed to create session: %s", err)
	}
	defer cluster.Session.Close()

	role, err := cluster.GetRole("cassandra")
	if err != nil {
		t.Fatalf("failed to get role: %s", err)
	}

	expectedRole := Role{
		Role:        "cassandra",
		CanLogin:    true,
		IsSuperuser: true,
		MemberOf:    nil,
	}

	assert.Equal(t, expectedRole, role)
}
