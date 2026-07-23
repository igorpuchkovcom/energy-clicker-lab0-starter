# Lab 1 Workbook — MirrorScout

## Product

MirrorScout validates candidate mirror endpoints before they are allowed to
participate in routing or activation.

It is both:

- a CLI scanner;
- a synchronous HTTP API.

For every candidate it performs:

1. URL validation;
2. DNS lookup;
3. TCP connection;
4. `GET /healthz`;
5. `GET /readyz`;
6. scoring and classification.

## Concrete scenarios

The lab includes five local mock mirrors:

| Mirror | Port | Behaviour |
|---|---:|---|
| good | 18081 | health 200, readiness 200 |
| health500 | 18082 | health 500 |
| notready | 18083 | health 200, readiness 503 |
| hanging | 18084 | waits until timeout/cancellation |
| slow | 18085 | responds after a configurable delay |

The candidate file also includes a hostname that cannot be resolved.

## API contract

### CLI

```bash
go run ./cmd/mirrorscout scan \
  --file ./configs/mirrors.local.json \
  --concurrency 3 \
  --timeout 1s
```

### HTTP

```bash
go run ./cmd/mirrorscout serve \
  --addr :8090 \
  --concurrency 20 \
  --timeout 2s
```

```http
POST /api/scan
Content-Type: application/json

{
  "concurrency": 3,
  "timeout_ms": 1000,
  "candidates": [
    {"id":"good","url":"http://localhost:18081"}
  ]
}
```

## Result shape

```json
{
  "candidate_id": "good",
  "url": "http://localhost:18081",
  "duration_ms": 4,
  "score": 100,
  "healthy": true,
  "checks": {
    "dns": {"status":"passed","latency_ms":0},
    "tcp": {"status":"passed","latency_ms":0},
    "health": {"status":"passed","latency_ms":1},
    "readiness": {"status":"passed","latency_ms":1}
  }
}
```

## Scoring

| Signal | Points |
|---|---:|
| DNS passed | 15 |
| TCP passed | 20 |
| health passed | 25 |
| readiness passed | 30 |
| total latency ≤ 200 ms | 10 |
| total latency 201–500 ms | 5 |
| total latency > 500 ms | 0 |

A candidate is healthy only when all four checks pass and the score is at least 90.

This means latency influences ranking but a slow, functionally healthy candidate is
not automatically rejected.

---

# Milestone A — Start the mock world

Copy the starter overlay into the existing Energy Clicker repository.

```bash
chmod +x scripts/start-lab1-mocks.sh
chmod +x scripts/stop-lab1-mocks.sh
chmod +x scripts/lab1-smoke.sh
chmod +x scripts/generate-10000-candidates.py
```

Start the mirrors:

```bash
./scripts/start-lab1-mocks.sh
```

Verify manually:

```bash
curl -i http://localhost:18081/healthz
curl -i http://localhost:18081/readyz

curl -i http://localhost:18082/healthz
curl -i http://localhost:18083/readyz
```

The hanging mirror must not be called without a client timeout.

Stop all mocks:

```bash
./scripts/stop-lab1-mocks.sh
```

---

# Milestone B — Implement URL and DNS checks

Implement `scanCandidate` in:

```text
internal/mirrorscout/checks.go
```

Requirements:

- reject URLs without scheme or host;
- support `http` and `https`;
- use `net.ParseIP` for literal IP addresses;
- otherwise call `Resolver.LookupHost(ctx, hostname)`;
- measure latency;
- if DNS fails, mark TCP/health/readiness as skipped;
- do not replace the incoming context with `context.Background()`.

Exercise:

```bash
go test ./internal/mirrorscout -run TestScanCandidate
```

Questions:

- Why is DNS a separate check if `http.Client` resolves the host again?
- Should DNS failure block later checks?
- Which value belongs in `Detail`: all addresses, only one address, or none?

---

# Milestone C — Implement TCP and HTTP checks

TCP:

- derive default port 80 or 443;
- use `net.JoinHostPort`;
- call `Dialer.DialContext`;
- close successful connections immediately;
- skip HTTP checks after TCP failure.

HTTP:

- build request with `http.NewRequestWithContext`;
- use `/healthz` and `/readyz`;
- limit response body;
- require status 200;
- require readiness JSON status `ready`;
- always close response bodies.

Run:

```bash
./scripts/start-lab1-mocks.sh
go test ./internal/mirrorscout
```

---

