// Copyright RetailNext, Inc. 2026

package scylladb

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"

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
	proxyScheme string
	tlsConfig   *tls.Config
	hostMap     map[string]string // maps the dummy host to the actual host
}

func (d *ProxyHostDialer) DialHost(ctx context.Context, host *gocql.HostInfo) (dialedHost *gocql.DialedHost, err error) {
	// Construct connection through proxy

	// Determine the real host address to connect to based on the dummy host
	realHost, ok := d.hostMap[host.ConnectAddress().String()]
	if !ok {
		// No mapping found, use the original host address
		realHost = host.ConnectAddressAndPort()
	}

	log.Printf("Asked to connect to hosts %v through proxy", realHost)
	conn, err := d.proxyDialer.Dial("tcp", realHost)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %v through proxy: %w", realHost, err)
	}
	log.Printf("successfully connected to %s", realHost)

	if d.tlsConfig != nil {
		return gocql.WrapTLS(ctx, conn, realHost, d.tlsConfig)
	}

	return &gocql.DialedHost{
		Conn:            conn,
		DisableCoalesce: false,
	}, nil
}

func getProxyHostDialer(hosts []string, proxyAddr string) (proxyHostDialer *ProxyHostDialer, dummyHosts []string, err error) {
	proxyURL, err := url.Parse(proxyAddr)
	if err != nil {
		log.Printf("fails to parse proxy address %v. Returning error", proxyAddr)
		return nil, nil, fmt.Errorf("failed to parse proxy address %v: %w", proxyAddr, err)
	}

	if proxyURL.Host == "" {
		log.Printf("No host found after parsing proxy URL %v. Trying again after assuming http scheme", proxyAddr)
		newProxyStr := "http://" + proxyAddr
		proxyURL, err = url.Parse(newProxyStr)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse proxy address %v: %w", proxyAddr, err)
		}
	}

	// Create dialer
	proxyDialer, err := proxy.FromURL(proxyURL, proxy.Direct)
	if err != nil {
		log.Printf("Failed to create proxy dialer: %v", err)
		return nil, nil, err
	}
	hostMap := createProxyHostMap(hosts)
	dummyHosts = make([]string, 0, len(hosts))
	for dummyHost := range hostMap {
		dummyHosts = append(dummyHosts, dummyHost)
	}

	log.Println("proxyhostdialer is set")

	return &ProxyHostDialer{
		proxyDialer: proxyDialer,
		proxyScheme: proxyURL.Scheme,
		hostMap:     hostMap,
	}, dummyHosts, nil
}

func createProxyHostMap(hosts []string) map[string]string {
	hostMap := make(map[string]string)
	dummyIndex := 0
	for _, hostName := range hosts {
		dummyHost := fmt.Sprintf("127.0.%d.%d", dummyIndex/254, dummyIndex%254+1)
		hostPart, portPart, err := net.SplitHostPort(hostName)
		if err != nil {
			// If there's an error, it means there's no port part, so we use the whole hostName as hostPart
			hostPart = hostName
			portPart = "9042" // default port
		}
		hostMap[dummyHost] = net.JoinHostPort(hostPart, portPart)
		dummyIndex++
	}
	return hostMap
}

func NewClusterConfig(hosts []string) (newCluster *Cluster, err error) {
	return NewClusterConfigWithProxy(hosts, "")
}

func NewClusterConfigWithProxy(hosts []string, proxyAddr string) (newCluster *Cluster, err error) {
	var clusterHostDialer gocql.HostDialer

	// if proxyAddr is not provided as an argument, check environment variables
	if proxyAddr == "" {
		proxyAddr = httpProxyEnv.Get()
	}
	if proxyAddr != "" {
		// Use dummy hosts and a custom HostDialer to route through the proxy
		clusterHostDialer, hosts, err = getProxyHostDialer(hosts, proxyAddr)
		if err != nil {
			return nil, err
		}

	}

	// Create cluster
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
	// Dialer must handle TLS wrapping. Propagating tlsConfig to ProxyHostDialer.
	if proxyHostDialer, ok := c.Cluster.HostDialer.(*ProxyHostDialer); ok {
		proxyHostDialer.tlsConfig = tlsConfig
	}

	return nil
}
