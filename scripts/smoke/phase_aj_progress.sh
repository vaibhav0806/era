#!/usr/bin/env bash
# Phase AJ smoke: writeProgress + runPi callback + StreamToWithProgress + Queue ProgressNotifier.
set -euo pipefail
go test -race -count=1 -run 'TestWriteProgress_|TestRunPi_FiresProgress|TestRunPi_NilProgress|TestStreamToWithProgress_|TestQueue_RunNext_FiresProgress' \
    ./cmd/runner/... ./internal/runner/... ./internal/queue/... > /dev/null
echo "OK: phase AJ — progress DM pipeline all unit tests green"
