// Copyright RetailNext, Inc. 2026

package scylladb

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"

	"golang.org/x/net/proxy"
)

// Register the http and https schemes for proxy.FromURL
// Note that socks5 is already registered by the proxy package, so no need to register it here.
func init() {
	proxy.RegisterDialerType("http", newHTTPProxy)
	proxy.RegisterDialerType("https", newHTTPProxy)
}

type HTTPProxyDialer struct {
	proxyURL *url.URL
	forward  proxy.Dialer
}

type BufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (b *BufferedConn) Read(p []byte) (int, error) {
	return b.r.Read(p)
}

func newHTTPProxy(uri *url.URL, forward proxy.Dialer) (proxy.Dialer, error) {
	return &HTTPProxyDialer{proxyURL: uri, forward: forward}, nil
}

// proxyHost returns the proxy host:port, applying a default port based on
// scheme when the URL omits one.
func (h *HTTPProxyDialer) proxyHost() string {
	if h.proxyURL.Port() != "" {
		return h.proxyURL.Host
	}
	defaultPort := "80"
	if h.proxyURL.Scheme == "https" {
		defaultPort = "443"
	}
	return net.JoinHostPort(h.proxyURL.Hostname(), defaultPort)
}

// Dial connects to the address via the HTTP proxy.
func (h *HTTPProxyDialer) Dial(network, addr string) (net.Conn, error) {
	// Establish connection to the proxy
	conn, err := h.forward.Dial(network, h.proxyHost())
	if err != nil {
		return nil, err
	}

	// If the proxy URL uses https, wrap the connection with TLS
	if h.proxyURL.Scheme == "https" {
		host, _, err := net.SplitHostPort(h.proxyURL.Host)
		if err != nil {
			host = h.proxyURL.Host
		}
		tlsConn := tls.Client(conn, &tls.Config{ServerName: host})
		if err := tlsConn.Handshake(); err != nil {
			conn.Close()
			return nil, fmt.Errorf("TLS handshake with proxy failed: %v", err)
		}
		conn = tlsConn
	}

	// CONNECT request
	req := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: addr},
		Host:   addr,
		Header: make(http.Header),
	}

	// Handle proxy authentication if credentials are provided in the URL
	if u := h.proxyURL.User; u != nil {
		pass, _ := u.Password()
		creds := base64.StdEncoding.EncodeToString([]byte(u.Username() + ":" + pass))
		req.Header.Set("Proxy-Authorization", "Basic "+creds)
	}

	if err := req.Write(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send CONNECT request: %v", err)
	}

	// Read the response
	bufReader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(bufReader, req)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read CONNECT response: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, fmt.Errorf("proxy CONNECT request failed: %s", resp.Status)
	}

	log.Printf("HTTP CONNECT to %s successful", h.proxyURL.Host)

	if bufReader.Buffered() > 0 {
		// If there are any bytes buffered, it may interfere with the TLS handshake or subsequent protocol communication.
		log.Printf("Warning: Proxy sent %d unexpected bytes after CONNECT response. Using buffered reader", bufReader.Buffered())
		return &BufferedConn{Conn: conn, r: bufReader}, nil
	}

	return conn, nil
}

var (
	httpProxyEnv = &envOnce{
		names: []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy"},
	}
)

// envOnce looks up an environment variable (Borrowed from net/proxy/proxy.go).
type envOnce struct {
	names []string
	once  sync.Once
	val   string
}

func (e *envOnce) Get() string {
	e.once.Do(e.init)
	return e.val
}

func (e *envOnce) init() {
	for _, n := range e.names {
		e.val = os.Getenv(n)
		if e.val != "" {
			return
		}
	}
}
