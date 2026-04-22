package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// fakePi is a deterministic replacement for the real `pi --mode json` process.
// Tests supply canned stdout (JSONL events); the driver consumes them as if
// they came from a real Pi invocation.
type fakePi struct {
	stdout  io.Reader
	stderr  io.Reader
	waitErr error
	waited  bool
}

func (f *fakePi) Stdout() (io.Reader, error) { return f.stdout, nil }
func (f *fakePi) Stderr() (io.Reader, error) { return f.stderr, nil }
func (f *fakePi) Start() error               { return nil }
func (f *fakePi) Wait() error                { f.waited = true; return f.waitErr }
func (f *fakePi) Abort() error               { return nil }

func TestPi_DrainsEventsAndAggregates(t *testing.T) {
	f := &fakePi{
		stdout: strings.NewReader(`{"type":"tool_execution_end","tool":"bash"}
{"type":"message_end","message":{"usage":{"totalTokens":10,"cost":{"total":0.01}},"stopReason":"toolUse"}}
{"type":"message_end","message":{"usage":{"totalTokens":20,"cost":{"total":0.02}},"stopReason":"endTurn"}}
{"type":"agent_end"}
`),
		stderr: strings.NewReader(""),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	summary, err := runPi(ctx, f, nopObserver{})
	require.NoError(t, err)
	require.Equal(t, int64(30), summary.TotalTokens)
	require.InDelta(t, 0.03, summary.TotalCostUSD, 1e-9)
	require.Equal(t, 1, summary.ToolUseCount)
	require.True(t, f.waited)
}

type nopObserver struct{}

func (nopObserver) onEvent(e *piEvent) error { return nil }
