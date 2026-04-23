package queue_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vaibhav0806/era/internal/audit"
	"github.com/vaibhav0806/era/internal/db"
	"github.com/vaibhav0806/era/internal/diffscan"
	"github.com/vaibhav0806/era/internal/queue"
)

type fakeRunner struct {
	branch    string
	summary   string
	tokens    int64
	costCents int
	audits    []audit.Entry
	err       error
	calls     int
	lastID    int64
	lastDes   string
	lastToken string
}

func (f *fakeRunner) Run(ctx context.Context, taskID int64, desc string, ghToken string) (string, string, int64, int, []audit.Entry, error) {
	f.calls++
	f.lastID = taskID
	f.lastDes = desc
	f.lastToken = ghToken
	return f.branch, f.summary, f.tokens, f.costCents, f.audits, f.err
}

type fakeTokens struct {
	token string
	err   error
}

func (f *fakeTokens) InstallationToken(ctx context.Context) (string, error) {
	return f.token, f.err
}

func newRunQueue(t *testing.T, r queue.Runner) (*queue.Queue, *db.Repo) {
	t.Helper()
	return newRunQueueWithTokens(t, r, nil)
}

func newRunQueueWithTokens(t *testing.T, r queue.Runner, tokens queue.TokenSource) (*queue.Queue, *db.Repo) {
	t.Helper()
	return newRunQueueWithDeps(t, r, tokens, nil, "")
}

func newRunQueueWithDeps(t *testing.T, r queue.Runner, tokens queue.TokenSource, compare queue.DiffSource, repoFQN string) (*queue.Queue, *db.Repo) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t.db")
	h, err := db.Open(context.Background(), path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = h.Close() })
	repo := db.NewRepo(h)
	return queue.New(repo, r, tokens, compare, repoFQN), repo
}

type fakeCompare struct {
	diffs []diffscan.FileDiff
	err   error
	calls int
}

func (f *fakeCompare) Compare(ctx context.Context, repo, base, head string) ([]diffscan.FileDiff, error) {
	f.calls++
	return f.diffs, f.err
}

func TestQueue_RunNext_Success(t *testing.T) {
	ctx := context.Background()
	fr := &fakeRunner{branch: "agent/1/x", summary: "ok"}
	q, repo := newRunQueue(t, fr)

	id, err := q.CreateTask(ctx, "do x")
	require.NoError(t, err)

	ran, err := q.RunNext(ctx)
	require.NoError(t, err)
	require.True(t, ran)
	require.Equal(t, 1, fr.calls)
	require.Equal(t, id, fr.lastID)
	require.Equal(t, "do x", fr.lastDes)

	got, err := repo.GetTask(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "completed", got.Status)
	require.Equal(t, "agent/1/x", got.BranchName.String)
	require.Equal(t, "ok", got.Summary.String)

	// No more tasks.
	ran, err = q.RunNext(ctx)
	require.NoError(t, err)
	require.False(t, ran)
}

func TestQueue_RunNext_Failure(t *testing.T) {
	ctx := context.Background()
	fr := &fakeRunner{err: errors.New("container exploded")}
	q, repo := newRunQueue(t, fr)

	id, err := q.CreateTask(ctx, "boom")
	require.NoError(t, err)

	ran, err := q.RunNext(ctx)
	require.True(t, ran)
	require.Error(t, err)
	require.Contains(t, err.Error(), "container exploded")

	got, err := repo.GetTask(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "failed", got.Status)
	require.Contains(t, got.Error.String, "exploded")
}

func TestQueue_RunNext_EmitsEvents(t *testing.T) {
	ctx := context.Background()
	fr := &fakeRunner{branch: "agent/1/y", summary: "ok"}
	q, repo := newRunQueue(t, fr)

	id, _ := q.CreateTask(ctx, "x")
	_, err := q.RunNext(ctx)
	require.NoError(t, err)

	events, err := repo.ListEvents(ctx, id)
	require.NoError(t, err)
	require.Len(t, events, 2)
	require.Equal(t, "started", events[0].Kind)
	require.Equal(t, "completed", events[1].Kind)
}

