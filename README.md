# Community Waste Collection API

A Go backend for a community waste-collection service: households request waste pickups, pickups move through a lifecycle, and payments are raised and confirmed for collection. Built with a hexagonal (ports & adapters) architecture, Fiber, PostgreSQL (pgx + raw SQL), and S3-compatible object storage (MinIO).

## Architecture

Ports & adapters. A framework-free core surrounded by adapters; the dependency arrow points inward only.

```
adapter/in/{http,worker}  ──▶  core/port/in  ──▶  core/service  ──▶  core/port/out  ◀──  adapter/out/{postgres,storage,clock}
                                                       │
                                                       └──▶  core/domain   (pure entities, enums, invariants, errors)
```

- **`core/`** holds the domain and the use cases. It imports no framework or driver — no Fiber, no pgx, no MinIO. Business rules live here and are unit-tested with mocked outbound ports.
- **`adapter/in/`** are driving adapters: the HTTP server (Fiber) and the background worker. `*fiber.Ctx` never leaves the HTTP adapter.
- **`adapter/out/`** are driven adapters: the Postgres repositories (hand-written SQL via pgx), the MinIO file storage, and the clock. `pgx` types never leave the Postgres adapter.

Three decisions carry the most weight and shape the code throughout: the hexagonal split itself; transaction management via a `TxManager` port + context, which keeps the core free of any database driver; and conditional-`UPDATE` concurrency control for the pickup state machine, where the source status is the `WHERE` guard so a lost race fails safely instead of corrupting state.

## Tech stack

