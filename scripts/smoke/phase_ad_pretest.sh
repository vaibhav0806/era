#!/usr/bin/env bash
# Phase AD smoke: pre-commit test detector + runner + gate wrapper.
# 10-minute timeout test is opt-in via ERA_TEST_LONG=1.
set -euo pipefail
go test -race -count=1 -run 'TestHasMakefileTest_|TestRunMakefileTest_Pass|TestRunMakefileTest_Fail|TestPreCommitGate_' \
    ./cmd/runner/... > /dev/null
echo "OK: phase AD — pre-commit test gate all unit tests green"
