// Package audit parses AUDIT JSONL lines emitted by the sidecar into typed
// Entry values that the orchestrator persists to the events table.
package audit

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

// Entry mirrors the sidecar's auditEntry struct; both must stay in sync.
type Entry struct {
	Time    string `json:"time"`
	Method  string `json:"method"`
	Path    string `json:"path"`
	Host    string `json:"host"`
	Status  int    `json:"status"`
	Bytes   int    `json:"bytes"`
	Latency int    `json:"latency_ms"`
}

// Parse extracts an Entry from a single line. Returns ok=false if the line
// is not an AUDIT line or the JSON payload is malformed.
func Parse(line string) (Entry, bool) {
	const prefix = "AUDIT "
	if !strings.HasPrefix(line, prefix) {
		return Entry{}, false
	}
	var e Entry
	if err := json.Unmarshal([]byte(line[len(prefix):]), &e); err != nil {
		return Entry{}, false
	}
	return e, true
}

// Stream reads r line-by-line, calling onEntry for each AUDIT line. Other
// lines are ignored. Returns when r reaches EOF or the scanner errors.
func Stream(r io.Reader, onEntry func(Entry)) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		if e, ok := Parse(sc.Text()); ok {
			onEntry(e)
		}
	}
	return sc.Err()
}