Go · [Fiber](https://gofiber.io) · [pgx](https://github.com/jackc/pgx) + raw SQL · [shopspring/decimal](https://github.com/shopspring/decimal) · [golang-migrate](https://github.com/golang-migrate/migrate) · [minio-go](https://github.com/minio/minio-go) · [go-playground/validator](https://github.com/go-playground/validator) · `golang.org/x/time/rate` · `errgroup` · [zap](https://github.com/uber-go/zap) · testify + testcontainers + goleak (tests).

## Project structure

```
cmd/api/            entrypoint (serve | -migrate | -health)
internal/app/       composition root: wiring + graceful shutdown
core/
  domain/           entities, enums, invariants, error catalog
  port/in,out/      driving and driven interfaces
  service/          use cases (added per feature)
adapter/
  in/http/          handlers, dto, middleware, presenter, router
  in/worker/        ticker scheduler (organic auto-cancel)
  out/postgres/     pgxpool, TxManager, repositories
  out/storage/      MinIO FileStorage
  out/clock/        system clock
config/             typed env config + production-safety gates
db/migrations/      golang-migrate up/down SQL (embedded)
db/seeds/           sample data
test/integration/   testcontainers + replayable HTTP tests
```

## Getting started

### Prerequisites

- Docker + Docker Compose (the only requirement to run the full stack).
- Go 1.26+ (only if running outside Docker).

### Run everything with one command

```bash
make up          # docker compose up --build -d
```

This starts PostgreSQL and MinIO, runs database migrations to completion, then starts the API. Ordering is enforced by health checks: the app does not start until migrations have applied and dependencies are healthy.

- API: http://localhost:8080
- Liveness: `GET /healthz` · Readiness: `GET /readyz`
- MinIO console: http://localhost:9001 (`minioadmin` / `minioadmin`)

```bash
make logs        # follow logs
make seed        # load sample households/pickups
make down        # stop and remove volumes
```

### Run locally (outside Docker)

```bash
cp .env.example .env       # adjust as needed; point POSTGRES_HOST/MINIO_ENDPOINT at local services
make migrate-up
make run
```

## Configuration

All configuration is via environment variables (see `.env.example`). `config.Validate()` refuses to start when `APP_ENV=production` and any of `POSTGRES_SSLMODE=disable`, `CORS_ORIGINS=*`, or a required secret is empty.

| Variable | Default | Description |
|---|---|---|
| `APP_ENV` | `local` | `local` / `development` / `staging` / `production` |
| `APP_PORT` | `8080` | HTTP port |
| `LOG_LEVEL` | `info` | zap log level |
| `CORS_ORIGINS` | `*` | allowed origins (must not be `*` in production) |
| `REQUEST_TIMEOUT` | `5s` | per-request deadline |
| `SHUTDOWN_TIMEOUT` | `10s` | graceful-shutdown budget |
| `BODY_LIMIT_BYTES` | `1048576` | max JSON body size |
| `UPLOAD_LIMIT_BYTES` | `5242880` | max proof-file upload size |
| `POSTGRES_*` | see `.env.example` | host, port, user, password, db, sslmode, pool tuning, statement_timeout |
| `MINIO_*` | see `.env.example` | endpoint, access/secret key, bucket, ssl |
| `PRICE_STANDARD` | `10000` | auto-payment amount (IDR) for organic/plastic/paper |
| `PRICE_ELECTRONIC` | `50000` | auto-payment amount (IDR) for electronic |
| `SWEEP_INTERVAL` | `1m` | organic auto-cancel sweep frequency |
| `ORGANIC_TTL` | `72h` | age after which an unpicked organic pickup auto-cancels |
| `PICKUP_RATE_RPM` / `PICKUP_RATE_BURST` | `30` / `10` | per-IP rate limit on pickup creation |

## Migrations & seeding

Migrations are versioned SQL under `db/migrations/`, embedded into the binary and applied by `cmd/api -migrate up` (run as a one-shot `migrate` service in Compose).

```bash
make migrate-up                      # apply
make migrate-down                    # roll back
make migrate-create NAME=add_x       # new migration pair (requires the migrate CLI)
make seed                            # load db/seeds into the running db container
```

## API

All responses use a consistent envelope.

Success:
```json
{ "data": { ... }, "meta": { "page": 1, "per_page": 20, "total": 53 } }
```
Error:
```json
{ "error": { "code": "PICKUP_NOT_PENDING", "message": "pickup must be pending for this action",
             "details": [ { "field": "status", "reason": "must be pending" } ] } }
```

| Method | Path | Description |
|---|---|---|
| POST | `/api/households` | Create a household |
| GET | `/api/households` | List households (paginated) |
| GET | `/api/households/{id}` | Household detail |
| DELETE | `/api/households/{id}` | Delete a household |
| POST | `/api/pickups` | Create a pickup (rate-limited) |
| GET | `/api/pickups` | List pickups (filter by status, household_id) |
| PUT | `/api/pickups/{id}/schedule` | Schedule a pickup |
| PUT | `/api/pickups/{id}/complete` | Complete a pickup (auto-creates a payment) |
| PUT | `/api/pickups/{id}/cancel` | Cancel a pickup |
| POST | `/api/payments` | Create a payment |
| GET | `/api/payments` | List payments (filter by status, household, date range) |
| PUT | `/api/payments/{id}/confirm` | Confirm a payment (multipart proof upload) |
| GET | `/api/reports/waste-summary` | Pickups aggregated by type and status |
| GET | `/api/reports/payment-summary` | Totals by status and total revenue |
| GET | `/api/reports/households/{id}/history` | Full pickup and payment history |

A Postman collection (`wst-backend.postman_collection.json`) is included at the repo root.

## Business rules

1. A household cannot create a new pickup if it has any payment with `pending` status.
2. A pickup can only be scheduled if its current status is `pending`.
3. Electronic pickups cannot be scheduled unless `safety_check` is true.
4. Organic pickups auto-cancel if not picked up within `ORGANIC_TTL` (default 3 days) — run by a background goroutine that shuts down cleanly on exit.
5. Completing a pickup auto-creates a `pending` payment (in the same transaction) priced by type.
6. Confirming a payment requires uploading a proof file to S3-compatible storage; the URL is saved on the payment.

## Assumptions

- **Payment amounts (Rule 5)** were blank in the brief. They are configurable (`PRICE_STANDARD`, `PRICE_ELECTRONIC`) and default to IDR 10,000 (organic/plastic/paper) and IDR 50,000 (electronic).
- **"Not picked up" (Rule 4)** means an organic pickup still `pending` or `scheduled` (not `completed`/`canceled`) older than the TTL.
- **Deleting a household** with existing pickups/payments is refused with `409` (no cascade).
- Currency is IDR; amounts are stored as `numeric(14,2)`.

## Testing

```bash
make test               # unit tests, race detector on
make test-integration   # postgres + minio via testcontainers (Docker required)
```

Service-layer use cases are unit-tested against mocked outbound ports. The Postgres adapter's hand-written SQL is verified against a real ephemeral database via testcontainers. Clean shutdown is asserted with `goleak`.

## Production-readiness notes

- **Graceful shutdown**: `SIGINT`/`SIGTERM` drains in-flight HTTP requests, stops the worker, and closes the pool, bounded by `SHUTDOWN_TIMEOUT`.
- **Dependency injection**: explicit constructor wiring in `internal/app`; no DI framework.
- **Rate limiting**: per-IP token bucket on pickup creation, returning `429`.
- **Timeouts**: per-request deadline plus a Postgres `statement_timeout` backstop.
- **Observability**: structured zap logging, a request-ID per request (`X-Request-ID`), and split liveness/readiness probes.
- **Resilience**: dependency-unavailable conditions map to `503` and flip readiness, never a `500`.
- **Container**: multi-stage build to a non-root distroless image.

## Future work

- `Idempotency-Key` support for create endpoints.
- Prometheus `/metrics`.
- Distributed (Redis-backed) rate limiting for multi-instance deployments.
