package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteResult_EmitsJSONLine(t *testing.T) {
	var buf bytes.Buffer
	writeResult(&buf, runResult{
		Branch:    "agent/1/foo",
		Summary:   "Pi's answer with spaces\nand a newline",
		Tokens:    42,
		CostCents: 7,
	})
	line := buf.String()
	require.True(t, strings.HasPrefix(line, "RESULT "), "line must start with RESULT marker: %q", line)
	require.True(t, strings.HasSuffix(line, "\n"), "line must end with newline")
	payload := strings.TrimSuffix(strings.TrimPrefix(line, "RESULT "), "\n")
	var got runResult
	require.NoError(t, json.Unmarshal([]byte(payload), &got))
	require.Equal(t, "agent/1/foo", got.Branch)
	require.Equal(t, "Pi's answer with spaces\nand a newline", got.Summary)
	require.Equal(t, int64(42), got.Tokens)
	require.Equal(t, 7, got.CostCents)
}

func TestWriteResult_EmptySummaryOK(t *testing.T) {
	var buf bytes.Buffer
	writeResult(&buf, runResult{Branch: "b", Summary: "", Tokens: 1, CostCents: 1})
	line := buf.String()
	require.True(t, strings.HasPrefix(line, "RESULT "), "line must start with RESULT marker: %q", line)
	payload := strings.TrimSuffix(strings.TrimPrefix(line, "RESULT "), "\n")
	var got runResult
	require.NoError(t, json.Unmarshal([]byte(payload), &got))
	require.Equal(t, "b", got.Branch)
	require.Equal(t, "", got.Summary)
}

func TestWriteProgress_EmitsJSONLine(t *testing.T) {
	var buf bytes.Buffer
	writeProgress(&buf, runProgress{
		Iter: 7, Action: "write", Tokens: 8200, CostCents: 3,
	})
	line := buf.String()
	require.True(t, strings.HasPrefix(line, "PROGRESS "))
	require.True(t, strings.HasSuffix(line, "\n"))
	payload := strings.TrimSuffix(strings.TrimPrefix(line, "PROGRESS "), "\n")
	var got runProgress
	require.NoError(t, json.Unmarshal([]byte(payload), &got))
	require.Equal(t, 7, got.Iter)
	require.Equal(t, "write", got.Action)
	require.Equal(t, int64(8200), got.Tokens)
	require.Equal(t, 3, got.CostCents)
}

func TestFinalSummary_UsesLastTextWhenPresent(t *testing.T) {
	s := &runSummary{LastText: "Pi said something useful"}
	require.Equal(t, "Pi said something useful", finalSummary(s, nil))
}

func TestFinalSummary_FallsBackWhenEmpty(t *testing.T) {
	s := &runSummary{}
	require.Equal(t, "(no final message from pi)", finalSummary(s, nil))
}

func TestFinalSummary_Aborted(t *testing.T) {
	s := &runSummary{LastText: "should be ignored"}
	got := finalSummary(s, errors.New("wall clock cap fired"))
	require.Contains(t, got, "aborted_")
}
