package digest_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/vaibhav0806/era/internal/db"
	"github.com/vaibhav0806/era/internal/digest"
)

// task creates a fully-populated db.Task for testing. Fields not specified
// default to sensible zeros.
func task(id int64, desc, status string, tokens, cents int64) db.Task {
	t := db.Task{
		ID:          id,
		Description: desc,
		Status:      status,
		TokensUsed:  tokens,
		CostCents:   cents,
	}
	return t
}

func taskWithBranch(id int64, desc, status, branch string, tokens, cents int64) db.Task {
	t := task(id, desc, status, tokens, cents)
	t.BranchName = sql.NullString{String: branch, Valid: true}
	return t
}

func TestRender_Empty(t *testing.T) {
	from := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	out := digest.Render(nil, from, to)
	require.Contains(t, out, "0 tasks")
}

func TestRender_Counts(t *testing.T) {
	from := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	tasks := []db.Task{
		taskWithBranch(1, "add foo", "completed", "agent/1/foo", 100, 5),
		taskWithBranch(2, "refactor bar", "needs_review", "agent/2/bar", 3000, 17),
		task(3, "broken", "failed", 0, 0),
		task(4, "approved thing", "approved", 500, 2),
		task(5, "never did", "cancelled", 0, 0),
		task(6, "rejected change", "rejected", 200, 1),
	}
	out := digest.Render(tasks, from, to)
	require.Contains(t, out, "6 tasks")
	require.Contains(t, out, "1 completed")
	require.Contains(t, out, "1 needs_review")
	require.Contains(t, out, "1 failed")
	require.Contains(t, out, "1 approved")
	require.Contains(t, out, "1 cancelled")
	require.Contains(t, out, "1 rejected")
}

func TestRender_TotalsTokensAndCost(t *testing.T) {
	from := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	tasks := []db.Task{
		task(1, "a", "completed", 1000, 15),
		task(2, "b", "failed", 500, 3),
		task(3, "c", "completed", 2000, 27),
	}
	out := digest.Render(tasks, from, to)
	require.Contains(t, out, "3,500") // total tokens, with commas
	require.Contains(t, out, "$0.45") // $0.15 + $0.03 + $0.27 = $0.45
}

func TestRender_ListsEachTaskWithStatus(t *testing.T) {
	from := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	tasks := []db.Task{
		taskWithBranch(1, "first task", "completed", "agent/1/first", 100, 5),
		taskWithBranch(2, "second", "needs_review", "agent/2/second", 200, 10),
	}
	out := digest.Render(tasks, from, to)
	require.Contains(t, out, "#1")
	require.Contains(t, out, "first task")
	require.Contains(t, out, "agent/1/first")
	require.Contains(t, out, "#2")
	require.Contains(t, out, "second")
	require.Contains(t, out, "needs_review")
}

func TestRender_Deterministic(t *testing.T) {
	// Same input twice produces byte-identical output. This locks in that
	// Render is a pure function.
	from := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	tasks := []db.Task{
		taskWithBranch(1, "x", "completed", "agent/1/x", 100, 5),
		taskWithBranch(2, "y", "needs_review", "agent/2/y", 200, 10),
	}
	out1 := digest.Render(tasks, from, to)
	out2 := digest.Render(tasks, from, to)
	require.Equal(t, out1, out2)
}

func TestRender_TimeRangeInHeader(t *testing.T) {
	from := time.Date(2026, 4, 23, 5, 30, 0, 0, time.UTC)
	to := time.Date(2026, 4, 24, 5, 30, 0, 0, time.UTC)
	out := digest.Render(nil, from, to)
	// Window appears somewhere in the header (exact formatting flexible).
	require.Contains(t, out, "2026-04-23")
}
