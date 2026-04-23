package githubbranch_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vaibhav0806/era/internal/githubbranch"
)

type staticTokener struct{ tok string }

func (s staticTokener) InstallationToken(ctx context.Context) (string, error) {
	return s.tok, nil
}

func TestDeleteBranch_Success(t *testing.T) {
	var receivedAuth, receivedPath, receivedMethod string
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		receivedPath = r.URL.Path
		receivedMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer gh.Close()

	c := githubbranch.New(gh.URL, staticTokener{tok: "test-token"})
	err := c.DeleteBranch(context.Background(), "alice/bob", "agent/1/foo")
	require.NoError(t, err)
	require.Equal(t, "DELETE", receivedMethod)
	require.Equal(t, "/repos/alice/bob/git/refs/heads/agent/1/foo", receivedPath)
	require.Equal(t, "Bearer test-token", receivedAuth)
}

func TestDeleteBranch_NotFoundTolerated(t *testing.T) {
	// If the branch is already gone, GitHub returns 422 or 404.
	// We treat 404 as a success (idempotent delete).
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Reference does not exist"}`))
	}))
	defer gh.Close()

	c := githubbranch.New(gh.URL, staticTokener{tok: "t"})
	err := c.DeleteBranch(context.Background(), "a/b", "gone")
	require.NoError(t, err, "404 on delete should be treated as idempotent success")
}

func TestDeleteBranch_UnauthorizedPropagates(t *testing.T) {
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	defer gh.Close()

	c := githubbranch.New(gh.URL, staticTokener{tok: "bad"})
	err := c.DeleteBranch(context.Background(), "a/b", "foo")
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
}

func TestDeleteBranch_TokenerError(t *testing.T) {
	c := githubbranch.New("http://unused", errorTokener{err: errStatic("mint failed")})
	err := c.DeleteBranch(context.Background(), "a/b", "x")
	require.Error(t, err)
	require.Contains(t, err.Error(), "mint failed")
}

type errorTokener struct{ err error }

func (e errorTokener) InstallationToken(ctx context.Context) (string, error) {
	return "", e.err
}

type errStatic string

func (e errStatic) Error() string { return string(e) }
