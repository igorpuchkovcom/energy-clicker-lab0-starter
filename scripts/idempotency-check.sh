#!/usr/bin/env sh
set -eu

BASE_URL="${BASE_URL:-http://localhost:8080}"

SESSION_JSON="$(curl -fsS -X POST "$BASE_URL/api/session")"
SESSION_ID="$(printf '%s' "$SESSION_JSON" | python3 -c 'import json,sys; print(json.load(sys.stdin)["session_id"])')"
KEY="lost-response-$(date +%s)"

echo "Session: $SESSION_ID"
echo
echo "1) Send an idempotent collect. The DB commits, but curl times out before the delayed response."

set +e
curl --max-time 0.5 -sS -X POST "$BASE_URL/api/collect"   -H "Content-Type: application/json"   -H "Idempotency-Key: $KEY"   -H "X-Debug-Delay-After-Commit-Ms: 2000"   -d "{\"session_id\":\"$SESSION_ID\"}"
FIRST_EXIT=$?
set -e

echo
echo "curl exit code: $FIRST_EXIT (timeout is expected)"
sleep 2

echo
echo "2) Retry with the SAME Idempotency-Key. Expected: points=1 and replayed=true."
SECOND="$(curl -fsS -X POST "$BASE_URL/api/collect"   -H "Content-Type: application/json"   -H "Idempotency-Key: $KEY"   -d "{\"session_id\":\"$SESSION_ID\"}")"
echo "$SECOND"

printf '%s' "$SECOND" | python3 -c '
import json,sys
v=json.load(sys.stdin)
assert v["points"] == 1, v
assert v["replayed"] is True, v
print("PASS: one logical click produced exactly one increment.")
'
