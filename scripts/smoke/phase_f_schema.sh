#!/usr/bin/env bash
# Phase F smoke: verify migrations 0001 + 0002 apply cleanly on a fresh DB
# and produce the expected schema (including the new tokens_used / cost_cents
# columns on tasks).
set -euo pipefail
TMP=$(mktemp -t era-phasef.XXXXXX.db)
trap "rm -f $TMP $TMP-wal $TMP-shm" EXIT
goose -dir migrations sqlite3 "$TMP" up 2>&1 | tail -3
sqlite3 "$TMP" "PRAGMA table_info(tasks);" | grep -q tokens_used
sqlite3 "$TMP" "PRAGMA table_info(tasks);" | grep -q cost_cents
echo "OK: phase F — migrations 0001+0002 applied, tokens_used + cost_cents present"
