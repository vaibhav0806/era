#!/usr/bin/env bash
# Phase AL smoke: stats queries + Queue.Stats + handler /stats route.
set -euo pipefail
go test -race -count=1 -run 'TestStats_|TestPeriodStats_|TestHandler_StatsCommand_' \
    ./internal/stats/... ./internal/queue/... ./internal/telegram/... > /dev/null
echo "OK: phase AL — /stats command all unit tests green"
