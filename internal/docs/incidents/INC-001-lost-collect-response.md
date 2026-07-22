# INC-001: Lost HTTP response caused duplicate energy collection

## Scenario

A client sent an unsafe collect request.

The server committed the PostgreSQL update but delayed the HTTP
response. The client timed out before receiving the response.

The client then retried the request.

## Observed result

Initial points: 0

After the first request timed out:
points = 1

After the retry:
points = 2

## Root cause

The collect operation was non-idempotent.

The client could not distinguish between:

- request not delivered;
- transaction failed;
- transaction committed but response lost.

The retry executed the state mutation a second time.

## Required property

A retry of the same logical collect operation must return the
original result without applying another increment.

## Candidate mechanism

Client-generated Idempotency-Key scoped to the session.