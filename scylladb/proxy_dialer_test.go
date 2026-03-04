// Copyright RetailNext, Inc. 2026

package scylladb

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPProxyDialer_proxyHost(t *testing.T) {
	tests := []struct {
		rawURL string
		want   string
	}{
		{"http://proxy.example.com", "proxy.example.com:80"},
		{"https://proxy.example.com", "proxy.example.com:443"},
		{"http://proxy.example.com:3128", "proxy.example.com:3128"},
		{"https://proxy.example.com:8443", "proxy.example.com:8443"},
	}
	for _, tc := range tests {
		u, err := url.Parse(tc.rawURL)
		require.NoError(t, err)
		d := &HTTPProxyDialer{proxyURL: u}
		assert.Equal(t, tc.want, d.proxyHost(), "url: %s", tc.rawURL)
	}
}

// mockDialer implements proxy.Dialer for testing.
type mockDialer struct {
	conn net.Conn
	err  error
}

func (m *mockDialer) Dial(network, addr string) (net.Conn, error) {
	return m.conn, m.err
}

// readCONNECTRequest reads an HTTP CONNECT request from conn and returns it.
func readCONNECTRequest(conn net.Conn) (*http.Request, error) {
	return http.ReadRequest(bufio.NewReader(conn))
}

// drainCONNECTRequest reads and discards an HTTP CONNECT request from conn.
func drainCONNECTRequest(conn net.Conn) error {
	_, err := readCONNECTRequest(conn)
	return err
}

func TestHTTPProxyDialer_Dial_ForwardDialError(t *testing.T) {
	dialer := &HTTPProxyDialer{
		proxyURL: &url.URL{Scheme: "http", Host: "proxy:8080"},
		forward:  &mockDialer{err: fmt.Errorf("connection refused")},
	}

	_, err := dialer.Dial("tcp", "target:9042")
	assert.Error(t, err)
}

func TestHTTPProxyDialer_Dial_WriteRequestError(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	// Close server side immediately so the client's Write fails.
	serverConn.Close()

	dialer := &HTTPProxyDialer{
		proxyURL: &url.URL{Scheme: "http", Host: "proxy:8080"},
		forward:  &mockDialer{conn: clientConn},
	}

	_, err := dialer.Dial("tcp", "target:9042")
	assert.Error(t, err)
}

func TestHTTPProxyDialer_Dial_NonOKResponse(t *testing.T) {
	clientConn, serverConn := net.Pipe()

	go func() {
		defer serverConn.Close()
		if err := drainCONNECTRequest(serverConn); err != nil {
			return
		}
		fmt.Fprint(serverConn, "HTTP/1.1 407 Proxy Authentication Required\r\n\r\n")
	}()

	dialer := &HTTPProxyDialer{
		proxyURL: &url.URL{Scheme: "http", Host: "proxy:8080"},
		forward:  &mockDialer{conn: clientConn},
	}

	_, err := dialer.Dial("tcp", "target:9042")
	assert.ErrorContains(t, err, "proxy CONNECT request failed")
}

func TestHTTPProxyDialer_Dial_Success(t *testing.T) {
	clientConn, serverConn := net.Pipe()

	go func() {
		defer serverConn.Close()
		if err := drainCONNECTRequest(serverConn); err != nil {
			return
		}
		fmt.Fprint(serverConn, "HTTP/1.1 200 OK\r\n\r\n")
	}()

	dialer := &HTTPProxyDialer{
		proxyURL: &url.URL{Scheme: "http", Host: "proxy:8080"},
		forward:  &mockDialer{conn: clientConn},
	}

	conn, err := dialer.Dial("tcp", "target:9042")
	require.NoError(t, err)
	conn.Close()

	// Should be a plain conn, not BufferedConn.
	_, isBuffered := conn.(*BufferedConn)
	assert.False(t, isBuffered)
}

func TestHTTPProxyDialer_Dial_SuccessWithBufferedBytes(t *testing.T) {
	clientConn, serverConn := net.Pipe()

	go func() {
		defer serverConn.Close()
		if err := drainCONNECTRequest(serverConn); err != nil {
			return
		}
		// Write response and extra bytes in a single write so they arrive together
		// in the client's bufio.Reader buffer.
		fmt.Fprint(serverConn, "HTTP/1.1 200 OK\r\n\r\nextra")
	}()

	dialer := &HTTPProxyDialer{
		proxyURL: &url.URL{Scheme: "http", Host: "proxy:8080"},
		forward:  &mockDialer{conn: clientConn},
	}

	conn, err := dialer.Dial("tcp", "target:9042")
	require.NoError(t, err)
	defer conn.Close()

	// Extra bytes were buffered, so Dial should return a BufferedConn.
	_, isBuffered := conn.(*BufferedConn)
	assert.True(t, isBuffered)
}

func TestHTTPProxyDialer_Dial_ProxyAuthorizationHeader(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		wantAuth string
	}{
		{
			name:     "username and password",
			rawURL:   "http://user:secret@proxy:8080",
			wantAuth: "Basic dXNlcjpzZWNyZXQ=", // base64("user:secret")
		},
		{
			name:     "username only, no password",
			rawURL:   "http://user@proxy:8080",
			wantAuth: "Basic dXNlcjo=", // base64("user:")
		},
		{
			name:     "no credentials",
			rawURL:   "http://proxy:8080",
			wantAuth: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, err := url.Parse(tc.rawURL)
			require.NoError(t, err)

			clientConn, serverConn := net.Pipe()
			var capturedAuth string

			go func() {
				defer serverConn.Close()
				req, err := readCONNECTRequest(serverConn)
				if err != nil {
					return
				}
				capturedAuth = req.Header.Get("Proxy-Authorization")
				fmt.Fprint(serverConn, "HTTP/1.1 200 OK\r\n\r\n")
			}()

			dialer := &HTTPProxyDialer{
				proxyURL: u,
				forward:  &mockDialer{conn: clientConn},
			}

			conn, err := dialer.Dial("tcp", "target:9042")
			require.NoError(t, err)
			conn.Close()

			assert.Equal(t, tc.wantAuth, capturedAuth)
		})
	}
}
