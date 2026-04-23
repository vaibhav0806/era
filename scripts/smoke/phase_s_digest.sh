#!/usr/bin/env bash
# Phase S smoke: digest rendering + cron-time parsing + cancel/retry state
# transitions. The live digest arrival is verified by observing the bot at
# PI_DIGEST_TIME_UTC (default 17:30 UTC = 11 PM IST).
set -euo pipefail
go test -race -count=1 -run 'TestRender_|TestParseDigestTime|TestLoad_DigestTime|TestQueue_CancelTask|TestQueue_RetryTask|TestRepo_ListBetween' \
    ./internal/digest/... ./internal/config/... ./internal/queue/... ./internal/db/... > /dev/null
echo "OK: phase S — digest render + cron parse + cancel/retry + list-between all green"
