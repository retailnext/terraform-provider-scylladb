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

func TestSetTLS_InvalidCA(t *testing.T) {
	cluster, err := NewClusterConfig([]string{"127.0.0.1:9042"})
	if err != nil {
		t.Fatalf("failed to create cluster config: %s", err)
	}
	err = cluster.SetTLS([]byte("not-a-pem"), nil, nil, false)
	assert.EqualError(t, err, "failed to append CA certificate")
}

func TestSetTLS_InvalidClientKeyPair(t *testing.T) {
	cluster, err := NewClusterConfig([]string{"127.0.0.1:9042"})
	if err != nil {
		t.Fatalf("failed to create cluster config: %s", err)
	}
	err = cluster.SetTLS(caCertPEM, clientCertPEM, []byte("not-a-key"), false)
	assert.Error(t, err)
}

func TestSetTLS_InsecureSkipVerify(t *testing.T) {
	cluster, err := NewClusterConfig([]string{"127.0.0.1:9042"})
	if err != nil {
		t.Fatalf("failed to create cluster config: %s", err)
	}

	err = cluster.SetTLS(caCertPEM, nil, nil, false)
	assert.NoError(t, err)
	assert.True(t, cluster.Cluster.SslOpts.InsecureSkipVerify)

	err = cluster.SetTLS(caCertPEM, nil, nil, true)
	assert.NoError(t, err)
	assert.False(t, cluster.Cluster.SslOpts.InsecureSkipVerify)
}

func TestSetTLS_NoClientCert(t *testing.T) {
	cluster, err := NewClusterConfig([]string{"127.0.0.1:9042"})
	if err != nil {
		t.Fatalf("failed to create cluster config: %s", err)
	}
	err = cluster.SetTLS(caCertPEM, nil, nil, false)
	assert.NoError(t, err)
	assert.Empty(t, cluster.Cluster.SslOpts.Certificates)
}

func TestSetTLS_WithClientCert(t *testing.T) {
	cluster, err := NewClusterConfig([]string{"127.0.0.1:9042"})
	if err != nil {
		t.Fatalf("failed to create cluster config: %s", err)
	}
	err = cluster.SetTLS(caCertPEM, clientCertPEM, clientKeyPEM, false)
	assert.NoError(t, err)
	assert.Len(t, cluster.Cluster.SslOpts.Certificates, 1)
	assert.NotNil(t, cluster.Cluster.SslOpts.GetClientCertificate)
}

func TestSetTLS_PropagatesConfigToProxyHostDialer(t *testing.T) {
	proxyHostDialer := &ProxyHostDialer{}
	cluster, err := NewClusterConfig([]string{"127.0.0.1:9042"})
	if err != nil {
		t.Fatalf("failed to create cluster config: %s", err)
	}
	cluster.Cluster.HostDialer = proxyHostDialer

	err = cluster.SetTLS(caCertPEM, clientCertPEM, clientKeyPEM, false)
	assert.NoError(t, err)
	assert.NotNil(t, proxyHostDialer.tlsConfig)
	assert.Same(t, cluster.Cluster.SslOpts.Config, proxyHostDialer.tlsConfig)
}

func TestSetTLS_NoProxyDialer_DoesNotPanic(t *testing.T) {
	cluster, err := NewClusterConfig([]string{"127.0.0.1:9042"})
	if err != nil {
		t.Fatalf("failed to create cluster config: %s", err)
	}
	// HostDialer is nil by default; SetTLS should not panic
	assert.NotPanics(t, func() {
		_ = cluster.SetTLS(caCertPEM, nil, nil, false)
	})
}

func TestCreateProxyHostMap(t *testing.T) {
	tests := []struct {
		name         string
		hosts        []string
		wantProxyMap map[string]string
	}{
		{
			name:         "Single hostname without port",
			hosts:        []string{"test-client.scylla-cluster.svc"},
			wantProxyMap: map[string]string{"127.0.0.1": "test-client.scylla-cluster.svc:9042"},
		},
		{
			name:         "2 hostnames with ports",
			hosts:        []string{"test-client-1.scylla-cluster.svc:9142", "test-client-2.scylla-cluster.svc:9142"},
			wantProxyMap: map[string]string{"127.0.0.1": "test-client-1.scylla-cluster.svc:9142", "127.0.0.2": "test-client-2.scylla-cluster.svc:9142"},
		},
		{
			name:         "IPs and hostnames mixed",
			hosts:        []string{"test-client-1.scylla-cluster.svc:9042", "192.168.1.100:9042"},
			wantProxyMap: map[string]string{"127.0.0.1": "test-client-1.scylla-cluster.svc:9042", "127.0.0.2": "192.168.1.100:9042"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hostMap := createProxyHostMap(tc.hosts)
			assert.Equal(t, tc.wantProxyMap, hostMap)
		})
	}
}
