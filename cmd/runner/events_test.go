package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseEvent_MessageEnd(t *testing.T) {
	line := `{"type":"message_end","message":{"usage":{"totalTokens":12345,"cost":{"total":0.17}},"stopReason":"endTurn"}}`
	e, err := parseEvent([]byte(line))
	require.NoError(t, err)
	require.Equal(t, "message_end", e.Type)
	require.Equal(t, int64(12345), e.Message.Usage.TotalTokens)
	require.InDelta(t, 0.17, e.Message.Usage.Cost.Total, 1e-9)
	require.Equal(t, "endTurn", e.Message.StopReason)
}

func TestParseEvent_ToolUseEnd(t *testing.T) {
	line := `{"type":"tool_use_end","tool":"bash"}`
	e, err := parseEvent([]byte(line))
	require.NoError(t, err)
	require.Equal(t, "tool_use_end", e.Type)
}

func TestParseEvent_AgentEnd(t *testing.T) {
	e, err := parseEvent([]byte(`{"type":"agent_end"}`))
	require.NoError(t, err)
	require.Equal(t, "agent_end", e.Type)
}

func TestParseEvent_Error(t *testing.T) {
	e, err := parseEvent([]byte(`{"type":"error","error":"rate limited"}`))
	require.NoError(t, err)
	require.Equal(t, "error", e.Type)
	require.Equal(t, "rate limited", e.Error)
}

func TestParseEvent_Malformed(t *testing.T) {
	_, err := parseEvent([]byte(`not json`))
	require.Error(t, err)
}

func TestStreamEvents(t *testing.T) {
	r := strings.NewReader(`{"type":"agent_end"}
{"type":"error","error":"x"}

`)
	evts, err := streamEvents(r)
	require.NoError(t, err)
	require.Len(t, evts, 2)
	require.Equal(t, "agent_end", evts[0].Type)
	require.Equal(t, "error", evts[1].Type)
}

func TestParseEvent_MessageEndWithTextContent(t *testing.T) {
	data, err := os.ReadFile("testdata/message_end_with_text.jsonl")
	require.NoError(t, err)
	e, err := parseEvent(data)
	require.NoError(t, err)
	require.Equal(t, "message_end", e.Type)
	require.Equal(t, "assistant", e.Message.Role)
	require.Len(t, e.Message.Content, 1)
	require.Equal(t, "text", e.Message.Content[0].Type)
	require.Equal(t, "Here is what I found in the README.", e.Message.Content[0].Text)
}
