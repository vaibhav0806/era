package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"
)

// MaybeRunPreCommitTest runs `make test` if the workspace has a `test` target.
// Returns (skipped, error). skipped=true means no Makefile test target existed.
// On test failure, error wraps the output in a "tests_failed: <out>" message
// suitable for use as the runner's summary field.
func MaybeRunPreCommitTest(ctx context.Context, workspace string) (bool, error) {
	if !HasMakefileTest(workspace) {
		return true, nil
	}
	out, err := RunMakefileTest(ctx, workspace)
	if err != nil {
		return false, fmt.Errorf("tests_failed: %s", truncate(out, 2000))
	}
	return false, nil
}

// truncate caps s at n bytes (byte-wise, may cut a rune — acceptable for log
// summaries where the user can open the full branch anyway).
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// makefileTestTarget matches a `test:` target at the start of a line.
// Excludes `test-*:`, `test_*:`, commented lines, and indented recipe lines.
var makefileTestTarget = regexp.MustCompile(`(?m)^test\s*:`)

// HasMakefileTest returns true iff workspace/Makefile exists and contains a
// `test` target declaration at column 0.
func HasMakefileTest(workspace string) bool {
	f, err := os.Open(filepath.Join(workspace, "Makefile"))
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if makefileTestTarget.MatchString(sc.Text()) {
			return true
		}
	}
	return false
}

// RunMakefileTest runs `make test` in workspace with a 10-minute hard cap.
// Returns combined stdout+stderr plus any error. Non-zero exit from make is
// surfaced as error; output is always returned regardless.
func RunMakefileTest(ctx context.Context, workspace string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(cctx, "make", "test")
	cmd.Dir = workspace
	out, err := cmd.CombinedOutput()
	if cctx.Err() == context.DeadlineExceeded {
		return string(out), fmt.Errorf("pre-commit test exceeded 10-minute cap")
	}
	return string(out), err
}
