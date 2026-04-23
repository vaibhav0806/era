package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeTavily serves canned responses for testing.
func fakeTavily(t *testing.T, fn func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(fn))
}

func TestSearch_ForwardsQueryAndInjectsAuth(t *testing.T) {
	var receivedAuth string
	var receivedBody map[string]interface{}
	tavily := fakeTavily(t, func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"query":"test","results":[{"title":"A","url":"https://docs.example.com/a","content":"x","score":0.9}],"response_time":0.1}`))
	})
	defer tavily.Close()

	a := newAllowlist()
	h := newSearchHandler(tavily.URL, "sk-tvly-test", a)
	req := httptest.NewRequest("POST", "/search",
		strings.NewReader(`{"query":"how to use go context"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, 200, rec.Code)
	require.Equal(t, "Bearer sk-tvly-test", receivedAuth)
	require.Equal(t, "how to use go context", receivedBody["query"])

	// Response body pass-through
	body, _ := io.ReadAll(rec.Body)
	require.Contains(t, string(body), "docs.example.com")

	// Result host was permit'd on the allowlist
	require.True(t, a.allowed("docs.example.com"))
}

func TestSearch_PermitHasLimitedTTL(t *testing.T) {
	tavily := fakeTavily(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"query":"x","results":[{"url":"https://temp.example.com/"}]}`))
	})
	defer tavily.Close()

	a := newAllowlist()
	h := newSearchHandler(tavily.URL, "k", a)
	h.(*searchHandler).permitTTL = 50 * time.Millisecond // inject short TTL for test
	req := httptest.NewRequest("POST", "/search",
		strings.NewReader(`{"query":"x"}`))
	h.ServeHTTP(httptest.NewRecorder(), req)

	require.True(t, a.allowed("temp.example.com"))
	time.Sleep(70 * time.Millisecond)
	require.False(t, a.allowed("temp.example.com"))
}

func TestSearch_MissingAPIKeyReturns503(t *testing.T) {
	a := newAllowlist()
	h := newSearchHandler("", "", a) // empty key
	req := httptest.NewRequest("POST", "/search", strings.NewReader(`{"query":"x"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, 503, rec.Code)
	body, _ := io.ReadAll(rec.Body)
	require.Contains(t, string(body), "Tavily API key not configured")
}

func TestSearch_InvalidQueryReturns400(t *testing.T) {
	a := newAllowlist()
	h := newSearchHandler("http://unused", "k", a)
	req := httptest.NewRequest("POST", "/search", strings.NewReader(`{"notquery":"x"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, 400, rec.Code)
}
