# ADR-002: Bounded worker pool for mirror validation

## Status

Proposed

## Context

MirrorScout may receive thousands of candidate mirrors. Starting one goroutine and
several network operations per candidate without a bound can create excessive
memory use, open sockets, DNS pressure, and load against downstream systems.

## Decision

Use a fixed-size worker pool. The configured concurrency limits the number of
candidate scans active at one time.

Each candidate scan receives its own timeout derived from the parent context.
SIGTERM, HTTP client cancellation, or an upstream deadline cancels outstanding work.

## Consequences

### Positive

- predictable concurrency;
- bounded pressure on DNS, TCP, and HTTP dependencies;
- one mechanism for CLI and HTTP execution;
- cancellation propagates through queued and active work.

### Negative

- queued candidates add latency;
- one concurrency value may not be optimal for every environment;
- slow candidates can occupy workers until timeout;
- operational metrics are required to tune the pool.

## Alternatives

- one goroutine per candidate;
- semaphore around unbounded goroutines;
- separate pools for DNS, TCP, and HTTP checks;
- external job queue.
