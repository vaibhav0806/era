#!/usr/bin/env bash
# Phase R smoke: approval state transitions + BranchDeleter unit tests.
# The full live callback round-trip is verified manually (see commit log
# for the M3-15 task: diffscan-flagged task got an approval DM; rejecting
# deleted the branch on GitHub and transitioned task to rejected).
set -euo pipefail
go test -race -count=1 -run 'TestQueue_ApproveTask|TestQueue_RejectTask|TestDeleteBranch_|TestQueue_RunNext_FlaggedDiff_CallsNotifyNeedsReview' \
    ./internal/queue/... ./internal/githubbranch/... > /dev/null
echo "OK: phase R — approval state machine + BranchDeleter all tests green (manual smoke verified in M3-15)"
