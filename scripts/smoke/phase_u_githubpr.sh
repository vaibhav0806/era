#!/usr/bin/env bash
# Phase U smoke: githubpr package (Create, Close, DefaultBranch) unit tests.
# Live integration test is in the E2E suite (phase Y).
set -euo pipefail
go test -race -count=1 -run 'TestNew_DefaultsPopulated|TestDefaultBranch|TestCreate_|TestClose_' \
    ./internal/githubpr/... > /dev/null
echo "OK: phase U — githubpr package all tests green"
