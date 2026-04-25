#!/usr/bin/env bash
# Phase AK smoke: CreateAskTask atomicity + handler /ask route + buildDockerArgs ReadOnly.
set -euo pipefail
go test -race -count=1 -run 'TestRepo_CreateAskTask_|TestQueue_CreateAskTask_|TestHandler_AskCommand_|TestHandler_AskWithoutRepo_|TestBuildDockerArgs_ReadOnly|TestBuildDockerArgs_NotReadOnly' \
    ./internal/db/... ./internal/queue/... ./internal/telegram/... ./internal/runner/... > /dev/null
echo "OK: phase AK — /ask read-only all unit tests green"
