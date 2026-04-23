#!/usr/bin/env bash
# Phase Q smoke: telegram client + handler support inline keyboards and
# callback-query routing. Unit-tests-only; live Telegram callback exchange
# is verified in Phase R manual smoke.
set -euo pipefail
go test -race -count=1 -run 'TestFakeClient_|TestHandler_' ./internal/telegram/... > /dev/null
echo "OK: phase Q — inline keyboards + callback routing + /cancel + /retry unit tests green"
