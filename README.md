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

## Development with Cursor Dev Container

### Prerequisites

- Docker Desktop (running)
- Cursor (or VS Code) with Dev Containers support

### Open the project in the container

1. Open this folder in Cursor.
2. Run the command **Dev Containers: Reopen in Container** (Ctrl+Shift+P).
3. Wait for the first build: the container installs Go tools and runs
   `go mod download` and `go test ./...` automatically (`postCreateCommand`).

The Dev Container starts only the `db` service (PostgreSQL). The production
`app` service is **not** started, so port 8080 stays free for `go run`.

### Networking

- Inside the Dev Container PostgreSQL is available as `db:5432`
  (`DATABASE_URL` is already set to
  `postgres://energy:energy@db:5432/energy?sslmode=disable`).
- From the Windows host the same database is `localhost:5432`
  (the port is published by `compose.yaml`).

### Wait for PostgreSQL

The `db` service has a healthcheck, and the Dev Container waits for it before
starting. To verify manually from a container terminal:

```bash
pg_isready -h db -U energy -d energy
# or open a SQL shell:
psql "$DATABASE_URL"
```

### Run the API

```bash
go run ./cmd/server
```

Then open `http://localhost:8080` in a browser on the Windows host
(port 8080 is forwarded automatically). You can also use the task
**Lab 0: Run API** or the debugger configuration **Debug API (cmd/server)**.

### Run tests

```bash
go test ./...
go test -race ./...
```

Or the tasks **Lab 0: Test** / **Lab 0: Test Race**.

### Run the idempotency experiment

With the API running (`go run ./cmd/server` in another terminal):

```bash
./scripts/idempotency-check.sh
```

It is expected to fail in the starter until you implement `Store.Collect`.

### Rebuild the container

After changing anything under `.devcontainer/`, run the command
**Dev Containers: Rebuild Container** (Ctrl+Shift+P).

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
