package githubpr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// TokenSource yields a GitHub App installation token. Satisfied by *githubapp.Client.
type TokenSource interface {
	InstallationToken(ctx context.Context) (string, error)
}

// Client calls the GitHub Pull Requests API.
type Client struct {
	tokens  TokenSource
	http    *http.Client
	baseURL string
}

// New returns a Client. baseURL is the GitHub API base (empty → https://api.github.com).
func New(baseURL string, tokens TokenSource) *Client {
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	return &Client{
		tokens:  tokens,
		http:    &http.Client{Timeout: 30 * time.Second},
		baseURL: baseURL,
	}
}

// DefaultBranch returns the default branch name for repo (owner/repo).
func (c *Client) DefaultBranch(ctx context.Context, repo string) (string, error) {
	req, err := c.newReq(ctx, "GET", "/repos/"+repo, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("get repo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("get repo %s: %d %s", repo, resp.StatusCode, string(body))
	}
	var body struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	return body.DefaultBranch, nil
}

func (c *Client) newReq(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	tok, err := c.tokens.InstallationToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("mint token: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+tok)
	req.Header.Set("Accept", "application/vnd.github+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}
