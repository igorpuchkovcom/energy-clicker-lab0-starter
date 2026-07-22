#!/usr/bin/env sh
set -eu

BASE_URL="${BASE_URL:-http://localhost:8080}"

echo "Health:"
curl -fsS "$BASE_URL/healthz"
echo

echo "Readiness:"
curl -fsS "$BASE_URL/readyz"
echo

SESSION_JSON="$(curl -fsS -X POST "$BASE_URL/api/session")"
SESSION_ID="$(printf '%s' "$SESSION_JSON" | python3 -c 'import json,sys; print(json.load(sys.stdin)["session_id"])')"

echo "Created session: $SESSION_ID"
curl -fsS "$BASE_URL/api/state/$SESSION_ID"
echo

KEY="demo-$(date +%s)"
curl -fsS -X POST "$BASE_URL/api/collect"   -H "Content-Type: application/json"   -H "Idempotency-Key: $KEY"   -d "{\"session_id\":\"$SESSION_ID\"}"
echo

echo "Replay the same request:"
curl -fsS -X POST "$BASE_URL/api/collect"   -H "Content-Type: application/json"   -H "Idempotency-Key: $KEY"   -d "{\"session_id\":\"$SESSION_ID\"}"
echo
