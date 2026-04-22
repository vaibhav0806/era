#!/usr/bin/env bash
# Phase H smoke: era-runner:m1 image builds, the runner binary inside it
# rejects missing config with a clear error, Pi accepts the flags we pass.
set -euo pipefail
make docker-runner > /dev/null
docker images era-runner:m1 --format '{{.Size}}' | head -1
docker run --rm era-runner:m1 2>&1 | grep -q "ERA_TASK_ID is required"
docker run --rm --entrypoint pi era-runner:m1 --help 2>&1 | grep -qE -- "--mode"
echo "OK: phase H — image builds, runner config-error works, pi --help responds"
