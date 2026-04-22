#!/usr/bin/env bash
# Phase G smoke: cmd/runner is unit-test green and cross-compiles for the
# Linux/amd64 target the Docker image expects.
set -euo pipefail
go test -race -count=1 -run 'TestParseEvent|TestStreamEvents|TestPi_|TestCaps_|TestGit_|TestWriteResult|TestRunnerConfig' ./cmd/runner/... > /dev/null
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o /tmp/era-runner ./cmd/runner
file /tmp/era-runner | grep -q "ELF 64-bit"
rm /tmp/era-runner
echo "OK: phase G — cmd/runner unit tests green + linux/amd64 cross-compile green"
