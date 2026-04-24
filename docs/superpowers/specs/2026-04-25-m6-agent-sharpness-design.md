# M6 — Agent Sharpness Design

**Date:** 2026-04-25
**Status:** design, pre-implementation
**Predecessor:** M5 (polish + safety)

## 1. Goal

Make era the agent sharper at its core job — finishing real tasks, reacting to your follow-ups, and staying observable while running. Six items across three themes:

- **Better at finishing:** bumped iteration/cost/wall caps + per-task `--budget` profiles; smarter egress allowlist for common dev hosts.
- **Conversational:** reply to a completion DM to thread a follow-up task with prior context.
- **Observable:** mid-run progress DMs (pinned, edited per tool call); `/stats` command.
- **Quick reads:** `/ask <repo> <question>` — skips Docker commit path, returns prose in ~15-30s.

## 2. Non-goals (deferred to M7+)

- Dedicated read-only sandbox image (option C from Q1). Only if `/ask` latency becomes painful.
- HTTP-based progress transport (option B from Q2). Only if stdout pipe hits limits.
- Richer `telegram_messages` audit table (option B from Q3). Only if we thread progress/reject DMs too.
- Transitive reply chains (cumulative conversation history).
- `/stats` detailed breakdown with expensive-tasks list (option C from Q6).
- PR auto-merge, NL repo parsing, long-answer attachments, scheduled/recurring tasks, task chaining, multi-repo fan-out, dev/prod bot split.
- sqlc drift check in CI.
- Reconcile cleaning up orphan progress messages.
- Rate-limiting progress edits.

## 3. Architecture overview

M6 makes two surfaces bidirectional for the first time:

- **Runner ↔ orchestrator:** today the runner emits exactly one `RESULT <json>` line at exit. M6 adds a streaming channel — zero-or-more `PROGRESS <json>` lines during execution, consumed by the existing Docker log scanner and fanned out via a new `ProgressNotifier` interface.
- **Telegram ↔ task lifecycle:** today each incoming message is a standalone command. M6 recognizes `reply_to_message_id` and threads the original task's context into a new queued task.

### 3.1 Touched packages

```
cmd/runner/
├── pi.go                    MODIFY (AJ) — emit progress callback on tool_execution_end
├── main.go                  MODIFY (AJ, AK) — wire progress callback; detect ERA_READ_ONLY and skip commit
└── result.go                MODIFY (AJ) — writeProgress(runProgress)

internal/runner/
└── docker.go                MODIFY (AG, AJ, AK) — per-task env overrides; streamToWithProgress; ERA_READ_ONLY plumbing

internal/queue/
├── queue.go                 MODIFY (AG, AI, AJ, AK) — profile-aware CreateTask, ProgressNotifier, CreateAskTask
├── budget.go                CREATE (AG) — Profile struct + ParseBudgetFlag
├── reply_compose.go         CREATE (AI) — ComposeReplyPrompt
└── stats.go                 CREATE (AL) — Stats struct + Queue.Stats()

internal/db/
├── repo.go                  MODIFY (AG, AI, AK, AL) — SetBudgetProfile, SetCompletionMessageID, stats wrappers
└── (sqlc-regenerated)       tasks.sql.go + models.go

internal/telegram/
├── client.go                MODIFY (AI, AJ) — SendMessage returns msg_id; new EditMessageText
├── handler.go               MODIFY (AG, AI, AK, AL) — --budget parse; reply detection; /ask, /stats routes
└── handler_test.go          MODIFY — new command coverage

cmd/orchestrator/main.go     MODIFY (AI, AJ) — tgNotifier holds repo + progressMsgs; NotifyProgress method

cmd/sidecar/
├── allowlist.go             MODIFY (AH) — new static hosts + PI_EGRESS_EXTRA parser
└── allowlist_test.go        MODIFY (AH) — new host tests + env-var test

migrations/
├── 0007_budget_profile.sql          CREATE (AG)
├── 0008_completion_message_id.sql   CREATE (AI)
└── 0009_read_only.sql               CREATE (AK)

queries/tasks.sql            MODIFY — 6+ new queries (SetBudgetProfile, SetCompletionMessageID, GetTaskByCompletionMessageID, SetReadOnly, stats ×4)

scripts/smoke/
├── phase_ag_caps.sh
├── phase_ah_allowlist.sh
├── phase_ai_reply.sh
├── phase_aj_progress.sh
├── phase_ak_ask.sh
└── phase_al_stats.sh
```

### 3.2 Protocol additions

**Runner stdout today:**

```
[normal logs...]
RESULT {"branch":"...","summary":"...","tokens":123,"cost_cents":4}
```

**Runner stdout M6:**

```
[normal logs...]
PROGRESS {"iter":1,"action":"read","tokens_cum":200,"cost_cents_cum":0}
PROGRESS {"iter":2,"action":"write","tokens_cum":2400,"cost_cents_cum":1}
[...]
RESULT {"branch":"...","summary":"...","tokens":123,"cost_cents":4}
```

