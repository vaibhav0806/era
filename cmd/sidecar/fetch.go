package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// fetchHandler retrieves a URL's body if the host is allowlisted. Non-text
// responses are rejected to discourage the agent from downloading binaries
// or scripts.
type fetchHandler struct {
	allow  *allowlist
	client *http.Client
	// maxBodyBytes caps the response size to avoid filling the runner's disk.
	maxBodyBytes int64
}

func newFetchHandler(allow *allowlist) http.Handler {
	return &fetchHandler{
		allow:        allow,
		client:       &http.Client{Timeout: 20 * time.Second},
		maxBodyBytes: 2 * 1024 * 1024, // 2 MB is plenty for a docs page
	}
}

func (f *fetchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	raw := r.URL.Query().Get("url")
	if raw == "" {
		http.Error(w, "missing 'url' query parameter", http.StatusBadRequest)
		return
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		http.Error(w, "invalid URL", http.StatusBadRequest)
		return
	}
	if !f.allow.allowed(u.Host) {
		http.Error(w, fmt.Sprintf("host not in allowlist: %s", u.Host), http.StatusForbidden)
		return
	}

	outReq, err := http.NewRequestWithContext(r.Context(), "GET", u.String(), nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	outReq.Header.Set("User-Agent", "era-sidecar/fetch")

	resp, err := f.client.Do(outReq)
	if err != nil {
		http.Error(w, "fetch upstream error: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !isTextContentType(ct) {
		http.Error(w, fmt.Sprintf("unsupported content-type: %s", ct), http.StatusUnsupportedMediaType)
		return
	}

	// Cap body size.
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(resp.Body, f.maxBodyBytes))
}

func isTextContentType(ct string) bool {
	// Strip parameters (e.g. "text/html; charset=utf-8" → "text/html")
	base := strings.TrimSpace(strings.SplitN(ct, ";", 2)[0])
	base = strings.ToLower(base)
	if strings.HasPrefix(base, "text/") {
		return true
	}
	switch base {
	case "application/json", "application/xml", "application/xhtml+xml",
		"application/javascript", "application/x-ndjson", "application/ld+json":
		return true
	}
	return false
}
