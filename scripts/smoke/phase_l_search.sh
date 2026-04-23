#!/usr/bin/env bash
# Phase L smoke: unit tests for /search + /fetch + prompt preamble are green
# and the image builds with those binaries embedded.
set -euo pipefail
go test -race -count=1 -run 'TestSearch_|TestFetch_|TestComposePrompt_' ./cmd/... > /dev/null
make docker-runner > /dev/null
# The /search + /fetch endpoints are loopback-only inside the container;
# live-exercising them requires a Telegram task (Phase M onwards). This smoke
# just proves the code ships inside the image.
docker run --rm era-runner:m2 2>&1 | grep -q "runner config: ERA_TASK_ID is required"
echo "OK: phase L — /search + /fetch + prompt preamble shipped in era-runner:m2"
