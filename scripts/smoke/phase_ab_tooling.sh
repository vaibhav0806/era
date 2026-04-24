#!/usr/bin/env bash
# Phase AB smoke: runner image contains the expected language toolchains +
# utilities. Tool verification is a simple "does the binary exist and print
# a version" check — deep correctness is Pi's job at task time.
set -euo pipefail

IMAGE="${IMAGE:-era-runner:m2}"

TOOLS=(
    "node --version"
    "npm --version"
    "python3 --version"
    "pip3 --version"
    "cargo --version"
    "rustc --version"
    "go version"
    "rg --version"
    "fd --version"
    "tree --version"
    "sqlite3 --version"
)

for cmd in "${TOOLS[@]}"; do
    if ! docker run --rm --entrypoint sh "$IMAGE" -c "$cmd" > /dev/null 2>&1; then
        echo "FAIL: '$cmd' did not succeed inside $IMAGE"
        docker run --rm --entrypoint sh "$IMAGE" -c "$cmd" 2>&1 | head -3
        exit 1
    fi
done

echo "OK: phase AB — runner image tooling bake verified (${#TOOLS[@]} binaries)"
