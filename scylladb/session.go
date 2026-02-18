// Copyright RetailNext, Inc. 2026

package scylladb

import (
	"cmp"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
	"golang.org/x/net/proxy"
)

type Cluster struct {
	Cluster                *gocql.ClusterConfig
	SystemAuthKeyspaceName string
	Session                *gocql.Session
}

type ProxyHostDialer struct {
	proxyDialer proxy.Dialer
	hosts       []string
	tlsConfig   *tls.Config
}

func (d *ProxyHostDialer) DialHost(ctx context.Context, host *gocql.HostInfo) (dialedHost *gocql.DialedHost, err error) {
	// Construct connection through proxy
	var conn net.Conn
	var connAddr string
	for _, hostAddr := range d.hosts {
		log.Printf("Connecting to %s via SOCKS5 proxy", hostAddr)
		conn, err = d.proxyDialer.Dial("tcp", hostAddr)
		if err == nil {
			connAddr = hostAddr
			log.Printf("successfully connected to %s", hostAddr)
			break
		}
		log.Printf("failed to dial %s through proxy: %v", hostAddr, err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to dial %v through proxy: %v", d.hosts, err)
	}

	if d.tlsConfig != nil {
		return gocql.WrapTLS(ctx, conn, connAddr, d.tlsConfig)
	}

	return &gocql.DialedHost{
		Conn:            conn,
		DisableCoalesce: false,
	}, nil
}

func getProxyHostDialer(hosts []string, proxyAddr string) (proxyHostDialer *ProxyHostDialer, err error) {
	// Create SOCKS5 dialer
	socksDialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		log.Printf("Failed to create SOCKS5 dialer: %v", err)
		return nil, err
	}

	log.Println("proxyhostdialer is set")

	return &ProxyHostDialer{
		proxyDialer: socksDialer,
		hosts:       hosts,
	}, nil
}

func getProxyAddrFromEnv() (string, error) {
	proxyStr := cmp.Or(os.Getenv("HTTPS_PROXY"), os.Getenv("HTTP_PROXY"), os.Getenv("https_proxy"), os.Getenv("http_proxy"))
	if proxyStr == "" {
		// no env vars are set
		return "", nil
	}
	proxyURL, err := url.Parse(proxyStr)
	if err != nil || proxyURL.Host == "" {
		log.Printf("fails to parse %v. Returning the original value as address", proxyStr)
		return proxyStr, nil //nolint:nilerr // intentionally swallowing parse error. falling back to raw string
	}
	if proxyURL.Scheme == "http" || proxyURL.Scheme == "https" {
		return "", fmt.Errorf("http or https proxy should not be used")
	}
	return proxyURL.Host, nil
}

func NewClusterConfig(hosts []string) (newCluster *Cluster, err error) {
	return NewClusterConfigWithProxy(hosts, "")
}

func NewClusterConfigWithProxy(hosts []string, proxyAddr string) (newCluster *Cluster, err error) {
	// Setup proxy if necessary
	if proxyAddr == "" {
		proxyAddr, err = getProxyAddrFromEnv()
		if err != nil {
			return
		}
		log.Printf("proxy host set up is found: %v", proxyAddr)
	}
	var clusterHostDialer gocql.HostDialer
	if proxyAddr != "" {
		clusterHostDialer, err = getProxyHostDialer(hosts, proxyAddr)
		if err != nil {
			return nil, err
		}
	}

	// Create cluster
	if proxyAddr != "" {
		// Use a dummy IP here to avoid dns resolution failure
		// this is overridden in HostDialer
		hosts = []string{"127.0.0.1"}
	}
	cluster := gocql.NewCluster(hosts...)
	if clusterHostDialer != nil {
		cluster.HostDialer = clusterHostDialer
	}
	cluster.DisableInitialHostLookup = true
	cluster.NumConns = 1
	return &Cluster{
		Cluster:                cluster,
		SystemAuthKeyspaceName: "system_auth",
	}, nil
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
		tlsConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &cert, nil
		}
	}
	c.Cluster.SslOpts = &gocql.SslOptions{
		Config: tlsConfig,
		// This option is the inverse of tls.Config.InsecureSkipVerify. Setting it explicitly for clarity.
		EnableHostVerification: enableHostVerification,
	}

	// When using a custom HostDialer (ProxyHostDialer), gocql ignores SslOpts;
	// Dialer must handle TLS wrapping. Progating tlsConfig to ProxyHostDialer.
	if proxyHostDialer, ok := c.Cluster.HostDialer.(*ProxyHostDialer); ok {
		proxyHostDialer.tlsConfig = tlsConfig
	}

	return nil
}