Scanner rules: unknown line prefixes ignored (same as today). `RESULT` terminal, exactly one. `PROGRESS` streaming, 0..N. Backward-compatible — an old runner without PROGRESS emission produces valid stdout.

### 3.3 Telegram client API change

```go
// Before (M5):
SendMessage(ctx context.Context, chatID int64, body string) error

// After (M6):
SendMessage(ctx context.Context, chatID int64, body string) (int64, error)
EditMessageText(ctx context.Context, chatID int64, messageID int64, body string) error  // new
```

Every M5 caller of `SendMessage` discards the new return value with `_, err :=`. `tgNotifier.NotifyCompleted` stores the message ID via `repo.SetCompletionMessageID`. `tgNotifier.NotifyProgress` uses EditMessageText after the first SendMessage.

## 4. Per-item mechanics

### 4.1 Chunk 1 — Caps + budget profiles (phase AG)

Migration 0007:

```sql
-- +goose Up
ALTER TABLE tasks ADD COLUMN budget_profile TEXT NOT NULL DEFAULT 'default';
-- +goose Down
SELECT 1;
```

New file `internal/queue/budget.go`:

```go
package queue

import "strings"

type Profile struct {
    Name       string
    MaxIter    int
    MaxCents   int
    MaxWallSec int
}

// Profiles defines the three named budget presets. Caps are independent —
// a profile exceeding any one cap is considered exceeded.
var Profiles = map[string]Profile{
    "quick":   {"quick",   20,   5,  600},  // 10 min, 20 iters, $0.05
    "default": {"default", 60,  20, 1800},  // 30 min, 60 iters, $0.20
    "deep":    {"deep",   120, 100, 3600},  // 60 min, 120 iters, $1.00
}

// ParseBudgetFlag strips a leading `--budget=NAME` token from the first
// whitespace position. Returns (profileName, cleanedDesc). Unknown profiles
// fall back to "default" silently (validation happens when resolving to caps).
func ParseBudgetFlag(desc string) (string, string) {
    desc = strings.TrimSpace(desc)
    if !strings.HasPrefix(desc, "--budget=") {
        return "default", desc
    }
    end := strings.IndexByte(desc, ' ')
    if end < 0 {
        return "default", desc // malformed; no space after flag, ignore flag
    }
    name := strings.TrimPrefix(desc[:end], "--budget=")
    if _, ok := Profiles[name]; !ok {
        return "default", desc // unknown name, preserve whole desc
    }
    return name, strings.TrimSpace(desc[end+1:])
}
```

Flag parsing integration — `internal/telegram/handler.go` currently calls `parseTaskArgs(body)` which extracts `(repo, desc)` from `/task [owner/repo] <desc>`. Add a step: AFTER repo extraction, run `ParseBudgetFlag(desc)`:

```go
repo, desc := parseTaskArgs(body)
profile, desc := queue.ParseBudgetFlag(desc)
id, err := h.queue.CreateTask(ctx, desc, repo, profile)
```

`Queue.CreateTask` signature gains `profile string`:

```go
func (q *Queue) CreateTask(ctx context.Context, desc, targetRepo, profile string) (int64, error)
```

Repo layer stores `budget_profile`. Default callers (existing `/retry`, non-budget-flagged `/task`) pass `"default"`.

Runner caps today come from environment vars read in the runner binary. M6 lets orchestrator override per-task via Docker run env:

- Add fields to `RunInput`: `MaxIter`, `MaxCents`, `MaxWallSec int`.
- In `Docker.buildDockerArgs`, if a field is non-zero, emit `-e ERA_MAX_ITERATIONS=N` etc.
- In `QueueAdapter.Run`, after looking up the profile for the task, populate those fields.

**`RetryTask` profile inheritance.** `RetryTask` in `internal/queue/queue.go` (~line 489) clones the description of a prior task. In M6, the retry should inherit the original task's `budget_profile` so a `deep` task stays `deep` on retry. Update the underlying sqlc query (or add `CloneTask`) to copy `description + target_repo + budget_profile` atomically. Spec calls this out so the implementer doesn't default every retry to `"default"`.

Existing `.env` and `deploy/env.template` defaults bump to match the new `default` profile:

```
PI_MAX_ITERATIONS=60
PI_MAX_COST_CENTS=20
PI_MAX_WALL_SECONDS=1800
```

Tests:
- `internal/queue/budget_test.go` — ParseBudgetFlag table: bare `/task ...`, `--budget=deep`, `--budget=unknown`, malformed no-space, `--budget=deep trailing`.
- `internal/queue/queue_test.go` — CreateTask persists profile; fetched task reports it.
- `internal/runner/docker_test.go` — buildDockerArgs emits per-task overrides when RunInput fields set.
- Handler test — `/task --budget=deep owner/repo desc` queues with profile=deep.

