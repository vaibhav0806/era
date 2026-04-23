#!/bin/sh
# /usr/local/bin/era-entrypoint
# Starts the sidecar in the background, waits for it to be ready, then execs
# the runner. Phase K will add iptables setup before the runner exec.
set -eu

# Sidecar listens on loopback so only in-container processes can reach it.
export PI_SIDECAR_LISTEN_ADDR="127.0.0.1:8080"

/usr/local/bin/era-sidecar &
SIDECAR_PID=$!

# Wait up to ~5s for /health.
for i in 1 2 3 4 5 10 20 30; do
    if wget -q -O - http://127.0.0.1:8080/health 2>/dev/null | grep -q "^ok$"; then
        echo "sidecar ready (pid=$SIDECAR_PID)" >&2
        break
    fi
    sleep 0.1
done

# Hard-fail if /health never returned ok within budget.
if ! wget -q -O - http://127.0.0.1:8080/health 2>/dev/null | grep -q "^ok$"; then
    echo "FATAL: sidecar failed to start" >&2
    exit 1
fi

# Hand off to runner. Sidecar continues in background.
exec /usr/local/bin/era-runner "$@"
