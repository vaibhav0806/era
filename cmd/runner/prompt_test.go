package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComposePrompt_IncludesSearchFetchInstructions(t *testing.T) {
	out := composePrompt("add a hello world file")
	// Core content preserved
	require.Contains(t, out, "add a hello world file")
	// Search/fetch instructions present
	require.Contains(t, out, "http://127.0.0.1:8080/search")
	require.Contains(t, out, "http://127.0.0.1:8080/fetch")
	// Warning that direct HTTP is blocked
	require.Contains(t, out, "blocked")
}

func TestComposePrompt_DelimitsUserTaskClearly(t *testing.T) {
	out := composePrompt("do the thing")
	// User task appears AFTER the instruction block, with a clear delimiter
	// so Pi knows where the instructions end and the task begins.
	instructionIdx := strings.Index(out, "blocked")
	taskIdx := strings.Index(out, "do the thing")
	require.Greater(t, taskIdx, instructionIdx, "user task must come AFTER instructions")
}

func TestComposePrompt_EmptyUserTaskUnchanged(t *testing.T) {
	// Edge case: empty user task still gets the preamble; runner config
	// validation rejects empty task elsewhere, but composePrompt shouldn't
	// crash or trim surprisingly.
	out := composePrompt("")
	require.Contains(t, out, "http://127.0.0.1:8080/search")
}