# Milestone D — Timeout and cancellation

The entire candidate gets a timeout:

```go
ctx, cancel := context.WithTimeout(parent, timeout)
defer cancel()
```

The same derived context is passed to:

- DNS;
- TCP;
- health request;
- readiness request.

This gives one total candidate budget, not a fresh full timeout for every step.

Test:

```bash
time go run ./cmd/mirrorscout scan \
  --file ./configs/mirrors.local.json \
  --concurrency 1 \
  --timeout 1s
```

The hanging candidate must terminate around its deadline rather than after 30 seconds.

---

# Milestone E — Bounded worker pool

Implement:

```go
func (s *Scanner) ScanAll(
    ctx context.Context,
    candidates []Candidate,
    concurrency int,
) ([]Result, error)
```

Required properties:

- start at most `concurrency` workers;
- do not start one goroutine per candidate;
- stop queueing work after cancellation;
- active network calls receive the cancelled context;
- preserve input order in output;
- support 10,000 candidates;
- no channel send can remain blocked forever after cancellation.

Suggested structures:

```go
type indexedCandidate struct {
    index int
    candidate Candidate
}

type indexedResult struct {
    index int
    result Result
}
```

Do not protect a shared result slice by writing from workers as the first solution.
Use a result channel and a single collector.

---

# Milestone F — CLI

Run:

```bash
go run ./cmd/mirrorscout scan \
  --file ./configs/mirrors.local.json \
  --concurrency 3 \
  --timeout 1s
```

Expected classification:

- good: healthy;
- health500: health failed;
- notready: readiness failed;
- hanging: timeout;
- slow: depends on timeout;
- bad-dns: DNS failed, later checks skipped.

Send SIGTERM while a scan is running:

```bash
python3 scripts/generate-10000-candidates.py
go run ./cmd/mirrorscout scan \
  --file /tmp/mirrors-10000.json \
  --concurrency 20 \
  --timeout 2s
```

From another terminal:

```bash
pkill -TERM mirrorscout
```

When using `go run`, the visible process topology can complicate signal delivery. For
the explicit SIGTERM exercise, prefer:

```bash
go build -o /tmp/mirrorscout ./cmd/mirrorscout
/tmp/mirrorscout scan ...
```

---

# Milestone G — HTTP API

Start:

```bash
go run ./cmd/mirrorscout serve --addr :8090
```

Request:

```bash
jq -n \
  --slurpfile candidates configs/mirrors.local.json \
  '{concurrency:3, timeout_ms:1000, candidates:$candidates[0]}' |
curl -sS \
  -X POST http://localhost:8090/api/scan \
  -H 'Content-Type: application/json' \
  --data-binary @- |
jq
```

Cancellation exercise:

```bash
curl --max-time 0.2 ...
```

The request context should cancel the scan and its active network calls.

---

# Milestone H — 10,000 candidates

Generate:

```bash
python3 scripts/generate-10000-candidates.py
```

Run:

```bash
time go run ./cmd/mirrorscout scan \
  --file /tmp/mirrors-10000.json \
  --concurrency 20 \
  --timeout 2s \
  > /tmp/mirrors-10000-results.json
```

Observe:

```bash
ps -o pid,rss,vsz,cmd -C mirrorscout
```

Acceptance:

- worker concurrency remains bounded;
- output has 10,000 results;
- no goroutine-per-candidate design;
- no file descriptor exhaustion;
- cancellation stops work.

---

# Milestone I — Race detector

Run:

```bash
go test -race ./...
```

Intentional race exercise:

1. Temporarily let workers append directly to a shared `results` slice.
2. Run `go test -race`.
3. Observe the race report.
4. Restore the channel + single collector design.
5. Verify the detector is clean.

A passing race detector does not prove that the program has no races in paths that
were not executed. Tests must exercise the concurrent code.

---

# Definition of Done

- local mock mirrors run;
- DNS, TCP, health, readiness checks work;
- results include latency and score;
- hanging endpoint respects timeout;
- SIGTERM cancels an active CLI scan;
- HTTP client disconnect cancels an API scan;
- no more than configured workers are active;
- 10,000 candidates work with concurrency 20;
- output order is deterministic;
- `go test -race ./...` passes;
- ADR-002 is completed;
- failure cases are documented.

# Commit suggestion

```bash
git switch -c lab1-mirrorscout
git add .
git commit -m "Add MirrorScout candidate validation lab"
```
