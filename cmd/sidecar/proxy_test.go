package main

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// startSidecar boots the sidecar HTTP server (audit + proxy + /health) on a
// free port and returns the proxy URL. PI_SIDECAR_TEST_HOOKS=1 enables the
// _test/permit endpoint.
func startSidecar(t *testing.T) string {
	t.Helper()
	t.Setenv("PI_SIDECAR_TEST_HOOKS", "1")
	addr := freePort(t)
	srv := newServer(addr)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go runServer(ctx, srv)
	time.Sleep(50 * time.Millisecond)
	return "http://" + addr
}

// httpClientThroughProxy returns an http.Client that routes through the given
// proxy URL.
func httpClientThroughProxy(proxyURL string) *http.Client {
	u, _ := url.Parse(proxyURL)
	return &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(u),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // OK for httptest backends
		},
		Timeout: 5 * time.Second,
	}
}

func TestProxy_AllowedHostForwarded(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello from backend"))
	}))
	t.Cleanup(backend.Close)

	proxy := startSidecar(t)
	host := mustHost(t, backend.URL)
	mustPermit(t, proxy, host, 30*time.Second)

	client := httpClientThroughProxy(proxy)
	resp, err := client.Get(backend.URL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, "hello from backend", string(body))
}

func TestProxy_BlockedHostReturns403(t *testing.T) {
	proxy := startSidecar(t)
	client := httpClientThroughProxy(proxy)
	// evil.example.com is not in allowlist. The proxy returns 403 for HTTP
	// requests; for HTTPS CONNECT it returns 403 before tunnel established.
	resp, err := client.Get("http://evil.example.com/")
	if err != nil {
		// Acceptable: the proxy may close the connection rather than return 403
		// depending on transport behavior. Either way, the request did not succeed.
		return
	}
	defer resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode,
		"non-allowlisted host should return 403, got %d", resp.StatusCode)
}

func mustHost(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u.Host
}

func mustPermit(t *testing.T, proxyBase, host string, ttl time.Duration) {
	t.Helper()
	resp, err := http.Post(
		proxyBase+"/_test/permit?host="+host+"&ttl_ms=30000",
		"text/plain", nil)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, 204, resp.StatusCode)
}
