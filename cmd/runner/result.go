package main

import (
	"encoding/json"
	"fmt"
	"io"
)

// runResult is what the runner emits as its final RESULT line, parsed by the
// orchestrator's ParseResult.
type runResult struct {
	Branch    string `json:"branch"`
	Summary   string `json:"summary"`
	Tokens    int64  `json:"tokens"`
	CostCents int    `json:"cost_cents"`
}

type runProgress struct {
	Iter      int    `json:"iter"`
	Action    string `json:"action"`
	Tokens    int64  `json:"tokens_cum"`
	CostCents int    `json:"cost_cents_cum"`
}

func writeProgress(w io.Writer, p runProgress) {
	payload, err := json.Marshal(p)
	if err != nil {
		return // best-effort
	}
	fmt.Fprintf(w, "PROGRESS %s\n", payload)
}

// writeResult emits the RESULT line to w as JSON so prose summaries with
// spaces and newlines round-trip cleanly.
func writeResult(w io.Writer, r runResult) {
	payload, err := json.Marshal(r)
	if err != nil {
		// runResult is plain and always marshals; this branch is defensive.
		fmt.Fprintf(w, "RESULT {\"branch\":\"\",\"summary\":\"marshal_error\",\"tokens\":0,\"cost_cents\":0}\n")
		return
	}
	fmt.Fprintf(w, "RESULT %s\n", payload)
}
