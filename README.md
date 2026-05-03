# Cinema Reservation API

A Go + Redis cinema reservation system focused on preventing seat double booking
under high contention. The app exposes a versioned HTTP API, a small browser UI,
temporary seat holds, confirmation, release, health checks, and structured
request logging.

## Architecture

```text
cmd/
  main.go                         # dependency composition and HTTP server
internal/
  features/
    catalog/                      # film catalog feature
    reservations/                 # seat reservation domain + API + repository
  platform/
    cache/                        # Redis-backed cache abstraction
    config/                       # environment-based configuration
    web/                          # JSON responses and middleware
static/
  index.html                      # lightweight booking UI
```

The reservation feature depends on a `ClaimBook` interface. Redis is kept behind
the `platform/cache` module, so the business rules do not directly use Redis
client APIs.

## Key Behavior

- Atomic seat hold with Redis Lua script and `SET ... NX` semantics.
- Hold expiry using Redis TTL.
- Confirmed bookings are persisted without TTL.
- Wrong-customer confirm/release requests are rejected.
- Duplicate seat holds return `409 Conflict`.
- Request ID, access logging, recovery middleware, and readiness checks.

## Run Locally

Start Redis:

```bash
docker compose up -d cache
```

Run the API:

```bash
go run cmd/main.go
```

Open the UI:

```text
http://localhost:8080
```

Run everything with Docker:

```bash
docker compose up --build
```

## API

```text
GET    /healthz
GET    /readyz
GET    /api/v1/films
GET    /api/v1/films/{filmID}/seats
POST   /api/v1/films/{filmID}/seats/{seatCode}/claims
PUT    /api/v1/claims/{claimID}/confirm
DELETE /api/v1/claims/{claimID}
```

Create a temporary hold:

```bash
curl -X POST http://localhost:8080/api/v1/films/inception/seats/A1/claims \
  -H 'Content-Type: application/json' \
  -d '{"customer_id":"customer-1"}'
```

Confirm the claim:

```bash
curl -X PUT http://localhost:8080/api/v1/claims/<claim_id>/confirm \
  -H 'Content-Type: application/json' \
  -d '{"customer_id":"customer-1"}'
```

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `8080` | HTTP port |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_DB` | `0` | Redis DB index |
| `REDIS_POOL_SIZE` | `20` | Redis connection pool size |
| `HOLD_TTL` | `2m` | Temporary seat hold duration |
| `STATIC_DIR` | `static` | Static asset directory |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error` |

## Test

```bash
go test ./...
```
