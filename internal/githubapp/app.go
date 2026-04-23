// Package githubapp holds a GitHub App client that mints per-installation
// access tokens on demand and caches them until they expire.
//
// The App's private key and identifiers live in orchestrator config (loaded
// from env). Installation tokens have 1-hour TTL and contents:write scope
// only on repos where the App is installed. They replace the long-lived
// classic PAT used in M0/M1.
package githubapp

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Config holds the static App identifiers plus the private key. PrivateKey
// is passed base64-encoded (single-line .env safe); New() decodes + parses.
type Config struct {
	AppID            int64
	InstallationID   int64
	PrivateKeyBase64 string
	// APIBaseURL is the GitHub API endpoint. Defaults to "https://api.github.com".
	// Tests override with an httptest server URL.
	APIBaseURL string
}

// Client mints and caches installation tokens for a single App+installation.
// Safe for concurrent use.
type Client struct {
	cfg     Config
	key     *rsa.PrivateKey
	baseURL string
	http    *http.Client

	mu    sync.Mutex
	token string
	exp   time.Time
}

func New(cfg Config) (*Client, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(cfg.PrivateKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("decode private key base64: %w", err)
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key pem: %w", err)
	}
	base := strings.TrimRight(cfg.APIBaseURL, "/")
	if base == "" {
		base = "https://api.github.com"
	}
	return &Client{
		cfg:     cfg,
		key:     key,
		baseURL: base,
		http:    &http.Client{Timeout: 15 * time.Second},
	}, nil
}

// InstallationToken returns a valid installation token, minting a new one if
// the cache is empty or the cached token expires within the next minute.
// Safe to call concurrently.
func (c *Client) InstallationToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Refresh if empty or within 60s of expiry
	if c.token != "" && time.Now().Add(60*time.Second).Before(c.exp) {
		return c.token, nil
	}
	tok, exp, err := c.mint(ctx)
	if err != nil {
		return "", err
	}
	c.token, c.exp = tok, exp
	return tok, nil
}

func (c *Client) mint(ctx context.Context) (string, time.Time, error) {
	appJWT, err := c.signJWT()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign app JWT: %w", err)
	}
	url := fmt.Sprintf("%s/app/installations/%d/access_tokens", c.baseURL, c.cfg.InstallationID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Authorization", "Bearer "+appJWT)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("POST access_tokens: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", time.Time{}, fmt.Errorf("github api %d: %s", resp.StatusCode, string(body))
	}
	var parsed struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", time.Time{}, fmt.Errorf("parse response: %w; body=%s", err, string(body))
	}
	if parsed.Token == "" {
		return "", time.Time{}, errors.New("github returned empty token")
	}
	return parsed.Token, parsed.ExpiresAt, nil
}

func (c *Client) signJWT() (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    fmt.Sprintf("%d", c.cfg.AppID),
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)), // clock skew
		ExpiresAt: jwt.NewNumericDate(now.Add(9 * time.Minute)),   // max 10 min per GitHub
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(c.key)
}
