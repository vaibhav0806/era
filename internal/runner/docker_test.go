package runner_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vaibhav0806/era/internal/runner"
)

func TestParseResult(t *testing.T) {
	out := bytes.NewBufferString("some log line\nanother log\n" +
		`RESULT {"branch":"agent/3/foo","summary":"dummy-commit-ok","tokens":0,"cost_cents":0}` + "\n")
	o, err := runner.ParseResult(out)
	require.NoError(t, err)
	require.Equal(t, "agent/3/foo", o.Branch)
	require.Equal(t, "dummy-commit-ok", o.Summary)
}

func TestParseResult_Missing(t *testing.T) {
	out := bytes.NewBufferString("nope\nnothing here\n")
	_, err := runner.ParseResult(out)
	require.ErrorIs(t, err, runner.ErrNoResult)
}

func TestParseResult_OnlyBranchNoSummary(t *testing.T) {
	// The entrypoint always emits both, but parser should tolerate a
	// RESULT line with only branch. summary stays empty.
	out := bytes.NewBufferString(`RESULT {"branch":"agent/7/x","summary":"","tokens":0,"cost_cents":0}` + "\n")
	o, err := runner.ParseResult(out)
	require.NoError(t, err)
	require.Equal(t, "agent/7/x", o.Branch)
	require.Equal(t, "", o.Summary)
}

func TestParseResult_MultipleRESULTLinesUsesFirst(t *testing.T) {
	// If two RESULT lines somehow appear, parser picks the first (most
	// conservative — later work may have failed).
	out := bytes.NewBufferString(
		`RESULT {"branch":"agent/1/a","summary":"one","tokens":0,"cost_cents":0}` + "\n" +
			`RESULT {"branch":"agent/1/b","summary":"two","tokens":0,"cost_cents":0}` + "\n")
	o, err := runner.ParseResult(out)
	require.NoError(t, err)
	require.Equal(t, "agent/1/a", o.Branch)
	require.Equal(t, "one", o.Summary)
}

func TestParseResult_ExtendedWithTokensAndCost(t *testing.T) {
	r := bytes.NewBufferString(`RESULT {"branch":"a/1/x","summary":"ok","tokens":12345,"cost_cents":17}` + "\n")
	o, err := runner.ParseResult(r)
	require.NoError(t, err)
	require.Equal(t, "a/1/x", o.Branch)
	require.Equal(t, "ok", o.Summary)
	require.Equal(t, int64(12345), o.Tokens)
	require.Equal(t, 17, o.CostCents)
}

func TestParseResult_SummaryWithSpacesAndNewlines(t *testing.T) {
	combined := strings.NewReader(`RESULT {"branch":"foo","summary":"hello world\nand a newline","tokens":100,"cost_cents":5}` + "\n")
	out, err := runner.ParseResult(combined)
	require.NoError(t, err)
	require.Equal(t, "hello world\nand a newline", out.Summary)
}
