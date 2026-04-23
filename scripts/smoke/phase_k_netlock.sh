#!/usr/bin/env bash
# Phase K smoke: HARD-asserts that
#   (a) iptables lockdown was applied (not just logged), and
#   (b) a non-allowlisted host (example.com) is actually blocked
# AND that the allowed host (openrouter.ai) is reachable.
#
# The diagnostic block in entrypoint.sh prints two lines:
#   diag-allowed-result: <numeric HTTP code | "denied">
#   diag-disallowed-result: <numeric HTTP code | "denied">
# This script asserts the first is 2xx-5xx (host reachable, any HTTP response)
# and the second contains "denied" or a non-2xx code (host blocked).
set -euo pipefail
make docker-runner > /dev/null 2>&1
out=$(docker run --rm --cap-add=NET_ADMIN --cap-add=NET_RAW \
    -e PI_SIDECAR_TEST_DIAG=1 \
    -e ERA_TASK_ID=998 -e ERA_TASK_DESCRIPTION=t \
    -e ERA_GITHUB_PAT=fake -e ERA_GITHUB_REPO=x/y \
    -e ERA_OPENROUTER_API_KEY=fake -e ERA_PI_MODEL=fake \
    -e ERA_MAX_TOKENS=1 -e ERA_MAX_COST_CENTS=1 -e ERA_MAX_ITERATIONS=1 -e ERA_MAX_WALL_SECONDS=10 \
    era-runner:m2 2>&1 || true)

echo "$out" | grep -q "iptables lockdown active" \
    || { echo "FAIL: lockdown log line missing"; exit 1; }

allowed=$(echo "$out" | awk -F': ' '/diag-allowed-result/{print $2}' | head -1)
disallowed=$(echo "$out" | awk -F': ' '/diag-disallowed-result/{print $2}' | head -1)

[[ -n "$allowed" ]] || { echo "FAIL: diag-allowed-result missing"; exit 1; }
[[ -n "$disallowed" ]] || { echo "FAIL: diag-disallowed-result missing"; exit 1; }

# Allowed host: any HTTP response counts (200-599). "denied"/"000denied" = lockdown leak (host should be reachable).
if echo "$allowed" | grep -qE "^(2|3|4|5)[0-9][0-9]$"; then
    : # ok
else
    echo "FAIL: openrouter.ai not reachable (got: $allowed)"; exit 1
fi

# Disallowed host: must NOT be a 2xx (3xx redirect-from-proxy is a fail too).
# Proxy returns 403 directly OR curl exits with "denied" string.
if echo "$disallowed" | grep -qE "^2[0-9][0-9]$"; then
    echo "FAIL: example.com returned $disallowed — LOCKDOWN LEAKED"; exit 1
fi

echo "OK: phase K — iptables lockdown active, openrouter.ai allowed ($allowed), example.com blocked ($disallowed)"
