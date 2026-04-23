package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// searchHandler proxies search requests to the Tavily API. The API key lives
// here (from sidecar env); the agent inside the container never sees it.
// Each result URL's host is permitted on the allowlist for a short window so
// that /fetch (M2-11) can retrieve it.
type searchHandler struct {
	tavilyURL string // base URL; "" means use default https://api.tavily.com
	apiKey    string
	allow     *allowlist
	permitTTL time.Duration
	client    *http.Client
}

func newSearchHandler(tavilyURL, apiKey string, allow *allowlist) http.Handler {
	return &searchHandler{
		tavilyURL: tavilyURL,
		apiKey:    apiKey,
		allow:     allow,
		permitTTL: 10 * time.Minute,
		client:    &http.Client{Timeout: 20 * time.Second},
	}
}

type searchRequest struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results,omitempty"`
}

type searchResponse struct {
	Query        string         `json:"query"`
	Results      []searchResult `json:"results"`
	ResponseTime float64        `json:"response_time"`
}

type searchResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

func (s *searchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.apiKey == "" {
		http.Error(w, "Tavily API key not configured (set PI_SIDECAR_TAVILY_API_KEY)", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req searchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Query == "" {
		http.Error(w, "invalid request: expected JSON {\"query\":\"...\"}", http.StatusBadRequest)
		return
	}
	if req.MaxResults == 0 {
		req.MaxResults = 5
	}

	base := s.tavilyURL
	if base == "" {
		base = "https://api.tavily.com"
	}
	endpoint := base + "/search"

	body, _ := json.Marshal(req)
	outReq, err := http.NewRequestWithContext(r.Context(), "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	outReq.Header.Set("Content-Type", "application/json")
	outReq.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.client.Do(outReq)
	if err != nil {
		slog.Error("tavily call", "err", err)
		http.Error(w, "search upstream error: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	buf, _ := io.ReadAll(resp.Body)

	// Parse to extract result hosts for the permit; then pass through.
	var parsed searchResponse
	if err := json.Unmarshal(buf, &parsed); err == nil {
		for _, res := range parsed.Results {
			if u, err := url.Parse(res.URL); err == nil && u.Host != "" {
				s.allow.permit(u.Host, s.permitTTL)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(buf)
}
