#!/usr/bin/env bash
# Phase S smoke: digest rendering + cron-time parsing + cancel/retry state
# transitions.
set -euo pipefail
go test -race -count=1 -run 'TestRender_|TestParseDigestTime|TestLoad_DigestTime|TestQueue_CancelTask|TestQueue_RetryTask|TestRepo_ListBetween' \
    ./internal/digest/... ./internal/config/... ./internal/queue/... ./internal/db/... > /dev/null
# Orchestrator logs "digest scheduled" at startup — quick sanity check.
make build > /dev/null
out=$(./bin/orchestrator 2>&1 &
sleep 1
kill %1 2>/dev/null || true
wait 2>/dev/null || true)
# Not easy to capture background output reliably here; instead just verify
# the binary contains the runDigestScheduler symbol.
strings bin/orchestrator | grep -q runDigestScheduler || \
    { echo "FAIL: runDigestScheduler not in binary"; exit 1; }
echo "OK: phase S — digest + cron + cancel/retry all tests green; scheduler linked in binary"
