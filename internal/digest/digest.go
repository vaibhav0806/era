// Package digest renders a Telegram-safe summary of tasks completed within
// a time window. Pure function — no I/O, no wall-clock, no DB access.
package digest

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/vaibhav0806/era/internal/db"
)

// Render returns a plain-text summary of the given tasks for the [from, to)
// window. Deterministic for fixed input.
func Render(tasks []db.Task, from, to time.Time) string {
	var b strings.Builder

	fmt.Fprintf(&b, "era digest — %s (UTC)\n", from.Format("2006-01-02"))
	fmt.Fprintf(&b, "window: %s → %s (UTC)\n\n",
		from.Format("15:04"), to.Format("15:04"))

	fmt.Fprintf(&b, "%d tasks total\n", len(tasks))

	if len(tasks) == 0 {
		return b.String()
	}

	// Per-status counts, sorted alphabetically for determinism.
	counts := map[string]int{}
	var totalTokens, totalCents int64
	for _, t := range tasks {
		counts[t.Status]++
		totalTokens += t.TokensUsed
		totalCents += t.CostCents
	}
	statuses := make([]string, 0, len(counts))
	for s := range counts {
		statuses = append(statuses, s)
	}
	sort.Strings(statuses)
	for _, s := range statuses {
		fmt.Fprintf(&b, "  %d %s\n", counts[s], s)
	}

	fmt.Fprintf(&b, "\ntokens: %s | cost: $%.2f\n\n",
		commafy(totalTokens), float64(totalCents)/100.0)

	// Per-task list, sorted by ID for determinism.
	sorted := make([]db.Task, len(tasks))
	copy(sorted, tasks)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	b.WriteString("tasks:\n")
	for _, t := range sorted {
		branch := ""
		if t.BranchName.Valid && t.BranchName.String != "" {
			branch = " → " + t.BranchName.String
		}
		desc := truncate(t.Description, 60)
		fmt.Fprintf(&b, "  #%d %s: %s%s\n", t.ID, t.Status, desc, branch)
	}

	return b.String()
}

// commafy inserts thousands separators. "3500" → "3,500".
func commafy(n int64) string {
	s := fmt.Sprintf("%d", n)
	if n < 0 {
		return "-" + commafy(-n)
	}
	if len(s) <= 3 {
		return s
	}
	var out []byte
	rem := len(s) % 3
	if rem > 0 {
		out = append(out, s[:rem]...)
		if len(s) > rem {
			out = append(out, ',')
		}
	}
	for i := rem; i < len(s); i += 3 {
		out = append(out, s[i:i+3]...)
		if i+3 < len(s) {
			out = append(out, ',')
		}
	}
	return string(out)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
