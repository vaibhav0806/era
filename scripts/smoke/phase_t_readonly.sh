#!/usr/bin/env bash
# Phase T smoke: runner captures pi's last assistant text and surfaces it as
# the result summary; orchestrator truncates for telegram without splitting runes.
set -euo pipefail
go test -race -count=1 -run 'TestParseEvent_MessageEndWithTextContent|TestRunPi_TracksLastAssistantText|TestRunPi_LastTextEmptyWhenNoAssistantMessage|TestWriteResult_EmitsJSONLine|TestFinalSummary_|TestTruncateForTelegram_' \
    ./cmd/runner/... ./cmd/orchestrator/... > /dev/null
echo "OK: phase T — read-only answer path all unit tests green"
