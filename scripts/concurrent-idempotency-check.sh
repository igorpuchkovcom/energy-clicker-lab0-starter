#!/usr/bin/env bash

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
REQUESTS="${REQUESTS:-20}"

SESSION_JSON="$(
  curl -fsS \
    -X POST \
    "$BASE_URL/api/session"
)"

SESSION_ID="$(
  printf '%s' "$SESSION_JSON" |
    jq -r '.session_id'
)"

KEY="concurrent-$(date +%s)"
RESULT_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "$RESULT_DIR"
}

trap cleanup EXIT

echo "Session: $SESSION_ID"
echo "Idempotency key: $KEY"
echo "Concurrent requests: $REQUESTS"
echo

for index in $(seq 1 "$REQUESTS"); do
  curl -fsS \
    -X POST \
    "$BASE_URL/api/collect" \
    -H 'Content-Type: application/json' \
    -H "Idempotency-Key: $KEY" \
    -d "{\"session_id\":\"$SESSION_ID\"}" \
    > "$RESULT_DIR/result-$index.json" &
done

wait

echo "Responses:"
jq -s '.' "$RESULT_DIR"/*.json

FIRST_EXECUTIONS="$(
  jq -s \
    '[.[] | select(.replayed == false)] | length' \
    "$RESULT_DIR"/*.json
)"

REPLAYS="$(
  jq -s \
    '[.[] | select(.replayed == true)] | length' \
    "$RESULT_DIR"/*.json
)"

DISTINCT_POINTS="$(
  jq -s \
    '[.[].points] | unique | length' \
    "$RESULT_DIR"/*.json
)"

STATE="$(
  curl -fsS \
    "$BASE_URL/api/state/$SESSION_ID"
)"

FINAL_POINTS="$(
  printf '%s' "$STATE" |
    jq -r '.points'
)"

echo
echo "First executions: $FIRST_EXECUTIONS"
echo "Replays: $REPLAYS"
echo "Distinct returned points values: $DISTINCT_POINTS"
echo "Final authoritative points: $FINAL_POINTS"

test "$FIRST_EXECUTIONS" -eq 1
test "$REPLAYS" -eq $((REQUESTS - 1))
test "$DISTINCT_POINTS" -eq 1
test "$FINAL_POINTS" -eq 1

echo
echo "PASS: $REQUESTS concurrent HTTP requests produced one logical increment."