func TestQueue_RunNext_FailureEmitsEvent(t *testing.T) {
	ctx := context.Background()
	fr := &fakeRunner{err: errors.New("nope")}
	q, repo := newRunQueue(t, fr)

	id, _ := q.CreateTask(ctx, "x")
	_, _ = q.RunNext(ctx)

	events, err := repo.ListEvents(ctx, id)
	require.NoError(t, err)
	require.Len(t, events, 2)
	require.Equal(t, "started", events[0].Kind)
	require.Equal(t, "failed", events[1].Kind)
}

func TestQueue_RunNext_NoTasks(t *testing.T) {
	ctx := context.Background()
	fr := &fakeRunner{}
	q, _ := newRunQueue(t, fr)

	ran, err := q.RunNext(ctx)
	require.NoError(t, err)
	require.False(t, ran)
	require.Equal(t, 0, fr.calls)
}

type completedArgs struct {
	ID        int64
	Branch    string
	Summary   string
	Tokens    int64
	CostCents int
}
type failedArgs struct {
	ID     int64
	Reason string
}

type fakeNotifier struct {
	completed []completedArgs
	failed    []failedArgs
}

func (f *fakeNotifier) NotifyCompleted(ctx context.Context, id int64, b, s string, t int64, c int) {
	f.completed = append(f.completed, completedArgs{id, b, s, t, c})
}
func (f *fakeNotifier) NotifyFailed(ctx context.Context, id int64, r string) {
	f.failed = append(f.failed, failedArgs{id, r})
}

func TestQueue_Notifier_OnSuccess(t *testing.T) {
	ctx := context.Background()
	fr := &fakeRunner{branch: "agent/1/ok", summary: "done"}
	q, _ := newRunQueue(t, fr)
	n := &fakeNotifier{}
	q.SetNotifier(n)

	id, _ := q.CreateTask(ctx, "work")
	_, err := q.RunNext(ctx)
	require.NoError(t, err)

	require.Len(t, n.completed, 1)
	require.Equal(t, id, n.completed[0].ID)
	require.Equal(t, "agent/1/ok", n.completed[0].Branch)
	require.Equal(t, "done", n.completed[0].Summary)
	require.Len(t, n.failed, 0)
}

func TestQueue_Notifier_OnFailure(t *testing.T) {
	ctx := context.Background()
	fr := &fakeRunner{err: errors.New("boom")}
	q, _ := newRunQueue(t, fr)
	n := &fakeNotifier{}
	q.SetNotifier(n)

	id, _ := q.CreateTask(ctx, "work")
	_, _ = q.RunNext(ctx)

	require.Len(t, n.failed, 1)
	require.Equal(t, id, n.failed[0].ID)
	require.Contains(t, n.failed[0].Reason, "boom")
	require.Len(t, n.completed, 0)
}

func TestQueue_Notifier_NilSafe(t *testing.T) {
	// If no notifier is attached, RunNext must not panic.
	ctx := context.Background()
	fr := &fakeRunner{branch: "b", summary: "s"}
	q, _ := newRunQueue(t, fr)
	// intentionally no SetNotifier

	_, _ = q.CreateTask(ctx, "x")
	_, err := q.RunNext(ctx)
	require.NoError(t, err)
}

func TestQueue_RunNext_RecordsTokensAndCost(t *testing.T) {
	ctx := context.Background()
	fr := &fakeRunner{branch: "b", summary: "s", tokens: 4321, costCents: 9}
	q, repo := newRunQueue(t, fr)
	id, _ := q.CreateTask(ctx, "x")
	_, err := q.RunNext(ctx)
	require.NoError(t, err)
	got, err := repo.GetTask(ctx, id)
	require.NoError(t, err)
	require.Equal(t, int64(4321), got.TokensUsed)
	require.Equal(t, int64(9), got.CostCents)
}

func TestQueue_RunNext_PersistsAuditEntries(t *testing.T) {
	ctx := context.Background()
	fr := &fakeRunner{
		branch: "b", summary: "s", tokens: 1, costCents: 1,
		audits: []audit.Entry{
			{Method: "GET", Path: "/health", Status: 200},
			{Method: "CONNECT", Host: "github.com", Status: 200},
		},
	}
	q, repo := newRunQueue(t, fr)
	id, _ := q.CreateTask(ctx, "x")
	_, err := q.RunNext(ctx)
	require.NoError(t, err)

	events, err := repo.ListEvents(ctx, id)
	require.NoError(t, err)
	// Expect: started, completed, + 2 http_request events = 4
	httpReqs := 0
	for _, e := range events {
		if e.Kind == "http_request" {
			httpReqs++
		}
	}
	require.Equal(t, 2, httpReqs)
}

