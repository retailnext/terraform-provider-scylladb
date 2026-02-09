// Copyright RetailNext, Inc. 2026

package scylladb

import (
	"bufio"
	"cmp"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
	"golang.org/x/net/proxy"
)

type Cluster struct {
	Cluster                *gocql.ClusterConfig
	SystemAuthKeyspaceName string
	Session                *gocql.Session
}

type ProxyDialer struct {
	ProxyURL *url.URL
}

type BufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (b *BufferedConn) Read(p []byte) (int, error) {
	return b.r.Read(p)
}

func (p *ProxyDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	// 1. Dial the proxy server itself
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}
	var d net.Dialer
	proxyConn, err := d.DialContext(ctx, "tcp", p.ProxyURL.Host)
	if err != nil {
		return nil, err
	}
	// 2. Request a tunnel to the scylla node. addr ~ 1.2.3.4:9042
	req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", addr, addr)
	if _, err := proxyConn.Write([]byte(req)); err != nil {
		proxyConn.Close()
		return nil, err
	}

	// 3. Read the response from Tinyproxy
	br := bufio.NewReader(proxyConn)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
	if err != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("failed to read proxy response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		proxyConn.Close()
		return nil, fmt.Errorf("proxy refused connection: %s", resp.Status)
	}

	if br.Buffered() > 0 {
		return &BufferedConn{Conn: proxyConn, r: br}, nil
	}

	// 4. Return the hijacked connection to gocql
	return proxyConn, nil
}

func getProxyDialer(proxyStr, nameserverIP string) (proxyDialer proxy.ContextDialer, err error) {
	if !strings.HasPrefix(proxyStr, "http://") && !strings.HasPrefix(proxyStr, "https://") && !strings.HasPrefix(proxyStr, "socks5://") {
		proxyStr = "http://" + proxyStr
	}
	proxyURL, err := url.Parse(proxyStr)
	if err != nil {
		return nil, err
	}

	if proxyURL.Scheme == "http" || proxyURL.Scheme == "https" {
		// HTTP CONNECT
		proxyDialer = &ProxyDialer{ProxyURL: proxyURL}
	} else {
		// SOCKS5
		dialer, err := proxy.FromURL(proxyURL, proxy.Direct)
		if err != nil {
			return nil, err
		}
		var ok bool
		proxyDialer, ok = dialer.(proxy.ContextDialer)
		if !ok {
			return proxyDialer, fmt.Errorf("fails to cast dialer to contextDialer")
		}
	}

	if nameserverIP == "" {
		return proxyDialer, nil
	}

	// Override default DNS resolver to route through proxy
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			// Force TCP because proxies don't support UDP
			dnsAddr := fmt.Sprintf("%s:53", nameserverIP)
			log.Printf("DNS resolver: routing %s query to %s through proxy\n", network, dnsAddr)
			conn, err := proxyDialer.DialContext(ctx, "tcp", dnsAddr)
			if err != nil {
				fmt.Printf("DNS resolver error: %v\n", err)
			}
			return conn, err
		},
	}

	return proxyDialer, err

}

// NewClusterConfig creates a new ScyllaDB cluster configuration.
// It reads proxy settings from HTTPS_PROXY or HTTP_PROXY environment variables.
// DNS resolution is not routed through the proxy when using this function.
func NewClusterConfig(hosts []string) (newCluster *Cluster, err error) {
	return NewClusterConfigWithDns(hosts, "")
}

// NewClusterConfigWithDns creates a new ScyllaDB cluster configuration with proxy and DNS support.
//
// Parameters:
//   - hosts: ScyllaDB host addresses (can be hostnames or IPs with ports, e.g., "scylla.internal:9142")
//   - dnsAddress: DNS server IP address for resolving hostnames through the proxy.
//     When connecting through a proxy (HTTPS_PROXY or HTTP_PROXY env vars), hostnames
//     need to be resolved on the proxy's network. This address specifies the DNS server
//     accessible from the proxy host (e.g., "127.0.0.53" for systemd-resolved on the proxy).
//     If empty, DNS resolution is not routed through the proxy.
//
// Proxy support:
//   - SOCKS5 proxy: Set HTTPS_PROXY=socks5://host:port (e.g., via SSH tunnel: ssh -D 8888 user@host)
//   - HTTP CONNECT proxy: Set HTTPS_PROXY=http://host:port (e.g., tinyproxy)
//
// Example:
//
//	// With SOCKS5 proxy and DNS through proxy
//	cluster, err := NewClusterConfigWithDns([]string{"scylla.internal:9142"}, "127.0.0.53")
func NewClusterConfigWithDns(hosts []string, dnsAddress string) (newCluster *Cluster, err error) {
	// Check proxy first and set up DNS resolver before creating cluster
	proxyStr := cmp.Or(os.Getenv("HTTPS_PROXY"), os.Getenv("HTTP_PROXY"), os.Getenv("https_proxy"), os.Getenv("http_proxy"))
	var clusterDialer gocql.Dialer

	if proxyStr != "" {
		clusterDialer, err = getProxyDialer(proxyStr, dnsAddress)
		if err != nil {
			return nil, err
		}
	}

	// Create cluster after DNS resolver is configured
	cluster := gocql.NewCluster(hosts...)
	if clusterDialer != nil {
		cluster.Dialer = clusterDialer
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
	return nil
}
