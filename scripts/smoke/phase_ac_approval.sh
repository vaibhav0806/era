#!/usr/bin/env bash
# Phase AC smoke: githubpr new methods + queue approve/reject wiring.
# Live GitHub PR round-trip verified manually.
set -euo pipefail
go test -race -count=1 -run 'TestApprovePR_|TestAddLabel_|TestAddComment_|TestApproveTask_|TestRejectTask_|TestRejectionCommentBody_' \
    ./internal/githubpr/... ./internal/queue/... > /dev/null
echo "OK: phase AC — PR approval feedback unit tests green"