func TestQueue_RunNext_PassesGhTokenFromSource(t *testing.T) {
	ctx := context.Background()
	fr := &fakeRunner{branch: "b", summary: "s"}
	tokens := &fakeTokens{token: "ghs_test_token_123"}
	q, _ := newRunQueueWithTokens(t, fr, tokens)
	_, _ = q.CreateTask(ctx, "x")
	_, err := q.RunNext(ctx)
	require.NoError(t, err)
	require.Equal(t, "ghs_test_token_123", fr.lastToken, "runner should receive the minted token")
}

func TestQueue_RunNext_TokenMintFailure(t *testing.T) {
	ctx := context.Background()
	fr := &fakeRunner{}
	tokens := &fakeTokens{err: errors.New("github down")}
	q, repo := newRunQueueWithTokens(t, fr, tokens)
	id, _ := q.CreateTask(ctx, "x")
	_, err := q.RunNext(ctx)
	require.Error(t, err)
	task, _ := repo.GetTask(ctx, id)
	require.Equal(t, "failed", task.Status)
	require.Contains(t, task.Error.String, "token mint")
}

func TestQueue_RunNext_CleanDiff_StaysCompleted(t *testing.T) {
	ctx := context.Background()
	fr := &fakeRunner{branch: "agent/1/x", summary: "s"}
	fc := &fakeCompare{diffs: []diffscan.FileDiff{
		{Path: "foo.go", Added: []string{"foo"}},
	}}
	q, repo := newRunQueueWithDeps(t, fr, nil, fc, "a/b")
	id, _ := q.CreateTask(ctx, "x")
	_, err := q.RunNext(ctx)
	require.NoError(t, err)
	task, _ := repo.GetTask(ctx, id)
	require.Equal(t, "completed", task.Status)
	require.Equal(t, 1, fc.calls, "compare should have been called exactly once")
}

func TestQueue_RunNext_FlaggedDiff_SetsNeedsReview(t *testing.T) {
	ctx := context.Background()
	fr := &fakeRunner{branch: "agent/1/x", summary: "s"}
	fc := &fakeCompare{diffs: []diffscan.FileDiff{
		{Path: "foo_test.go", Removed: []string{"func TestBar(t *testing.T) {}"}},
	}}
	q, repo := newRunQueueWithDeps(t, fr, nil, fc, "a/b")
	id, _ := q.CreateTask(ctx, "x")
	_, err := q.RunNext(ctx)
	require.NoError(t, err)
	task, _ := repo.GetTask(ctx, id)
	require.Equal(t, "needs_review", task.Status)

	events, _ := repo.ListEvents(ctx, id)
	sawFlag := false
	for _, e := range events {
		if e.Kind == "diffscan_flagged" {
			sawFlag = true
			require.Contains(t, e.Payload, "removed_test")
		}
	}
	require.True(t, sawFlag)
}

func TestQueue_RunNext_CompareError_LogsEventButDoesntBlock(t *testing.T) {
	ctx := context.Background()
	fr := &fakeRunner{branch: "agent/1/x", summary: "s"}
	fc := &fakeCompare{err: errors.New("github 404")}
	q, repo := newRunQueueWithDeps(t, fr, nil, fc, "a/b")
	id, _ := q.CreateTask(ctx, "x")
	_, err := q.RunNext(ctx)
	require.NoError(t, err)
	task, _ := repo.GetTask(ctx, id)
	require.Equal(t, "completed", task.Status)

	events, _ := repo.ListEvents(ctx, id)
	sawErr := false
	for _, e := range events {
		if e.Kind == "diffscan_error" {
			sawErr = true
		}
	}
	require.True(t, sawErr)
}

func TestQueue_RunNext_NoCompareClient_NoDiffscan(t *testing.T) {
	ctx := context.Background()
	fr := &fakeRunner{branch: "agent/1/x", summary: "s"}
	q, repo := newRunQueueWithDeps(t, fr, nil, nil, "") // compare == nil
	id, _ := q.CreateTask(ctx, "x")
	_, err := q.RunNext(ctx)
	require.NoError(t, err)
	task, _ := repo.GetTask(ctx, id)
	require.Equal(t, "completed", task.Status)
}