### 4.2 Chunk 2 — Egress allowlist expansion (phase AH)

Additions to `cmd/sidecar/allowlist.go:staticHosts`:

```go
// M6 AH: common dev ecosystem hosts
"crates.io", "static.crates.io", "index.crates.io",
"registry.yarnpkg.com",
"cdn.jsdelivr.net", "cdnjs.cloudflare.com", "unpkg.com",
"fonts.googleapis.com", "fonts.gstatic.com",
"services.gradle.org",
```

New `PI_EGRESS_EXTRA` env var, parsed at sidecar boot:

```go
if extra := os.Getenv("PI_EGRESS_EXTRA"); extra != "" {
    for _, h := range strings.Split(extra, ",") {
        h = strings.TrimSpace(h)
        if h != "" {
            staticHosts = append(staticHosts, h)
        }
    }
}
```

Tests:
- Each new static host asserted with `require.True(t, a.allowed("<host>"))`.
- `TestPIEgressExtra_AppendsHosts` sets env before constructing allowlist, asserts custom host allowed.
- `TestPIEgressExtra_EmptyWhitespaceSkipped` with value `"foo.com, , bar.org, "` — only foo.com and bar.org added.

**Explicit skip list** (documented in spec, not code): Alpine CDN, gitlab/bitbucket/sr.ht, IP literals, pastebin services. These stay blocked; future milestones evaluate case-by-case.

### 4.3 Chunk 3 — Reply-to-continue (phase AI)

Migration 0008:

```sql
-- +goose Up
ALTER TABLE tasks ADD COLUMN completion_message_id INTEGER;
-- +goose Down
SELECT 1;
```

New sqlc queries:

```sql
-- name: SetCompletionMessageID :exec
UPDATE tasks SET completion_message_id = ? WHERE id = ?;

-- name: GetTaskByCompletionMessageID :one
SELECT * FROM tasks WHERE completion_message_id = ? LIMIT 1;
```

Repo wrappers:

```go
func (r *Repo) SetCompletionMessageID(ctx context.Context, id, msgID int64) error
func (r *Repo) GetTaskByCompletionMessageID(ctx context.Context, msgID int64) (Task, error)
```

**Telegram client change:**

```go
// Before:
func (c *tgClient) SendMessage(ctx context.Context, chatID int64, body string) error

// After:
func (c *tgClient) SendMessage(ctx context.Context, chatID int64, body string) (int64, error)
```

