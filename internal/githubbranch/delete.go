// Package githubbranch hits GitHub's DELETE /repos/.../git/refs/heads/<branch>
// endpoint to remove a pushed branch. Used by queue.Queue's RejectTask path
// when a needs_review task is rejected.
package githubbranch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Tokener yields a GitHub App installation token. Satisfied by *githubapp.Client.
type Tokener interface {
	InstallationToken(ctx context.Context) (string, error)
}

// Client calls the GitHub Refs API to delete branches.
type Client struct {
	base    string
	tokener Tokener
	http    *http.Client
}

// New returns a Client. baseURL is the GitHub API base (empty → https://api.github.com).
func New(baseURL string, t Tokener) *Client {
	b := strings.TrimRight(baseURL, "/")
	if b == "" {
		b = "https://api.github.com"
	}
	return &Client{base: b, tokener: t, http: &http.Client{Timeout: 15 * time.Second}}
}

// DeleteBranch removes `branch` on `repo` (owner/name). Treats 404 as success
// (idempotent — already-gone is indistinguishable from just-deleted for us).
func (c *Client) DeleteBranch(ctx context.Context, repo, branch string) error {
	tok, err := c.tokener.InstallationToken(ctx)
	if err != nil {
		return fmt.Errorf("installation token: %w", err)
	}
	url := fmt.Sprintf("%s/repos/%s/git/refs/heads/%s", c.base, repo, branch)
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("github delete %d: %s", resp.StatusCode, string(body))
}
