package main

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestTruncateForTelegram_AsciiCases(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		budget int
		want   string
	}{
		{"under budget", "hello", 100, "hello"},
		{"exactly at budget", strings.Repeat("a", 50), 50, strings.Repeat("a", 50)},
		{"over budget", strings.Repeat("a", 60), 50, strings.Repeat("a", 50) + "\n…(10 bytes truncated)"},
		{"empty", "", 100, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateForTelegram(tc.in, tc.budget)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestTruncateForTelegram_UnicodeSafe(t *testing.T) {
	// 3-byte rune × 20 = 60 bytes. Budget 50 should cut at a rune boundary.
	s := strings.Repeat("€", 20)
	got := truncateForTelegram(s, 50)
	// Strip footer to get just the truncated prefix.
	prefix := got
	if idx := strings.Index(got, "\n…("); idx >= 0 {
		prefix = got[:idx]
	}
	require.True(t, utf8.ValidString(prefix), "truncated prefix must be valid UTF-8: %q", prefix)
	require.True(t, strings.HasPrefix(s, prefix), "prefix must be a prefix of input")
	// Budget 50 with 3-byte runes → 16 complete runes = 48 bytes kept.
	require.LessOrEqual(t, len(prefix), 50)
}
