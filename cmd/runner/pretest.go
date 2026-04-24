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
