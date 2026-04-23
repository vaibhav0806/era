package githubapp_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vaibhav0806/era/internal/githubapp"
)

// generateTestKey creates an RSA private key and returns it both as a parsed
// key and as the base64-encoded PEM string that Config.PrivateKeyBase64 expects.
func generateTestKey(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return key, base64.StdEncoding.EncodeToString(pemBytes)
}

// githubMock simulates GitHub's /app/installations/<id>/access_tokens endpoint.
func githubMock(t *testing.T, verifyJWT func(jwt string)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		require.True(t, strings.HasPrefix(auth, "Bearer "), "should be Bearer JWT")
		if verifyJWT != nil {
			verifyJWT(strings.TrimPrefix(auth, "Bearer "))
		}
		// Extract installation ID from path like /app/installations/12345/access_tokens
		require.Contains(t, r.URL.Path, "/app/installations/")
		require.Contains(t, r.URL.Path, "/access_tokens")

		// Return a plausible installation token response
		body := fmt.Sprintf(`{
			"token": "ghs_faketoken%d",
			"expires_at": "%s",
			"permissions": {"contents": "write", "metadata": "read"},
			"repository_selection": "all"
		}`, time.Now().Unix(), time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))
}

func TestClient_MintInstallationToken(t *testing.T) {
	_, keyB64 := generateTestKey(t)

	var capturedJWT string
	gh := githubMock(t, func(j string) { capturedJWT = j })
	defer gh.Close()

	c, err := githubapp.New(githubapp.Config{
		AppID:            1234,
		InstallationID:   5678,
		PrivateKeyBase64: keyB64,
		APIBaseURL:       gh.URL, // override default https://api.github.com
	})
	require.NoError(t, err)

	ctx := context.Background()
	tok, err := c.InstallationToken(ctx)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(tok, "ghs_faketoken"), "expected ghs_ prefix; got %q", tok)

	// JWT should have 3 segments (header.payload.sig)
	require.Len(t, strings.Split(capturedJWT, "."), 3, "JWT must have header.payload.signature")
}

func TestClient_CachesTokenWithinTTL(t *testing.T) {
	_, keyB64 := generateTestKey(t)

	callCount := 0
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// Return a token that expires far in the future
		_, _ = io.WriteString(w, fmt.Sprintf(`{
			"token": "ghs_t%d",
			"expires_at": "%s",
			"permissions": {"contents": "write"}
		}`, callCount, time.Now().Add(time.Hour).UTC().Format(time.RFC3339)))
	}))
	defer gh.Close()

	c, err := githubapp.New(githubapp.Config{
		AppID: 1, InstallationID: 1, PrivateKeyBase64: keyB64, APIBaseURL: gh.URL,
	})
	require.NoError(t, err)
	ctx := context.Background()

	t1, err := c.InstallationToken(ctx)
	require.NoError(t, err)
	t2, err := c.InstallationToken(ctx)
	require.NoError(t, err)
	require.Equal(t, t1, t2, "second call within TTL should reuse cached token")
	require.Equal(t, 1, callCount, "should hit GitHub only once")
}

func TestClient_RefreshesExpiredToken(t *testing.T) {
	_, keyB64 := generateTestKey(t)

	callCount := 0
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// Return a token that's already stale (expires_at in the past)
		_, _ = io.WriteString(w, fmt.Sprintf(`{
			"token": "ghs_t%d",
			"expires_at": "%s",
			"permissions": {}
		}`, callCount, time.Now().Add(-time.Second).UTC().Format(time.RFC3339)))
	}))
	defer gh.Close()

	c, err := githubapp.New(githubapp.Config{
		AppID: 1, InstallationID: 1, PrivateKeyBase64: keyB64, APIBaseURL: gh.URL,
	})
	require.NoError(t, err)
	ctx := context.Background()

	_, err = c.InstallationToken(ctx)
	require.NoError(t, err)
	_, err = c.InstallationToken(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, callCount, "stale cached token should be re-minted")
}

func TestClient_GitHubAPIErrorPropagates(t *testing.T) {
	_, keyB64 := generateTestKey(t)
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"message":"Bad credentials"}`)
	}))
	defer gh.Close()

	c, err := githubapp.New(githubapp.Config{
		AppID: 1, InstallationID: 1, PrivateKeyBase64: keyB64, APIBaseURL: gh.URL,
	})
	require.NoError(t, err)
	_, err = c.InstallationToken(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
}

func TestClient_InvalidBase64PrivateKey(t *testing.T) {
	_, err := githubapp.New(githubapp.Config{
		AppID: 1, InstallationID: 1, PrivateKeyBase64: "!!!not-base64!!!",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "decode private key")
}

func TestClient_InvalidPEMContent(t *testing.T) {
	// Valid base64, but the decoded bytes aren't a PEM key.
	bad := base64.StdEncoding.EncodeToString([]byte("not a pem"))
	_, err := githubapp.New(githubapp.Config{
		AppID: 1, InstallationID: 1, PrivateKeyBase64: bad,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse private key")
}
