package githubpr_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vaibhav0806/era/internal/githubpr"
)

type fakeTokens struct{ tok string }

func (f *fakeTokens) InstallationToken(ctx context.Context) (string, error) { return f.tok, nil }

func TestNew_DefaultsPopulated(t *testing.T) {
	c := githubpr.New("", &fakeTokens{tok: "ghs_xxx"})
	require.NotNil(t, c)
}

func TestDefaultBranch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "GET", r.Method)
		require.Equal(t, "/repos/owner/repo", r.URL.Path)
		require.Equal(t, "token ghs_test", r.Header.Get("Authorization"))
		require.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"default_branch":"master"}`))
	}))
	defer srv.Close()

	c := githubpr.New(srv.URL, &fakeTokens{tok: "ghs_test"})

	got, err := c.DefaultBranch(context.Background(), "owner/repo")
	require.NoError(t, err)
	require.Equal(t, "master", got)
}

func TestDefaultBranch_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, 404)
	}))
	defer srv.Close()

	c := githubpr.New(srv.URL, &fakeTokens{tok: "ghs_test"})

	_, err := c.DefaultBranch(context.Background(), "owner/repo")
	require.Error(t, err)
	require.Contains(t, err.Error(), "404")
}
