package runner

import (
	"context"

	"github.com/vaibhav0806/era/internal/audit"
)

// QueueAdapter adapts a *Docker to the queue.Runner interface.
type QueueAdapter struct {
	D *Docker
}

func (q QueueAdapter) Run(ctx context.Context, taskID int64, description string, ghToken string) (string, string, int64, int, []audit.Entry, error) {
	out, err := q.D.Run(ctx, RunInput{TaskID: taskID, Description: description, GitHubToken: ghToken})
	if err != nil {
		return "", "", 0, 0, nil, err
	}
	return out.Branch, out.Summary, out.Tokens, out.CostCents, out.Audits, nil
}
