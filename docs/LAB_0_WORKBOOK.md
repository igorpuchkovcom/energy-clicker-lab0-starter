# Lab 0 Workbook — Energy Clicker

## Learning outcomes

By the end of this lab you can explain and demonstrate:

- process liveness versus dependency readiness;
- PostgreSQL as the authoritative state store;
- why a successful side effect and a successful HTTP response are different events;
- why blindly retrying a non-idempotent POST can duplicate a business effect;
- how an idempotency key turns retries into replays;
- why the idempotency record and the state change belong in one transaction;
- how `SELECT ... FOR UPDATE` serializes state changes for one session;
- how Go performs graceful HTTP shutdown.

## Time box

- Milestone A: 30–45 minutes
- Milestone B: 45 minutes
- Milestone C: 60–90 minutes
- Milestone D: 45–60 minutes
- Milestone E: 30 minutes
- Reflection and ADR: 30 minutes

Total: roughly 4–5 hours.

---

## Milestone A — Run the product

```bash
docker compose up --build
```

Open:

```text
http://localhost:8080
```

Verify:

```bash
curl -i http://localhost:8080/healthz
curl -i http://localhost:8080/readyz
```

Expected:

- `/healthz` says the process is alive;
- `/readyz` checks PostgreSQL and shutdown state.

Create a session and collect a few points from the browser.

### Questions

1. Why should `/healthz` not query every dependency?
2. Why should `/readyz` fail when PostgreSQL cannot be reached?
3. What should a load balancer do with a process that is alive but not ready?

---

## Milestone B — Reproduce the unsafe retry bug

1. Create a new session.
2. Enable “Lost-response experiment”.
3. Press **Collect unsafely**.
4. The browser aborts while the server is delaying its response after the DB commit.
5. Press **Retry unsafe request**.
6. Refresh authoritative state.

Expected: one user intention can produce two points.

### What happened?

```text
Client                   API                    PostgreSQL
  | POST collect          |                         |
  |---------------------->| UPDATE points + 1       |
  |                       |------------------------>|
  |                       | COMMIT                  |
  |                       |<------------------------|
  | X connection aborted |                         |
  |                       | response cannot arrive  |
  | POST retry            |                         |
  |---------------------->| UPDATE points + 1       |
```

The client cannot tell whether the first request:

- never reached the server;
- failed before commit;
- committed but lost its response.

Blind retry is therefore unsafe.

---

## Milestone C — Define the idempotency contract

The final endpoint is:

```http
POST /api/collect
Idempotency-Key: <client-generated-key>
Content-Type: application/json

{"session_id":"..."}
```

Required semantics:

1. The first `(session_id, key)` increments by exactly one.
2. A retry with the same pair returns the original `points_after`.
3. The retry must not increment again.
4. The same key may be used by another session without conflict.
5. Two concurrent requests with the same pair must have one logical effect.
6. The state mutation and idempotency record are committed atomically.

Database constraint:

```sql
PRIMARY KEY (session_id, idempotency_key)
```

### Design decision

This lab uses a row lock:

```sql
SELECT points
FROM game_sessions
WHERE id = $1
FOR UPDATE;
```

Then, in the same transaction:

- check the idempotency record;
- replay if it exists;
- otherwise increment points;
- insert the response record;
- commit.

This is intentionally simple and correct. It serializes all collects for one session, which is a throughput trade-off to revisit in later labs.

---

## Milestone D — Implement safe collect

In the starter repository, implement:

```go
func (s *Store) Collect(
    ctx context.Context,
    sessionID string,
    idempotencyKey string,
) (points int64, replayed bool, err error)
```

Checklist:

- begin transaction;
- `SELECT ... FOR UPDATE`;
- map missing session to `store.ErrNotFound`;
- query `collect_requests`;
- if found, return `points_after` with `replayed=true`;
- otherwise increment;
- insert idempotency record;
- commit;
- never commit partial state.

Then run:

```bash
make idempotency-check
```

Expected:

```text
PASS: one logical click produced exactly one increment.
```

---

## Milestone E — Verify concurrency

Run two requests concurrently with the same key.

Example:

```bash
KEY="same-key"
BODY='{"session_id":"YOUR_SESSION_ID"}'

curl -sS -X POST http://localhost:8080/api/collect   -H 'Content-Type: application/json'   -H "Idempotency-Key: $KEY"   -d "$BODY" &

curl -sS -X POST http://localhost:8080/api/collect   -H 'Content-Type: application/json'   -H "Idempotency-Key: $KEY"   -d "$BODY" &

wait
```

Expected:

- both responses show the same points value;
- one has `replayed=false`;
- the other has `replayed=true`;
- state increased only once.

---

## Graceful shutdown experiment

Follow logs:

```bash
docker compose logs -f app
```

In another terminal:

```bash
docker compose stop app
```

Observe:

1. SIGTERM is converted into context cancellation.
2. Server marks itself not ready.
3. New listeners are closed.
4. Active HTTP requests receive time to finish.
5. PostgreSQL pool closes after HTTP shutdown.

To make the behavior visible, start a delayed request before stopping:

```bash
curl -X POST http://localhost:8080/api/collect   -H 'Content-Type: application/json'   -H 'Idempotency-Key: shutdown-demo'   -H 'X-Debug-Delay-After-Commit-Ms: 5000'   -d '{"session_id":"YOUR_SESSION_ID"}'
```

Then stop the container while it is waiting.

---

## Tests

```bash
make test
make test-race
make vet
```

Add an integration test for these cases:

| Case | Expected |
|---|---|
| first key use | points +1, replayed false |
| same key retry | same points, replayed true |
| two concurrent same-key requests | one increment |
| two different keys | two increments |
| missing session | 404 / ErrNotFound |
| empty key | 400 |
| key longer than 200 | 400 |

---

## ADR assignment

Create:

```text
docs/adr/ADR-001-idempotent-collect.md
```

Use this structure:

```markdown
# ADR-001: Idempotency for collect operations

## Status
Accepted

## Context
The client can lose a response after the state change commits.

## Decision
Require a client-generated Idempotency-Key scoped to a session.
Store points_after in the same transaction as the increment.

## Consequences
Positive:
- safe retries;
- deterministic replay;
- auditable request identity.

Negative:
- storage growth;
- key-retention policy is needed;
- row locking serializes writes for one session.

## Alternatives
- no retry;
- server-generated request ID;
- optimistic version;
- globally unique event log.
```

---

## Definition of Done

- Browser game works.
- PostgreSQL survives app restarts.
- Unsafe lost-response retry is reproducible.
- Safe retry changes state once.
- Concurrent duplicate requests are safe.
- `/healthz` and `/readyz` have different semantics.
- SIGTERM triggers graceful shutdown.
- Tests and race detector pass.
- ADR-001 is written.