All existing callers (in cmd/orchestrator/main.go's tgNotifier methods, in digest scheduler, in handler error replies) mechanically become `_, err := n.client.SendMessage(...)`. One caller cares:

```go
// tgNotifier.NotifyCompleted after the SendMessage call:
msgID, err := n.client.SendMessage(ctx, n.chatID, msg)
if err != nil {
    slog.Error("notify completed", "err", err, "task", id)
    return
}
if err := n.repo.SetCompletionMessageID(ctx, id, msgID); err != nil {
    slog.Warn("set completion message id", "err", err, "task", id)
}
```

**`tgNotifier` struct renames + adds fields.** Today `tgNotifier` has a single `repo string // "owner/repo"` field (the sandbox repo, used by NotifyCompleted's URL fallback). AI does TWO changes:

1. **Rename** existing `repo string` → `sandboxRepo string`. Every call site inside `tgNotifier` methods that reads `n.repo` becomes `n.sandboxRepo`. (Grep: `cmd/orchestrator/main.go` line ~181 in NotifyCompleted.)
2. **Add** new `repo *db.Repo` field for calling `SetCompletionMessageID`. Wired via struct literal in `main.go` where tgNotifier is constructed.

```go
type tgNotifier struct {
    client       telegram.Client
    chatID       int64
    sandboxRepo  string        // renamed from `repo` in AI
    repo         *db.Repo      // new in AI
    progressMsgs sync.Map      // added in AJ — taskID (int64) → message_id (int64)
}
```

After the rename, the two "repo" identifiers don't collide: `sandboxRepo` is the remote "owner/repo" string; `repo` is the DB handle.

**Reply composition** — `internal/queue/reply_compose.go`:

```go
package queue

import (
    "fmt"
    "strings"

    "github.com/vaibhav0806/era/internal/db"
)

// ComposeReplyPrompt builds the prompt for a reply-threaded task.
// Non-transitive: original is the task the user replied to, not a chain.
func ComposeReplyPrompt(orig db.Task, replyBody string) string {
    var b strings.Builder
    fmt.Fprintf(&b, "You previously completed task #%d: %q\n", orig.ID, orig.Description)
    if orig.BranchName.Valid && orig.BranchName.String != "" {
        fmt.Fprintf(&b, "You made changes on branch %s.\n", orig.BranchName.String)
    }
    if orig.PrNumber.Valid {
        fmt.Fprintf(&b, "The pull request is #%d.\n", orig.PrNumber.Int64)
    }
    if orig.Summary.Valid && strings.TrimSpace(orig.Summary.String) != "" {
        fmt.Fprintf(&b, "\nSummary of what you did:\n%s\n", orig.Summary.String)
    }
    if orig.Status == "failed" && orig.Error.Valid {
        fmt.Fprintf(&b, "\nThat task failed with: %s\n", orig.Error.String)
    }
    fmt.Fprintf(&b, "\nNow the user has a follow-up: %s", replyBody)
    return b.String()
}
```

**Handler struct changes.** Today `type Handler struct { client Client; ops Ops }`. AI needs two new fields so `handleReply` can (a) look up the original task by `completion_message_id` and (b) fall back to the sandbox repo when `orig.TargetRepo == ""`:

```go
type Handler struct {
    client      Client
    ops         Ops         // queue, accepted as a narrow interface
    repo        *db.Repo    // new in AI — for GetTaskByCompletionMessageID
    sandboxRepo string      // new in AI — for reply-DM fallback
}
```

`NewHandler` gains two params; `cmd/orchestrator/main.go` threads `repo` and `cfg.GitHubSandboxRepo` into the construction.

**Handler routing** — in `telegram/handler.go`:

```go
func (h *Handler) Handle(ctx context.Context, u Update) error {
    // Existing prefix routing goes through strings.HasPrefix(u.Message.Text, "/...").
    // Insert BEFORE the first command match:
    if u.Message.ReplyToMessageID != 0 && !strings.HasPrefix(u.Message.Text, "/") {
        return h.handleReply(ctx, u)
    }
    // ... existing routes
}

func (h *Handler) handleReply(ctx context.Context, u Update) error {
    orig, err := h.repo.GetTaskByCompletionMessageID(ctx, int64(u.Message.ReplyToMessageID))
    if errors.Is(err, sql.ErrNoRows) {
        _, err := h.client.SendMessage(ctx, u.Message.Chat.ID,
            "sorry, couldn't find the task you're replying to")
        return err
    }
    if err != nil {
        return fmt.Errorf("get task by message id: %w", err)
    }
    prompt := queue.ComposeReplyPrompt(orig, u.Message.Text)
    id, err := h.queue.CreateTask(ctx, prompt, orig.TargetRepo, "default")
    if err != nil {
        return fmt.Errorf("queue reply task: %w", err)
    }
    repoLabel := orig.TargetRepo
    if repoLabel == "" {
        repoLabel = h.sandboxRepo
    }
    _, err = h.client.SendMessage(ctx, u.Message.Chat.ID,
        fmt.Sprintf("task #%d queued (reply to #%d, repo: %s)", id, orig.ID, repoLabel))
    return err
}
```

Tests:
- `TestHandler_ReplyWithUnknownMessageID_DMsNotFound`
- `TestHandler_ReplyWithKnownID_QueuesThreaded` — fake repo contains a task with completion_message_id=12345; reply update with reply_to=12345 + "add tests" → new task queued with prompt containing original description + reply body.
- `TestComposeReplyPrompt_HappyPath`, `TestComposeReplyPrompt_NoBranchNoSummary`, `TestComposeReplyPrompt_FailedTask`
- Integration (fake telegram client + real queue + in-memory DB): full reply round-trip.

### 4.4 Chunk 4 — Mid-run progress DMs (phase AJ)

Runner emits a `PROGRESS` line on each `tool_execution_end` event.

**`cmd/runner/result.go`** adds:

```go
type runProgress struct {
    Iter      int    `json:"iter"`
    Action    string `json:"action"`
    Tokens    int64  `json:"tokens_cum"`
    CostCents int    `json:"cost_cents_cum"`
}

func writeProgress(w io.Writer, p runProgress) {
    payload, err := json.Marshal(p)
    if err != nil {
        return // best-effort; drop rather than emit malformed
    }
    fmt.Fprintf(w, "PROGRESS %s\n", payload)
}
```

**`cmd/runner/pi.go`** gains a callback param:

```go
type progressFunc func(iter int, action string, tokens int64, costUSD float64)

func runPi(ctx context.Context, p piProcess, obs eventObserver, onProgress progressFunc) (*runSummary, error) {
    // ... existing setup ...
    for sc.Scan() {
        // ... existing event decode ...
        switch e.Type {
        case "message_end":
            // existing
        case "tool_execution_end":
            summary.ToolUseCount++
            if onProgress != nil {
                onProgress(summary.ToolUseCount, e.Tool, summary.TotalTokens, summary.TotalCostUSD)
            }
        case "error":
            // existing
        }
        // ...
    }
}
```

`cmd/runner/main.go` wires it:

```go
onProgress := func(iter int, action string, tokens int64, cost float64) {
    writeProgress(os.Stdout, runProgress{
        Iter: iter, Action: action,
        Tokens: tokens, CostCents: int(math.Round(cost * 100)),
    })
}
summary, piErr := runPi(ctx, p, c, onProgress)
```

Existing runPi tests pass `nil` for `onProgress` (backward compatible).

**Orchestrator side — `internal/runner/docker.go`:**

Today's `streamTo(mu, reader, combined, wg)` appends lines to `combined.String` during execution. We extend it to parse PROGRESS lines and fire a callback:

```go
type ProgressEvent struct {
    Iter      int    `json:"iter"`
    Action    string `json:"action"`
    Tokens    int64  `json:"tokens_cum"`
    CostCents int    `json:"cost_cents_cum"`
}

type ProgressCallback func(ev ProgressEvent)

func streamToWithProgress(mu *sync.Mutex, r io.Reader, combined *strings.Builder, wg *sync.WaitGroup, onProgress ProgressCallback) {
    defer wg.Done()
    sc := bufio.NewScanner(r)
    sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
    for sc.Scan() {
        line := sc.Text()
        mu.Lock()
        combined.WriteString(line)
        combined.WriteString("\n")
        mu.Unlock()
        if onProgress != nil && strings.HasPrefix(line, "PROGRESS ") {
            payload := strings.TrimPrefix(line, "PROGRESS ")
            var ev ProgressEvent
            if err := json.Unmarshal([]byte(payload), &ev); err == nil {
                onProgress(ev)
            }
        }
    }
}
```

`Docker.Run` signature gains `onProgress ProgressCallback` (may be nil):

```go
func (d *Docker) Run(ctx context.Context, in RunInput, onProgress ProgressCallback) (*RunOutput, error)
```

`QueueAdapter.Run` passes a callback through from the queue.

**Queue side — `internal/queue/queue.go`:**

Note on layering: `internal/runner/docker.go` defines its own `ProgressEvent` type, and `internal/queue/queue.go` defines its own mirror `ProgressEvent`. This duplication is **intentional** — the runner package cannot import queue (and shouldn't; runner is a lower-level primitive). The queue-side ProgressEvent is translated in `RunNext`'s progress callback from the runner-side one. Do NOT try to deduplicate during implementation — that introduces a circular import.

```go
type ProgressNotifier interface {
    NotifyProgress(ctx context.Context, taskID int64, ev ProgressEvent)
}

func (q *Queue) SetProgressNotifier(p ProgressNotifier) { q.progressNotifier = p }
```

`RunNext` builds a progress callback passed down to the adapter:

```go
progressCB := func(ev runner.ProgressEvent) {
    if q.progressNotifier != nil {
        q.progressNotifier.NotifyProgress(ctx, t.ID, ProgressEvent{
            Iter: ev.Iter, Action: ev.Action,
            Tokens: ev.Tokens, CostCents: ev.CostCents,
        })
    }
}
// Adapter.Run signature extended to accept progressCB
```

**Orchestrator tgNotifier — `cmd/orchestrator/main.go`:**

```go
type tgNotifier struct {
    client       telegram.Client
    chatID       int64
    repo         *db.Repo         // added in AI
    sandboxRepo  string           // renamed from `repo` in AI
    progressMsgs sync.Map         // taskID (int64) → message_id (int64)
}

func (n *tgNotifier) NotifyProgress(ctx context.Context, id int64, ev queue.ProgressEvent) {
    body := fmt.Sprintf("task #%d · iter %d · %s · $%.3f",
        id, ev.Iter, ev.Action, float64(ev.CostCents)/100.0)
    if existing, ok := n.progressMsgs.Load(id); ok {
        if err := n.client.EditMessageText(ctx, n.chatID, existing.(int64), body); err != nil {
            slog.Warn("edit progress", "err", err, "task", id)
        }
        return
    }
    msgID, err := n.client.SendMessage(ctx, n.chatID, body)
    if err != nil {
        slog.Warn("send progress", "err", err, "task", id)
        return
    }
    n.progressMsgs.Store(id, msgID)
}
```

Progress map uses `sync.Map` for concurrent safety (runner fires callbacks from a goroutine). On task terminal (completed/failed/cancelled/needs_review), the progress message is NOT deleted — it stays on Telegram as a historical trace; the terminal DM is a new message.

**Telegram client — new EditMessageText.**

Note: `internal/telegram/client.go` currently declares `EditMessageText(ctx, chatID int64, messageID int, text string) error` (with `messageID int`, matching go-telegram-bot-api/v5 which types message IDs as `int`). M6 AJ can either (a) keep the existing `int` signature and convert `int64` message IDs down at call sites, or (b) change the interface to `int64` and use `int(messageID)` inside the wrapper. **Pick (a)** — avoids a second interface break on top of SendMessage. Telegram message IDs fit comfortably in `int` (int32 min + positive values only in practice).

The new call from tgNotifier.NotifyProgress becomes:

```go
if existing, ok := n.progressMsgs.Load(id); ok {
    msgID := existing.(int64)
    if err := n.client.EditMessageText(ctx, n.chatID, int(msgID), body); err != nil { ... }
    return
}
```

The `progressMsgs` map still stores `int64` (matching the return type of `SendMessage`), but casts to `int` at the EditMessageText boundary. Fake client accepts `int` directly.

Fake client implements as a no-op store (records edits for assertion).

Tests:
- `TestStreamToWithProgress_FiresCallback` — canned stream with mixed lines; progress callback invoked with correct fields, RESULT line still reaches combined.
- `TestStreamToWithProgress_MalformedJSON_Ignored` — "PROGRESS {bad" line does not panic, does not call callback.
- `TestRunPi_EmitsProgressOnToolExecution` — extended runPi test with a `fakeProgressFunc` slice.
- `TestQueue_RunNext_FiresProgress` — fakeProgressNotifier gets called when fakeRunner emits progress events (new runnerInputFake that produces progress).
- `TestTgNotifier_NotifyProgress_FirstSendsThenEdits` — fakeClient records Send + Edit calls; first NotifyProgress sends, second edits with same message ID.

### 4.5 Chunk 5 — `/ask` read-only shortcut (phase AK)

Migration 0009:

```sql
-- +goose Up
ALTER TABLE tasks ADD COLUMN read_only INTEGER NOT NULL DEFAULT 0;
-- +goose Down
SELECT 1;
```

New sqlc queries:

```sql
-- name: SetReadOnly :exec
UPDATE tasks SET read_only = ? WHERE id = ?;
```

Not strictly needed for the happy path (set at create time via an extended `CreateTask` variant), but useful for future `/ask`-to-`/task` promotions. Current plan uses a direct CreateTask variant.

**Queue method.**

The naïve three-call version has a race: between `CreateTask` and `SetReadOnly`, `RunNext` could pick up the task with `read_only=0` and run it writable. Even at the 2-second poll interval, the race is real. Ship an atomic variant instead:

```sql
-- name: CreateAskTask :one
INSERT INTO tasks (description, target_repo, budget_profile, read_only, status)
VALUES (?, ?, 'quick', 1, 'queued')
RETURNING *;
```

Repo wrapper:

```go
func (r *Repo) CreateAskTask(ctx context.Context, desc, targetRepo string) (Task, error) {
    return r.q.CreateAskTask(ctx, CreateAskTaskParams{Description: desc, TargetRepo: targetRepo})
}
```

Queue method is then a thin wrapper returning the task ID:

```go
func (q *Queue) CreateAskTask(ctx context.Context, desc, targetRepo string) (int64, error) {
    task, err := q.repo.CreateAskTask(ctx, desc, targetRepo)
    if err != nil {
        return 0, err
    }
    return task.ID, nil
}
```

One migration, one sqlc query, one insert. No race window.

**Handler `/ask` route — `internal/telegram/handler.go`:**

```go
case strings.HasPrefix(text, "/ask "):
    args := strings.TrimPrefix(text, "/ask ")
    repo, desc := parseAskArgs(args)
    if repo == "" {
        _, err := h.client.SendMessage(ctx, chatID,
            "/ask needs: /ask <owner>/<repo> <question>")
        return err
    }
    id, err := h.queue.CreateAskTask(ctx, desc, repo)
    if err != nil {
        return err
    }
    _, err = h.client.SendMessage(ctx, chatID,
        fmt.Sprintf("task #%d queued (ask, repo: %s)", id, repo))
    return err
```

`parseAskArgs` requires an `owner/repo` prefix (unlike `/task` which falls back to the sandbox). Rationale: `/ask` is always about a specific repo; the sandbox repo isn't a meaningful default.

**Runner read-only path — `cmd/runner/main.go`:**

```go
readOnly := os.Getenv("ERA_READ_ONLY") == "1"

piTools := "read,write,edit,grep,find,ls,bash"
if readOnly {
    piTools = "read,grep,find,ls"
}

// ... runPi ...

if readOnly {
    // Never commit; emit result directly.
    writeResult(os.Stdout, runResult{
        Branch:    "",
        Summary:   finalSummary(summary, piErr),
        Tokens:    tokens,
        CostCents: int(math.Round(costUSD * 100)),
    })
    if piErr != nil {
        return piErr
    }
    return nil
}

// M5 pre-commit test gate + CommitAndPush + existing terminal switch
```

Orchestrator `RunInput.ReadOnly bool`. `buildDockerArgs` emits `-e ERA_READ_ONLY=1` when true.

Completion DM uses the existing no-changes path — `branch==""` → "task #N: no changes\nsummary: <pi prose>".

Tests:
- `TestCreateAskTask_SetsReadOnlyAndQuickProfile`
- `TestHandler_AskCommand_QueuesReadOnlyTask`
- `TestRunner_ReadOnly_SkipsCommitAndPush` — integration: set ERA_READ_ONLY=1, run runner, verify no `git commit`/`git push` happens (observe via test double).
- `TestBuildDockerArgs_ReadOnlyEmitsEnv`
- Live gate: `/ask vaibhav0806/trying-something what does main.go do?` — completes in <30s, DM contains Pi's prose.

### 4.6 Chunk 6 — `/stats` command (phase AL)

New sqlc queries:

```sql
-- name: CountTasksByStatusSince :many
SELECT status, COUNT(*) AS count FROM tasks WHERE created_at >= ? GROUP BY status;

-- name: SumTokensSince :one
SELECT COALESCE(SUM(tokens_used), 0) AS total FROM tasks WHERE created_at >= ?;

-- name: SumCostCentsSince :one
SELECT COALESCE(SUM(cost_cents), 0) AS total FROM tasks WHERE created_at >= ?;

-- name: CountQueuedTasks :one
SELECT COUNT(*) AS count FROM tasks WHERE status = 'queued';
```

New `internal/queue/stats.go`:

```go
package queue

import (
    "context"
    "time"
)

type PeriodStats struct {
    TasksTotal int
    TasksOK    int // completed + approved
    Tokens     int64
    CostCents  int64
}

func (p PeriodStats) SuccessRate() float64 {
    if p.TasksTotal == 0 {
        return 0
    }
    return float64(p.TasksOK) / float64(p.TasksTotal)
}

type Stats struct {
    Last24h      PeriodStats
    Last7d       PeriodStats
    Last30d      PeriodStats
    PendingQueue int
}

func (q *Queue) Stats(ctx context.Context) (Stats, error) {
    now := time.Now().UTC()
    periods := []time.Duration{24 * time.Hour, 7 * 24 * time.Hour, 30 * 24 * time.Hour}

    var s Stats
    targets := []*PeriodStats{&s.Last24h, &s.Last7d, &s.Last30d}

    for i, d := range periods {
        since := now.Add(-d)
        byStatus, err := q.repo.CountTasksByStatusSince(ctx, since)
        if err != nil { return Stats{}, err }
        for _, row := range byStatus {
            targets[i].TasksTotal += int(row.Count)
            if row.Status == "completed" || row.Status == "approved" {
                targets[i].TasksOK += int(row.Count)
            }
        }
        tok, err := q.repo.SumTokensSince(ctx, since)
        if err != nil { return Stats{}, err }
        targets[i].Tokens = tok
        cost, err := q.repo.SumCostCentsSince(ctx, since)
        if err != nil { return Stats{}, err }
        targets[i].CostCents = cost
    }
    // No mutex needed — DB handles its own locking; Queue.Stats is read-only.
    pending, err := q.repo.CountQueuedTasks(ctx)
    if err != nil { return Stats{}, err }
    s.PendingQueue = pending
    return s, nil
}
```

**Handler:**

```go
case text == "/stats":
    st, err := h.queue.Stats(ctx)
    if err != nil {
        return err
    }
    body := formatStatsDM(st)
    _, err = h.client.SendMessage(ctx, chatID, body)
    return err
```

**Formatter** (`internal/telegram/handler.go` or a new helper):

```go
func formatStatsDM(s queue.Stats) string {
    return fmt.Sprintf(
`era stats
────────────
            24h    7d     30d
tasks:      %-6d %-6d %-d
success:    %s %s %s
tokens:     %s %s %s
cost:       %s %s %s
queue: %d pending`,
        s.Last24h.TasksTotal, s.Last7d.TasksTotal, s.Last30d.TasksTotal,
        pct(s.Last24h.SuccessRate()), pct(s.Last7d.SuccessRate()), pct(s.Last30d.SuccessRate()),
        fmtK(s.Last24h.Tokens), fmtK(s.Last7d.Tokens), fmtK(s.Last30d.Tokens),
        fmtCost(s.Last24h.CostCents), fmtCost(s.Last7d.CostCents), fmtCost(s.Last30d.CostCents),
        s.PendingQueue,
    )
}

func pct(x float64) string   { return fmt.Sprintf("%-6.0f", x*100) + "%" }
func fmtK(n int64) string    { if n < 1000 { return fmt.Sprintf("%-6d", n) }; return fmt.Sprintf("%-5dk", n/1000) }
func fmtCost(c int64) string { return fmt.Sprintf("$%-5.2f", float64(c)/100.0) }
```

Tests:
- `TestStats_EmptyDB_ReturnsZeros`
- `TestStats_MixedStatuses_CountsSuccessCorrectly` — seed a set of completed/failed/approved/rejected tasks across the last 24h/7d/30d, assert counts and success rate.
- `TestHandler_StatsCommand_SendsFormattedDM` — fake queue returns canned Stats; handler sends formatted DM.

## 5. Phase order

| Phase | Chunk | Rationale |
|-------|-------|-----------|
| **AG** | §4.1 caps + profiles | Foundational (migration + config). Larger budgets reduce cap-exceeded failures during later live gates. |
| **AH** | §4.2 egress allowlist | Single-file sidecar change; enables more realistic live tasks in later phases. |
| **AI** | §4.3 reply threading | Telegram client signature change (SendMessage returns msg ID) is a prerequisite for AJ. |
| **AJ** | §4.4 progress DMs | Uses AI's signature change; progress visibility helps observe AK + AL live gates. |
| **AK** | §4.5 `/ask` read-only | Needs 0009 migration; benefits from AJ's progress + AH's broader allowlist for realistic read-only repos. |
| **AL** | §4.6 `/stats` | Zero-risk read-only addition; surfaces M6's live-gate data as the final proof. |

Migration numbering: 0007 (AG), 0008 (AI), 0009 (AK) — matches phase order.

## 6. Testing philosophy

TDD for every new function. Fail-first tests before implementation. `go test -race -count=1 ./...` green before every commit. Per-phase smoke scripts kept as regression guards. Live Telegram smoke at every phase gate — no phase is "done" until the live round-trip passes. Subagent-driven execution: fresh implementer per task, two-stage review (spec compliance + code quality). CI (M5 AF) re-runs all phase smokes on every push; deploy only on green.

**We are cautious. We are serious. We are productive. We do not build blindly.**

## 7. Risk log

1. **Telegram client API churn (AI).** SendMessage signature change cascades to every caller. Grep catches them; one commit fixes all. Tests pin the new signature.

2. **Progress message orphan on crash.** Mid-run crash leaves a pinned "iter 7 · ..." message without a completion DM. Completion DM for the same task when reconcile catches it (failed state) clarifies. Non-fatal, cosmetic.

3. **Reply target missing.** User replies to a pre-AI completion DM (no `completion_message_id`) → handler DMs "couldn't find the original task". Clear feedback, no crash.

4. **`/ask` writable tool escape.** Pi's CLI enforces `--tools` whitelist. Even if Pi ignored the flag, runner's ReadOnly branch bypasses CommitAndPush entirely. Defense in depth.

5. **Budget flag false positive.** Parser only looks at the first token; `--budget=` appearing later in description is untouched. Test covers the edge.

6. **Progress edit rate limit.** Telegram caps ~30 edits/sec/chat; real tasks emit 5-10 events over minutes. Worst case: edits fail, logged, task still completes.

7. **PROGRESS JSON schema drift.** Malformed JSON silently drops the progress event (unknown line). Smoke catches regression.

8. **`/stats` query performance.** O(<1ms) at today's volume; scales to 10k tasks before latency noticed. `idx_tasks_status` and `idx_tasks_created_at` indexes from M0 already cover the predicates.

## 8. Deliverables

Per-phase:
- AG: 6-7 commits (migration 0007, sqlc regen, budget lib + tests, queue CreateTask sig, runner env overrides, handler parse, .env defaults, smoke).
- AH: 2-3 commits (static additions + PI_EGRESS_EXTRA + tests + smoke).
- AI: 6-8 commits (migration 0008, sqlc regen, telegram client sig, caller fanout, tgNotifier storage, reply_compose + tests, handler reply route, smoke).
- AJ: 6-7 commits (writeProgress, pi.go callback, main.go wire, streamToWithProgress, ProgressNotifier, EditMessageText client method, tgNotifier.NotifyProgress, smoke).
- AK: 5-6 commits (migration 0009, sqlc regen, CreateAskTask, handler /ask route, runner ERA_READ_ONLY path, buildDockerArgs, smoke).
- AL: 3-4 commits (4 new queries, sqlc regen, stats lib + tests, handler /stats route, smoke).

Estimate: ~30-40 commits total. Smaller than M4, similar to M5.

After all phases:
- README.md prepend M6 status + roadmap row.
- `git tag m6-release && git push origin master m6-release`.
- CI auto-deploys on the tag push; live gate is the full system responding to `/stats` and returning real data from M6-completed tasks.

## 9. Out of scope (listed here so future-me doesn't sneak these into M6)

- PR auto-merge on approve
- Natural-language repo parsing (alias table)
- Long-answer Telegram file attachments
- Scheduled / recurring tasks
- Task chaining
- Multi-repo fan-out
- Dev/prod bot split
- sqlc drift check in CI
- `/stats` detailed breakdown (top-cost tasks, failure patterns)
- HTTP-based progress transport
- Dedicated read-only sandbox image
- Transitive reply conversation chains
- Reconcile deleting orphan progress messages
- Rate-limiting progress edits
- `/ask --budget=<x>` variants (always `quick`)
- Alpine CDN allowlist expansion
- Java/PHP/Ruby toolchain bake into runner image
