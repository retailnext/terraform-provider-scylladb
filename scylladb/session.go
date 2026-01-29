// Copyright RetailNext, Inc. 2026

package scylladb

import (
	"crypto/tls"
	"crypto/x509"
	"errors"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
)

type Cluster struct {
	Cluster                *gocql.ClusterConfig
	SystemAuthKeyspaceName string
	Session                *gocql.Session
}

func NewClusterConfig(hosts []string) Cluster {
	cluster := gocql.NewCluster(hosts...)
	cluster.DisableInitialHostLookup = true
	cluster.NumConns = 1
	return Cluster{
		Cluster:                cluster,
		SystemAuthKeyspaceName: "system_auth",
	}
}

func (c *Cluster) CreateSession() error {
	session, err := c.Cluster.CreateSession()
	if err != nil {
		return err
	}
	c.Session = session
	return nil
}

func (c *Cluster) SetUserPasswordAuth(username, password string) {
	c.Cluster.Authenticator = gocql.PasswordAuthenticator{
		Username: username,
		Password: password,
	}
}

func (c *Cluster) SetSystemAuthKeyspace(name string) {
	c.SystemAuthKeyspaceName = name
}

func (c *Cluster) SetTLS(caCert, clientCert, clientKey []byte, enableHostVerification bool) error {
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
		return errors.New("failed to append CA certificate")
	}

	tlsConfig := &tls.Config{
		RootCAs:            caCertPool,
		InsecureSkipVerify: !enableHostVerification,
	}

	if len(clientCert) > 0 && len(clientKey) > 0 {
		cert, err := tls.X509KeyPair(clientCert, clientKey)
		if err != nil {
			return err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}
	c.Cluster.SslOpts = &gocql.SslOptions{
		Config: tlsConfig,
		// This option is the inverse of tls.Config.InsecureSkipVerify. Setting it explicitly for clarity.
		EnableHostVerification: enableHostVerification,
	}
	return nil
}
