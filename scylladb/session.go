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
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
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
	// 2. Request a tunnel to the Cassandra node. addr ~ 1.2.3.4:9042
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

func NewClusterConfig(hosts []string) Cluster {
	cluster := gocql.NewCluster(hosts...)
	// Check proxy
	proxyStr := cmp.Or(os.Getenv("HTTPS_PROXY"), os.Getenv("HTTP_PROXY"))
	if proxyStr != "" {
		if !strings.HasPrefix(proxyStr, "http://") && !strings.HasPrefix(proxyStr, "https://") {
			proxyStr = "http://" + proxyStr
		}
		proxyURL, _ := url.Parse(proxyStr)
		cluster.Dialer = &ProxyDialer{ProxyURL: proxyURL}
	}
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
		tlsConfig.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &cert, error(nil)
		}
	}
	c.Cluster.SslOpts = &gocql.SslOptions{
		Config: tlsConfig,
		// This option is the inverse of tls.Config.InsecureSkipVerify. Setting it explicitly for clarity.
		EnableHostVerification: enableHostVerification,
	}
	return nil
}
