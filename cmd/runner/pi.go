package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// piProcess abstracts the `pi --mode rpc` child process so we can fake it in
// unit tests. The real implementation is realPi.
type piProcess interface {
	Stdin() (io.Writer, error)
	Stdout() (io.Reader, error)
	Stderr() (io.Reader, error)
	Start() error
	Wait() error
	Abort() error
}

type runSummary struct {
	TotalTokens  int64
	TotalCostUSD float64
	ToolUseCount int
}

// eventObserver watches the live event stream (used by caps enforcer).
type eventObserver interface {
	onEvent(e *piEvent) error
}

func runPi(ctx context.Context, p piProcess, prompt string, obs eventObserver) (*runSummary, error) {
	stdin, err := p.Stdin()
	if err != nil {
		return nil, fmt.Errorf("stdin: %w", err)
	}
	stdout, err := p.Stdout()
	if err != nil {
		return nil, fmt.Errorf("stdout: %w", err)
	}
	if err := p.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}

	// Watchdog: if the parent ctx is canceled (wall-clock cap or external
	// shutdown), kill Pi so the scanner unblocks and we can return promptly.
	// This is the only reliable wall-clock enforcement when Pi is blocked
	// inside an LLM call and producing no events.
	ctxDone := make(chan struct{})
	defer close(ctxDone)
	go func() {
		select {
		case <-ctx.Done():
			_ = p.Abort()
		case <-ctxDone:
		}
	}()

	// Send the prompt command as a single JSONL line.
	cmd := map[string]string{"type": "prompt", "prompt": prompt, "id": "era-1"}
	b, _ := json.Marshal(cmd)
	if _, err := fmt.Fprintf(stdin, "%s\n", b); err != nil {
		return nil, fmt.Errorf("write prompt: %w", err)
	}

	// Stream events live — one per line — so the observer can abort mid-stream.
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	summary := &runSummary{}

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		e, err := parseEvent([]byte(line))
		if err != nil {
			// Don't abort on a single malformed line — Pi might emit
			// something unexpected; keep reading, surface the error only if
			// nothing useful arrives after.
			continue
		}
		// Accumulate BEFORE calling observer so caps.Totals() reflects
		// running totals when onEvent is called.
		switch e.Type {
		case "message_end":
			summary.TotalTokens += e.Message.Usage.TotalTokens
			summary.TotalCostUSD += e.Message.Usage.Cost.Total
		case "tool_use_end":
			summary.ToolUseCount++
		case "error":
			_ = p.Abort()
			return summary, fmt.Errorf("pi error: %s", e.Error)
		}
		if obsErr := obs.onEvent(e); obsErr != nil {
			_ = p.Abort()
			return summary, obsErr
		}
		if e.Type == "agent_end" {
			break
		}
	}
	if err := sc.Err(); err != nil {
		return summary, fmt.Errorf("stream: %w", err)
	}
	if err := p.Wait(); err != nil {
		// If ctx was canceled (wall-clock), surface that as the cap error
		// rather than a generic "exit status 1" so the orchestrator's
		// failure reason is informative. errCapExceeded is defined in
		// caps.go (Task M1-7); for now it doesn't exist, so this branch
		// can use a placeholder error and Task M1-7 will rewire it.
		if ctx.Err() != nil {
			return summary, fmt.Errorf("wall-clock cap fired during pi run: %w", ctx.Err())
		}
		return summary, fmt.Errorf("pi wait: %w", err)
	}
	return summary, nil
}

// realPi is a thin exec.Cmd wrapper that implements piProcess.
type realPi struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}

func newRealPi(ctx context.Context, model, apiKey, workdir string) (*realPi, error) {
	cmd := exec.CommandContext(ctx, "pi",
		"--mode", "rpc",
		"--provider", "openrouter",
		"--model", model,
		"--tools", "read,write,edit,grep,find,ls,bash",
	)
	cmd.Dir = workdir // Pi uses process CWD as session CWD; pass explicitly.
	cmd.Env = append(cmd.Environ(),
		"OPENROUTER_API_KEY="+apiKey,
		"PI_CODING_AGENT_DIR=/tmp/pi-state",
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	return &realPi{cmd: cmd, stdin: stdin, stdout: stdout, stderr: stderr}, nil
}

func (r *realPi) Stdin() (io.Writer, error)  { return r.stdin, nil }
func (r *realPi) Stdout() (io.Reader, error) { return r.stdout, nil }
func (r *realPi) Stderr() (io.Reader, error) { return r.stderr, nil }
func (r *realPi) Start() error               { return r.cmd.Start() }
func (r *realPi) Wait() error                { return r.cmd.Wait() }
func (r *realPi) Abort() error {
	if r.cmd.Process == nil {
		return nil
	}
	return r.cmd.Process.Kill()
}
