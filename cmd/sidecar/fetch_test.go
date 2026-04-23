package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFetch_StaticAllowlistedHost(t *testing.T) {
	// Simulate a doc host the agent might fetch from.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>go context docs</html>"))
	}))
	defer backend.Close()

	a := newAllowlist()
	// Dynamically permit the backend's host so we can test the allowed path
	// without hardcoding a static-list entry that points at our test server.
	a.permit(mustHostFetch(t, backend.URL), 30*time.Second)

	h := newFetchHandler(a)
	req := httptest.NewRequest("POST", "/fetch?url="+backend.URL+"/go/context", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, 200, rec.Code)
	body, _ := io.ReadAll(rec.Body)
	require.Contains(t, string(body), "go context docs")
}

func TestFetch_DisallowedHost(t *testing.T) {
	a := newAllowlist()
	h := newFetchHandler(a)
	req := httptest.NewRequest("POST", "/fetch?url=https://evil.example.com/exfil", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, 403, rec.Code)
	body, _ := io.ReadAll(rec.Body)
	require.Contains(t, string(body), "not in allowlist")
}

func TestFetch_NonTextRejected(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte{0xde, 0xad, 0xbe, 0xef})
	}))
	defer backend.Close()

	a := newAllowlist()
	a.permit(mustHostFetch(t, backend.URL), 30*time.Second)
	h := newFetchHandler(a)
	req := httptest.NewRequest("POST", "/fetch?url="+backend.URL+"/bin", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, 415, rec.Code)
	body, _ := io.ReadAll(rec.Body)
	require.Contains(t, string(body), "unsupported content-type")
}

func TestFetch_MissingURL(t *testing.T) {
	a := newAllowlist()
	h := newFetchHandler(a)
	req := httptest.NewRequest("POST", "/fetch", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, 400, rec.Code)
}

func TestFetch_InvalidURL(t *testing.T) {
	a := newAllowlist()
	h := newFetchHandler(a)
	req := httptest.NewRequest("POST", "/fetch?url=not-a-valid-url", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, 400, rec.Code)
}

func TestFetch_ExpiredPermit(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	}))
	defer backend.Close()

	a := newAllowlist()
	a.permit(mustHostFetch(t, backend.URL), 50*time.Millisecond)
	h := newFetchHandler(a)

	// Fetch succeeds while permit is live
	req1 := httptest.NewRequest("POST", "/fetch?url="+backend.URL+"/", nil)
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req1)
	require.Equal(t, 200, rec1.Code)

	// Wait for permit to expire
	time.Sleep(100 * time.Millisecond)

	// Fetch now denied
	req2 := httptest.NewRequest("POST", "/fetch?url="+backend.URL+"/", nil)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	require.Equal(t, 403, rec2.Code)
}

// mustHostFetch extracts host from URL for the fetch test.
func mustHostFetch(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u.Host
}
