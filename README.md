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

Go · [Fiber](https://gofiber.io) · [pgx](https://github.com/jackc/pgx) + raw SQL · [shopspring/decimal](https://github.com/shopspring/decimal) · [golang-migrate](https://github.com/golang-migrate/migrate) · [minio-go](https://github.com/minio/minio-go) · [go-playground/validator](https://github.com/go-playground/validator) · `golang.org/x/time/rate` · `errgroup` · [zap](https://github.com/uber-go/zap) · testify + testcontainers (tests).

## Project structure

```
cmd/api/              entrypoint (serve | -migrate | -health)
internal/
  app/                composition root: wiring + graceful shutdown
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
  pkg/                domain-agnostic helpers (apperr, validator, pagination)
db/migrations/        golang-migrate up/down SQL (embedded)
db/seeds/             sample data
test/integration/     testcontainers Postgres + MinIO tests (build tag: integration)
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
| `BODY_LIMIT_BYTES` | `6291456` | max request body size (6 MiB); must exceed `UPLOAD_LIMIT_BYTES` — Fiber has no per-route body limit, so the global ceiling is sized to admit proof uploads |
| `UPLOAD_LIMIT_BYTES` | `5242880` | max proof-file upload size (5 MiB) |
| `UPLOAD_ALLOWED_TYPES` | `image/jpeg,image/png,application/pdf` | allowed proof content-types, validated by magic-byte sniffing (not the client-declared header) |
| `POSTGRES_*` | see `.env.example` | host, port, user, password, db, sslmode, pool tuning, statement_timeout |
| `MINIO_*` | see `.env.example` | endpoint, access/secret key, bucket, ssl |
| `PRICE_STANDARD` | `50000` | auto-payment amount (IDR) for organic/plastic/paper |
| `PRICE_ELECTRONIC` | `100000` | auto-payment amount (IDR) for electronic |
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
{ "data": { ... }, "meta": { "page": 1, "per_page": 10, "total": 53 } }
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
6. Confirming a payment requires uploading a proof file to S3-compatible storage; the returned object URL is saved on the payment and is directly retrievable.

## Clean Code Architecture

The hexagonal split is not decoration — it is what keeps each kind of change confined to one place. This section explains how the pieces connect and, concretely, where the "cleanliness" shows up when the code is enhanced, extended, or swapped.

### Dependency injection (constructor wiring, no framework)

There is no DI container and no reflection. The single composition root, `internal/app/app.go` (`New`), is the only place where concrete types are constructed and handed to each other; every other file depends on an **interface**, never a concrete neighbour. Wiring proceeds outward-in, one feature at a time:

```go
pickupRepo    := postgres.NewPickupRepository(pool)               // adapter/out  (concrete)
pickupService := pickup.NewService(pickupRepo, paymentRepo,       // core/service (depends on port/out interfaces)
                                    txManager, clk, pricing, ttl)
pickupHandler := handler.NewPickupHandler(pickupService)          // adapter/in   (depends on port/in interface)
httpx.RegisterPickupRoutes(server.API(), pickupHandler, ...)
```

- `PickupHandler` holds a `in.PickupService` (a `core/port/in` interface), **not** the concrete `*pickup.Service`.
- `pickup.Service` holds `out.PickupRepository`, `out.PaymentRepository`, `out.TxManager`, `out.Clock` (all `core/port/out` interfaces), **not** the pgx-backed structs.
- Shared infrastructure (`pool`, `txManager`, `clk`, `limiter`, MinIO `store`) is built once and injected into everything that needs it.

Because the only place a concrete type meets an interface is this one function, the inward dependency rule is enforced by construction: the compiler will not let a handler import a repository or the core import Fiber/pgx.

### How the DTO connects `adapter/in`, `port/in`, and `domain`

Three struct families meet at the HTTP boundary, each owning exactly one concern:

| Layer | Type (example) | Shape & responsibility |
|---|---|---|
| `adapter/in/http/dto` | `CreatePickupRequest{HouseholdID string; Type string; SafetyCheck *bool}` | **Wire shape.** JSON tags, `validate:` tags, `Sanitize()`. Strings, nothing trusted yet. |
| `core/port/in` | `CreatePickupCommand{HouseholdID uuid.UUID; Type domain.PickupType; SafetyCheck bool}` | **The contract.** Domain vocabulary only — typed, validated, framework-free. |
| `core/domain` | `domain.Pickup` | **The entity.** Pure types, enums, invariants. No JSON/DB tags, never crosses the wire directly. |

The handler is the translator. Inbound: `dto.CreatePickupRequest` → (bind, sanitize, validate, `uuid.Parse`) → `in.CreatePickupCommand`. Outbound: `domain.Pickup` → `dto.NewPickupResponse(...)` → `dto.PickupResponse` (e.g. `amount` rendered as a fixed-2 string). So the DTO is the HTTP-specific *projection* on either side of the core; the `port/in` command/result types are the stable contract the core actually speaks. The domain never learns it is being served over HTTP, and the wire format can change without touching a business rule.

### Request → response flow

Tracing `POST /api/pickups` end to end:

```
HTTP request
  → middleware: request-id → access-log → recover → timeout → rate-limit   (adapter/in/http/middleware)
  → PickupHandler.Create                                                    (adapter/in/http/handler)
      c.BodyParser → dto.CreatePickupRequest   (bad JSON → 400 VALIDATION_ERROR)
      req.Sanitize(); validator.Struct(req)     (required / oneof / required_if → 400 + field details)
      uuid.Parse(req.HouseholdID)               (non-UUID → 400, never handed raw to a repo)
      build in.CreatePickupCommand
  → h.svc.Create(c.UserContext(), cmd)          via the in.PickupService interface; ctx carries the deadline
  → pickup.Service.Create                                                   (core/service)
      tx.Do(ctx, fn):                            Rule 1 check + insert in ONE transaction
        payments.HasPendingByHousehold(...)      via out.PaymentRepository interface
        pickups.Insert(...)                      via out.PickupRepository interface
  → postgres adapter                                                        (adapter/out/postgres)
      resolves executor from ctx (tx vs pool), runs parameterized SQL,
      maps rows → domain.Pickup, maps no-rows/constraint → domain error
  ← returns domain.Pickup, or a typed domain error
  → presenter.Success(c, 201, dto.NewPickupResponse(pickup))  → { "data": { ... } }
    presenter.Error(c, svcErr): apperr.From maps domain error → HTTP status + { "error": { ... } }
       client errors carry code+details; server errors return a generic message (no SQL/driver text leaks)
```

At no point does a `*fiber.Ctx` enter the core, and at no point does a `pgx` type leave the Postgres adapter. The deadline set by the timeout middleware rides `c.UserContext()` straight through the service to pgx, which abandons the query if the client goes away.

### Where the "cleanliness" pays off when things change

The value of the structure is the **blast radius** of a change — each kind of edit is confined to one layer, and the compiler proves the rest still fits.

- **Swap a driven dependency** (pgx → another SQL driver, MinIO → real AWS S3): rewrite only the adapter in `adapter/out/*` so it still satisfies the same `port/out` interface, then change one constructor line in `app.go`. The service, domain, handlers, DTOs, and every service unit test are untouched — they depend on `out.FileStorage` / `out.PickupRepository`, not the implementation.
- **Swap the web framework** (Fiber → Gin / net/http): rewrite only `adapter/in/http/*` (handlers, router, middleware, presenter). The `port/in` commands and the services don't move, because `*fiber.Ctx` never crossed the boundary.
- **Change a response's shape** (add/rename/format a field): edit the `dto.XResponse` struct and its `NewXResponse` mapper — one file. Persistence and rules are unaffected unless genuinely new data is required.
- **Add a business rule or endpoint**: add a method to the `port/in` interface, implement it as a new `service/<feature>/<op>.go` file (one file per operation), add a handler method + a route line. If the interface and the implementation drift, the build fails — the contract is compiler-checked, not convention-checked.
- **Test without infrastructure**: services are unit-tested against `mockery` mocks of the `port/out` interfaces plus a `FixedClock` and a `PassthroughTx` — every rule's happy and violation paths run with zero Docker. Only the hand-written SQL needs real containers (the `integration`-tagged suite).
- **Change a policy value** — exactly the recent Rule 5 pricing fix: amounts are a `config` field flowing through `domain.Pricing` into the service by constructor injection. Correcting 50k/100k changed `config` and the test fixtures; the mapping (`Pricing.AmountFor`), the service, the handler, and the SQL never moved. A pure policy change had a one-layer blast radius — which is the whole point.

## Design assumptions & trade-offs

Where the brief was silent or a robustness decision was made, the call and its rationale are recorded here.

**Business contract**

- **Payment amounts (Rule 5)** follow the brief: IDR 50,000 (organic/plastic/paper) and IDR 100,000 (electronic). They are configurable via `PRICE_STANDARD` / `PRICE_ELECTRONIC`. Currency is IDR; amounts are `numeric(14,2)`, never float.
- **`POST /api/payments`** manually creates a `pending` payment for an existing pickup (`{household_id, waste_id}`); the amount is **server-derived** from the pickup's type, never client-supplied. One payment per pickup is enforced by `UNIQUE(waste_id)` → `409 PAYMENT_ALREADY_EXISTS`. Any pickup status is allowed (not just `completed`); a `pending` payment then blocks new pickups for that household via Rule 1 — an accepted consequence.
- **"Not picked up" (Rule 4)** means an organic pickup still `pending` or `scheduled` (not `completed`/`canceled`) older than `ORGANIC_TTL`. The sweep is a single guarded bulk `UPDATE` whose `WHERE` excludes terminal statuses, so it cannot race a concurrent user `complete`/`cancel`.
- **Payment list date range** filters on `payment_date`; `pending` payments (null `payment_date`) are excluded from a dated window.
- **Reporting**: `total_revenue` sums only `paid` amounts (money actually collected). Summary rows are present-combinations only (a type/status with zero rows is absent). `households/{id}/history` returns the household's **full** pickup + payment history unpaginated, per the brief's wording — bounded in query *time* by `statement_timeout`, but unbounded in response *size*; pagination or a signalled sanity ceiling is the hardening path if "full" should be capped (see Future work).
- **Deleting a household** with existing pickups/payments is refused with `409` (`ON DELETE RESTRICT`, no cascade).
- **Past `pickup_date`** values are allowed (the brief states no future-only rule).

**Robustness**

- **Proof upload content-type** is validated by **magic-byte sniffing** (`http.DetectContentType` on the first 512 bytes), not the client-declared multipart header — a spoofed `image/png` over HTML is rejected.
- **Proof retrievability**: the `proofs` bucket is set to a public-read (`s3:GetObject`) policy at startup so the stored `proof_file_url` is directly dereferenceable, without expiry and without credentials in the URL. Object keys are double-UUID (`payments/{paymentID}/{randomID}.ext`) and unguessable. For deployments that must keep proofs non-public, swap the policy for presigned GET URLs or an authenticated download proxy (the storage port already isolates this behind `FileStorage.Put`).
- **Upload size vs. body limit**: Fiber v2 has no per-route body limit, so the global `BODY_LIMIT_BYTES` (6 MiB) is sized above `UPLOAD_LIMIT_BYTES` (5 MiB); `config.Validate()` enforces `UPLOAD < BODY` in every environment. JSON routes inherit the 6 MiB ceiling.
- **Orphaned proof on race**: if two confirms race, the loser uploads its object but the guarded `UPDATE` matches 0 rows, leaving an unreferenced object. The database stays correct (the URL is only ever set by the same `UPDATE` that flips status); cleanup via a best-effort `Delete` or a sweep is noted as future work.
- **Unclassified DB errors** wrap to a generic `500 INTERNAL_ERROR` with the cause kept for logs only — no driver text, SQL, or table names reach the client.

**Scope**

- **No authorization** — the brief specifies none, so none is invented. Endpoints that return per-household PII/financials (notably `households/{id}/history`) would need an ownership/role check once auth is in scope.
- **UUID `validate:` tags are omitted** on DTOs; the handler's `uuid.Parse` is the single validation point for every `:id` (`400 VALIDATION_ERROR` on a non-UUID).

## Testing

```bash
make test               # unit tests, race detector on
make test-integration   # postgres + minio via testcontainers (Docker required)
```

Service-layer use cases are unit-tested against mocked outbound ports, with the HTTP edge (binding, validation, envelope, error mapping) covered by `httptest`. The Postgres adapter's hand-written SQL, the guarded transitions, TxManager atomicity, and the MinIO proof upload are verified against real ephemeral containers via testcontainers. Clean worker shutdown is proven by a deterministic context-cancellation test (the scheduler returns once its context is canceled).

## Production-readiness notes

- **Graceful shutdown**: `SIGINT`/`SIGTERM` drains in-flight HTTP requests, stops the worker, and closes the pool, bounded by `SHUTDOWN_TIMEOUT`.
- **Dependency injection**: explicit constructor wiring in `internal/app`; no DI framework.
- **Rate limiting**: per-IP token bucket on pickup creation, returning `429`.
- **Timeouts**: per-request deadline plus a Postgres `statement_timeout` backstop.
- **Observability**: structured zap logging, a request-ID per request (`X-Request-ID`), and split liveness/readiness probes.
- **Resilience**: dependency-unavailable conditions map to `503` and flip readiness, never a `500`.
- **Container**: multi-stage build to a non-root distroless image.

## Future work

- Pagination (or a signalled sanity ceiling) on `households/{id}/history` if its result should be bounded.
- Ownership/role checks on the report endpoints once authentication is in scope.
- Best-effort cleanup of orphaned proof objects on a confirm race, and sweep-failure alerting (a Prometheus counter on the worker).
- `Idempotency-Key` support for create endpoints.
- Prometheus `/metrics`.
- Distributed (Redis-backed) rate limiting for multi-instance deployments.
