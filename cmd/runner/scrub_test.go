package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScrubSecrets_TokenizedURL(t *testing.T) {
	in := "git push https://x-access-token:ghp_abcdef1234567890ABCDEF1234567890ABCDEF@github.com/x/y.git foo"
	out := scrubSecrets(in)
	require.Contains(t, out, "https://x-access-token:***@github.com/x/y.git")
	require.NotContains(t, out, "ghp_abcdef1234567890")
}

func TestScrubSecrets_BareClassicPAT(t *testing.T) {
	in := "Authorization: Bearer ghp_abcdef1234567890ABCDEF1234567890ABCDEF"
	out := scrubSecrets(in)
	require.Equal(t, "Authorization: Bearer ghp_***", out)
}

func TestScrubSecrets_FineGrainedPAT(t *testing.T) {
	in := "token=github_pat_11AAAAAAA0abcdefghijklmnop_qrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	out := scrubSecrets(in)
	require.Contains(t, out, "github_pat_***")
	require.NotContains(t, out, "_qrstuvwxyz")
}

func TestScrubSecrets_NoTokensPassThrough(t *testing.T) {
	in := "ordinary error message: file not found"
	require.Equal(t, in, scrubSecrets(in))
}

func TestScrubArgs(t *testing.T) {
	args := []string{"push", "https://x-access-token:ghp_secret123456789012345678901234567890@github.com/x/y.git", "branch"}
	out := scrubArgs(args)
	require.Equal(t, "push", out[0])
	require.Contains(t, out[1], "***@")
	require.NotContains(t, out[1], "ghp_secret")
	require.Equal(t, "branch", out[2])
}
