#!/usr/bin/env bash
# Phase AH smoke: new static hosts + PI_EGRESS_EXTRA parser unit tests.
set -euo pipefail
go test -race -count=1 -run 'TestAllowlist_StaticHostsAllowed|TestPIEgressExtra_' \
    ./cmd/sidecar/... > /dev/null
echo "OK: phase AH — egress allowlist expansion all unit tests green"
