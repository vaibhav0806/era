//go:build e2e
// +build e2e

package e2e_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vaibhav0806/era/internal/githubapp"
)

// githubAppTokenSource constructs a TokenSource from env vars. Used by every
// M2+ e2e test that needs per-task tokens. Skips the test if App env is not
// configured (CI without App credentials).
func githubAppTokenSource(t *testing.T) *githubapp.Client {
	t.Helper()
	appID := os.Getenv("PI_GITHUB_APP_ID")
	instID := os.Getenv("PI_GITHUB_APP_INSTALLATION_ID")
	key := os.Getenv("PI_GITHUB_APP_PRIVATE_KEY")
	if appID == "" || instID == "" || key == "" {
		t.Skip("PI_GITHUB_APP_{ID,INSTALLATION_ID,PRIVATE_KEY} must all be set")
	}

	cfg := githubapp.Config{
		PrivateKeyBase64: key,
	}
	fmtErr := func(k, v string) {
		require.FailNow(t, "bad env", "%s=%q is not an int64", k, v)
	}
	var ok bool
	if cfg.AppID, ok = parseInt64(appID); !ok {
		fmtErr("PI_GITHUB_APP_ID", appID)
	}
	if cfg.InstallationID, ok = parseInt64(instID); !ok {
		fmtErr("PI_GITHUB_APP_INSTALLATION_ID", instID)
	}

	client, err := githubapp.New(cfg)
	require.NoError(t, err)
	return client
}

func parseInt64(s string) (int64, bool) {
	var v int64
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err == nil
}

// mintGhToken is a convenience for tests that need a concrete token for
// direct git ops (e.g. cleanup pushes).
func mintGhToken(t *testing.T) string {
	t.Helper()
	tok, err := githubAppTokenSource(t).InstallationToken(context.Background())
	require.NoError(t, err)
	return tok
}
