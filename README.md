# Energy Clicker — MirrorQuest Lab 0

A small browser game that demonstrates a serious distributed-systems problem:

> The database commit succeeded, but the client never received the HTTP response. Is retrying safe?

This is the **starter** repository:

- session creation, state reads, PostgreSQL, health/readiness, graceful shutdown, and the deliberately unsafe endpoint already work;
- the safe PostgreSQL `Collect` transaction is intentionally left as a TODO.

## Stack

- Go 1.26
- standard-library `net/http`
- pgx v5
- PostgreSQL 18
- vanilla HTML/CSS/JavaScript
- Docker Compose

## Start

```bash
docker compose up --build
```

Open:

```text
http://localhost:8080
```

## API

### Create session

```http
POST /api/session
```

### Read state

```http
GET /api/state/{session_id}
```

### Safe collect

```http
POST /api/collect
Idempotency-Key: <unique client operation key>
Content-Type: application/json

{"session_id":"..."}
```

### Deliberately unsafe collect

```http
POST /api/debug/collect-unsafe
Content-Type: application/json

{"session_id":"..."}
```

### Health

```http
GET /healthz
GET /readyz
```

## Run the central experiment

```bash
make idempotency-check  # expected to fail until you implement Store.Collect
```

Read the complete workbook:

```text
docs/LAB_0_WORKBOOK.md
```

## Security note

Debug endpoints and artificial post-commit delays are enabled only for the local lab. They do not belong in a production service.
