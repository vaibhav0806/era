package main

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

type piEvent struct {
	Type    string `json:"type"`
	Error   string `json:"error,omitempty"`
	Message struct {
		StopReason string `json:"stopReason,omitempty"`
		Usage      struct {
			TotalTokens int64 `json:"totalTokens"`
			Cost        struct {
				Total float64 `json:"total"`
			} `json:"cost"`
		} `json:"usage"`
	} `json:"message,omitempty"`
	Tool string `json:"tool,omitempty"`
}

func parseEvent(b []byte) (*piEvent, error) {
	var e piEvent
	if err := json.Unmarshal(b, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// streamEvents reads a JSONL stream, one event per line, ignoring empty lines.
func streamEvents(r io.Reader) ([]*piEvent, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var out []*piEvent
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		e, err := parseEvent([]byte(line))
		if err != nil {
			return out, err
		}
		out = append(out, e)
	}
	return out, sc.Err()
}
