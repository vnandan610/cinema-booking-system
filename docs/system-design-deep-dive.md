# Cinema Reservation System: Engineering and System Design Deep Dive

Written from the point of view of the engineer who built and refactored this project.

This document explains the project from first principles. It starts with beginner-friendly ideas like "what is a backend?" and gradually moves into design trade-offs, load limits, Redis behavior, failure scenarios, Docker, scalability, and interview preparation.

The goal is not just to describe the code. The goal is to understand what this system can actually do, where it will perform well, where it will struggle, and what I would improve before treating it like a production-grade ticket booking platform.

---

## Table of Contents

1. Introduction
2. Fundamental Concepts from Zero
3. Project Overview
4. System Capabilities
5. System Limits and Breaking Points
6. Folder Structure Explained
7. Core Workflow Step by Step
8. Code Explanation
9. Redis in This Project
10. Docker Explained Practically
11. Design Decisions and Trade-offs
12. Alternative Architectures
13. Failure Scenarios and Edge Cases
14. Scalability and Future Improvements
15. How to Run the Project
16. Interview Readiness
17. Final Engineering Assessment

---

## 1. Introduction

### What This Project Is

This project is a cinema seat reservation system written in Go. It lets a user:

- View available films.
- View seat status for a film.
- Temporarily hold a seat.
- Confirm the held seat.
- Release a held seat before confirmation.
- Prevent two users from reserving the same seat at the same time.

The important part is not the movie list. The important part is the concurrency problem.

Imagine two users open the same film, both see seat `A1` as available, and both click the seat at nearly the exact same time. A naive system might accidentally allow both users to reserve `A1`. In a real cinema, that means two people arrive with the same seat assignment. That is a business failure, not just a technical bug.

This project solves that problem by using Redis as a fast shared coordination layer. Redis helps the application make a seat claim atomically, meaning "do this as one indivisible operation." Either one user wins the seat, or they do not. There should not be a middle state where two users both believe they won.

### Real-World Problem It Solves

The general problem is not limited to cinemas. The same pattern appears in many systems:

- Booking a movie seat.
- Reserving a concert ticket.
- Ordering the last item in inventory.
- Booking a cab.
- Reserving a hotel room.
- Claiming a limited coupon.
- Registering for a course with limited seats.

In all of these cases, the business rule is simple:

> When availability is limited, two users must not be allowed to claim the same unique resource.

The technical challenge is that modern applications run with many users at the same time. Requests can arrive concurrently. Multiple backend instances may be running. Network delays can reorder events. The system must be correct even when requests overlap.

### Simple Analogy for Beginners

Think of a cinema seat as a chair in a room.

If only one person is entering the room at a time, booking is easy. You can ask, "Is chair A1 empty?" If yes, you put a sticky note on it.

But now imagine 100 people run into the room at once. Everyone wants chair A1. If they all look at the chair before any sticky note is placed, they all see it as empty. Without coordination, many people may try to claim it.

Redis acts like a strict gatekeeper standing beside the chair. The gatekeeper says:

> I will let exactly one person place the sticky note. Everyone else gets rejected immediately.

That is the heart of this project.

### What This Project Is Not

This is not a complete cinema business system. It does not include:

- User authentication.
- Real payment integration.
- Admin dashboards.
- Real movie scheduling.
- Seat pricing.
- Persistent relational database storage.
- Email/SMS notifications.
- Multi-region deployment.
- Auditing and financial reconciliation.

It is a focused backend system for the seat reservation problem.

That is a good scope for an SDE1 portfolio project because it demonstrates:

- API design.
- Concurrency control.
- Redis usage.
- Error handling.
- Middleware.
- Docker deployment.
- System design thinking.

---

## 2. Fundamental Concepts from Zero

This section assumes very basic coding knowledge.

### What Is a Backend?

A backend is the part of an application that runs on a server and handles the real work behind the scenes.

For example, when you use a food delivery app, the screen on your phone is the frontend. The backend is the system that:

- Stores restaurants.
- Stores menus.
- Receives orders.
- Calculates delivery fees.
- Talks to payment systems.
- Sends notifications.
- Updates order status.

In this project, the backend:

- Serves the film list.
- Receives seat hold requests.
- Talks to Redis.
- Returns success or error responses.
- Serves a small static frontend.

### What Is a Server?

A server is a program that waits for requests.

In this project, the Go program starts an HTTP server on port `8080`. That means it listens for web requests such as:

```text
GET /api/v1/films
POST /api/v1/films/inception/seats/A1/claims
PUT /api/v1/claims/{claimID}/confirm
```

When a request arrives, the server looks at the route and calls the matching handler function.

### What Is an API?

API stands for Application Programming Interface. In simple terms, it is a contract between a client and a server.

The client says:

> I will send a request in this shape.

The server says:

> I will respond in this shape.

Example request:

```bash
curl -X POST http://localhost:8080/api/v1/films/inception/seats/A1/claims \
  -H 'Content-Type: application/json' \
  -d '{"customer_id":"customer-1"}'
```

Example response:

```json
{
  "claim_id": "7b9f...",
  "film_id": "inception",
  "seat_code": "A1",
  "customer_id": "customer-1",
  "state": "held",
  "expires_at": "2026-05-02T13:20:23Z"
}
```

The client does not need to know how the backend stores the claim internally. It only needs to know the API contract.

### What Is HTTP?

HTTP is the protocol used by browsers and web APIs. It defines methods such as:

- `GET`: read something.
- `POST`: create something.
- `PUT`: update or confirm something.
- `DELETE`: delete or release something.

This project uses HTTP routes like:

```text
GET    /api/v1/films
GET    /api/v1/films/{filmID}/seats
POST   /api/v1/films/{filmID}/seats/{seatCode}/claims
PUT    /api/v1/claims/{claimID}/confirm
DELETE /api/v1/claims/{claimID}
```

### What Is JSON?

JSON is a simple text format for sending structured data.

Example:

```json
{
  "customer_id": "customer-1",
  "seat_code": "A1"
}
```

JSON is used because it is easy for browsers, servers, and tools like `curl` to understand.

### What Is Docker?

Docker packages an application and its runtime environment into a container.

Without Docker, one developer might have Go 1.23, another might have Go 1.25, Redis might be installed differently, environment variables might differ, and the app might work on one machine but fail on another.

Docker helps by saying:

> Run this application inside a predictable environment.

In this project, Docker is used for:

- Running Redis.
- Running the Go API in a container.
- Providing a repeatable local environment with Docker Compose.

### What Is Redis?

Redis is a very fast in-memory data store.

Most databases store data primarily on disk. Redis stores data primarily in memory, which makes it very fast for reads and writes. Redis can also persist to disk, but its main strength is speed.

Redis is often used for:

- Caching.
- Rate limiting.
- Distributed locks.
- Temporary sessions.
- Queues.
- Fast counters.
- TTL-based temporary data.

In this project, Redis stores seat claims.

### Database vs Cache

A database is usually the long-term source of truth.

A cache is usually a fast temporary storage layer.

But Redis can sometimes be used as the main store for specific workflows. This project uses Redis as the active reservation store because seat holds are temporary and need atomic operations.

For a production cinema platform, I would usually add a durable database such as PostgreSQL for confirmed bookings. Redis would still handle fast temporary holds, but the confirmed ticket would be persisted elsewhere.

### What Is TTL?

TTL means Time To Live.

It tells Redis:

> Keep this key for this much time, then automatically delete it.

In this project, a held seat expires after the configured hold time, default `2m`.

That means if a user holds a seat but never confirms it, Redis automatically frees it later.

### What Is Atomicity?

Atomicity means an operation happens fully or not at all.

If two users try to reserve seat `A1`, the system must not do this:

1. User A checks seat.
2. User B checks seat.
3. Both see it as available.
4. Both write a reservation.

Instead, the system needs a single atomic operation:

> Create this reservation only if the seat does not already exist.

Redis supports this kind of operation very efficiently.

---

## 3. Project Overview

### High-Level Working

The system has four major parts:

1. Browser UI.
2. Go HTTP API.
3. Reservation business logic.
4. Redis cache/store.

The flow looks like this:

```text
Browser
  -> Go HTTP API
      -> Reservation service
          -> Redis cache abstraction
              -> Redis server
```

When a user clicks a seat:

1. The browser sends a `POST` request.
2. The Go handler reads the request.
3. The reservation service validates the input.
4. The repository asks Redis to atomically claim the seat.
5. Redis either creates the claim or rejects it.
6. The API returns success or conflict.

### The Main Business Concept: A Claim

Instead of saying "booking" immediately, the refactored system uses the word "claim."

Why?

Because when a user first clicks a seat, they have not fully purchased it yet. They have only claimed it temporarily.

A claim can be:

- `held`: temporary and expires.
- `confirmed`: permanent within this Redis-backed model.

This naming is useful because it separates:

- "I am holding this seat while I decide."
- "I have confirmed this seat."

### End-to-End Example

Suppose customer `c1` wants seat `A1` for film `inception`.

Request:

```text
POST /api/v1/films/inception/seats/A1/claims
```

Body:

```json
{
  "customer_id": "c1"
}
```

The system creates:

```text
cinema:v1:seat:inception:A1   -> claim JSON
cinema:v1:claim:{claim_id}    -> cinema:v1:seat:inception:A1
```

The seat key stores the claim details. The claim key is a reverse lookup. It lets the system find the seat if the user later says:

```text
PUT /api/v1/claims/{claimID}/confirm
```

Without the reverse lookup, the system would need to search all seats to find the claim, which is inefficient.

---

## 4. System Capabilities

This is the most important section because a system is not just its code. A system is also its limits.

### What Features Does It Support?

Current supported features:

- Serve a static browser UI.
- List films.
- List reserved seats for a film.
- Temporarily hold a seat.
- Automatically expire held seats using Redis TTL.
- Confirm a held seat.
- Prevent releasing already confirmed seats.
- Reject a duplicate hold on the same seat.
- Reject a confirm/release from a different customer.
- Return structured error responses.
- Add request IDs to responses.
- Log requests with structured JSON logs.
- Provide health and readiness endpoints.
- Run locally or with Docker Compose.

### What It Can Actually Do

At its core, this system can coordinate seat reservation safely across multiple simultaneous users as long as all application instances share the same Redis server.

That means even if I run 5 Go API containers behind a load balancer, they can still coordinate correctly because Redis is the central state holder. The Go app instances are mostly stateless.

This is a strong property.

### Request Types and Relative Cost

Not every endpoint costs the same.

#### Very Cheap Requests

```text
GET /healthz
GET /api/v1/films
```

These do not require Redis. They should be very fast and can handle high throughput.

#### Moderately Cheap Requests

```text
POST /api/v1/films/{filmID}/seats/{seatCode}/claims
PUT /api/v1/claims/{claimID}/confirm
DELETE /api/v1/claims/{claimID}
```

These hit Redis. They are still fast, but network and Redis latency matter.

#### Potentially Expensive Requests

```text
GET /api/v1/films/{filmID}/seats
```

This endpoint scans Redis keys for the selected film and reads claim data. Its cost increases with the number of reserved seats for that film.

This endpoint is the most likely performance bottleneck in the current design.

### Estimated Requests Per Second

These are not benchmark numbers. They are design estimates based on common Go + Redis behavior.

Assumptions:

- One Go API instance.
- One Redis instance.
- Redis is on the same machine or same local network.
- JSON payloads are small.
- No TLS termination inside the Go process.
- Modest machine: 2-4 CPU cores, 4-8 GB RAM.
- Redis connection pool around 20 connections.
- Normal Docker local/network overhead.

Under those assumptions:

| Endpoint Category | Rough Sustainable Estimate | Reasoning |
| --- | ---: | --- |
| Health/catalog endpoints | 5,000-20,000+ RPS | Mostly in-memory Go HTTP work |
| Seat hold endpoint | 1,000-5,000 RPS | Redis Lua script + JSON + HTTP overhead |
| Confirm/release | 1,000-5,000 RPS | A few Redis operations per request |
| Seat list endpoint | 100-2,000 RPS | Depends heavily on number of reserved seats |

These numbers can be higher on stronger hardware and lower on weaker hardware.

The main point:

> The hold path is reasonably scalable. The seat-list polling path is the bigger risk.

### How Many Simultaneous Users Can Use It?

This depends on user behavior.

The browser UI polls seat status every 2 seconds.

If `N` users are active, polling alone creates:

```text
N / 2 requests per second
```

Examples:

| Active Users | Polling RPS |
| ---: | ---: |
| 100 | 50 RPS |
| 1,000 | 500 RPS |
| 5,000 | 2,500 RPS |
| 10,000 | 5,000 RPS |

For 100-1,000 active users, this design is plausible on a decent machine if seat-list data is small.

For 10,000 active users, polling becomes a serious issue. The system may still survive for simple catalog endpoints, but Redis scanning and repeated JSON work can become expensive.

### How Performance Changes with Load

At low load:

- Requests are fast.
- Redis responds quickly.
- The app has idle CPU.
- The browser UI feels responsive.

At medium load:

- Redis connection pool starts to matter.
- Seat-list requests become more expensive.
- Response times increase gradually.
- Some users may see delayed seat updates.

At high load:

- Redis CPU can spike.
- Go goroutines wait for Redis connections.
- Latency grows sharply.
- Timeouts may happen.
- If polling dominates traffic, the system can waste most of its capacity repeatedly answering "what changed?"

### Main Bottlenecks

#### Bottleneck 1: Redis

All reservation state lives in Redis. Every hold/confirm/release needs Redis. If Redis is slow, the app is slow.

#### Bottleneck 2: Seat Listing

The current `ListByFilm` implementation scans keys using a pattern:

```text
cinema:v1:seat:{filmID}:*
```

This is better than `KEYS`, but it is still a repeated scan. If many users poll at the same time, this becomes costly.

#### Bottleneck 3: Network Latency

If Redis is far away from the Go API, every Redis call costs more time.

#### Bottleneck 4: Connection Pool Size

The Redis pool defaults to 20 connections. If 500 requests arrive at once and all need Redis, many wait for a connection.

#### Bottleneck 5: Single API Instance

One Go process can handle many requests, but it is still limited by CPU, memory, and network.

### Capability Summary

This project is good for:

- Demonstrating concurrency-safe reservations.
- Small to medium traffic.
- Portfolio and interview discussion.
- Local demos.
- Learning Redis-based coordination.

It is not yet ready for:

- Real money payments.
- Multi-region ticketing.
- Massive event launches.
- Full production durability.
- Regulatory or financial audit requirements.

---

## 5. System Limits and Breaking Points

### Where Will It Slow Down First?

The first slowdown will likely happen in seat status listing.

Why?

The UI polls repeatedly. Each poll calls:

```text
GET /api/v1/films/{filmID}/seats
```

The backend asks Redis to scan matching seat keys and fetch each claim. As the number of reserved seats and active users grows, this becomes expensive.

For example:

- 2,000 users are watching the same film.
- UI polls every 2 seconds.
- That is 1,000 seat-list requests per second.
- If each list scans or reads many Redis keys, Redis gets hammered.

This is not a correctness issue. It is a scalability issue.

### What Happens If Traffic Suddenly Spikes?

If a sudden spike arrives:

1. Go accepts many connections.
2. Goroutines are created.
3. Requests needing Redis wait for Redis connections.
4. Redis command queue grows.
5. Latency increases.
6. Some clients may timeout.
7. Readiness may still say Redis is alive even though latency is poor.

The system will not necessarily corrupt data because Redis atomic operations protect seat claims. But it may become slow.

This is an important distinction:

> The system can remain logically correct while still becoming too slow for users.

### Where Will Failure Occur First?

Likely order:

1. Seat-list polling becomes expensive.
2. Redis connection pool saturation.
3. Redis CPU/network saturation.
4. API request latency grows.
5. Clients retry, making load worse.
6. API instances consume more memory from queued goroutines.

### Most Fragile Parts

#### Redis as Single Point of Failure

If Redis is down, reservation operations fail. The API cannot safely claim seats without Redis.

#### Lack of Durable Confirmed Booking Store

Confirmed claims are stored in Redis without TTL. Redis can persist data, but a production system should usually write confirmed bookings to a durable database as the system of record.

#### Polling-Based UI

Polling is simple, but expensive at scale.

#### No Authentication

The system trusts `customer_id` from the request body. A real system needs authentication so users cannot pretend to be someone else.

#### No Payment State Machine

Real bookings involve payment. Payment is tricky because payment success and seat confirmation must be coordinated carefully.

### What If Redis Is Removed?

Without Redis, the app needs another way to coordinate concurrent seat claims.

Options:

- In-memory map with mutex.
- SQL database with unique constraints.
- Distributed lock service.
- Message queue with single consumer.

An in-memory map works only for one API instance. If the app scales horizontally, each instance has its own memory and double booking can happen.

Redis is used because it gives fast shared state across instances.

---

## 6. Folder Structure Explained

Current structure:

```text
cmd/
  main.go
internal/
  features/
    catalog/
      catalog.go
    reservations/
      domain.go
      service.go
      cache_repository.go
      http.go
      service_test.go
  platform/
    cache/
      redis.go
    config/
      config.go
    web/
      middleware.go
      respond.go
static/
  index.html
Dockerfile
docker-compose.yaml
README.md
```

### Why This Structure Exists

The project uses a feature/domain-oriented structure.

Instead of grouping files like this:

```text
handlers/
services/
repositories/
models/
```

it groups by business capability:

```text
features/catalog
features/reservations
```

Why?

Because as the app grows, business features become more important than technical categories.

If we add payments later, we can create:

```text
internal/features/payments
```

If we add users:

```text
internal/features/customers
```

Each feature can own its domain rules, handlers, tests, and data access.

### `cmd/main.go`

This is the application entry point.

It:

- Loads config.
- Creates the logger.
- Opens Redis.
- Creates the catalog library.
- Creates the reservation service.
- Registers HTTP routes.
- Adds middleware.
- Starts the server.

I intentionally keep business logic out of `main.go`. The main file is the wiring layer.

### `internal/features/catalog`

The catalog feature currently stores a small in-memory list of films.

It is intentionally simple because the project focus is reservations, not film management.

In production, this feature might read films from PostgreSQL or another service.

### `internal/features/reservations`

This is the core domain.

Files:

- `domain.go`: business concepts and errors.
- `service.go`: use cases such as hold, confirm, release.
- `cache_repository.go`: persistence adapter backed by cache/Redis.
- `http.go`: HTTP API for reservation operations.
- `service_test.go`: tests for core business rules.

### `internal/platform/cache`

This hides Redis behind an interface.

Why?

The reservation service should not care about Redis command details. It should ask for reservation storage behavior. Redis is an implementation detail.

### `internal/platform/config`

This reads environment variables into a typed config struct.

Why?

Hardcoded values are painful in deployment. In Docker, local development, and production, settings are different. Environment-based config keeps the binary flexible.

### `internal/platform/web`

This contains cross-cutting HTTP helpers:

- JSON response writer.
- Error response writer.
- Request ID handling.
- Logging middleware.
- Panic recovery middleware.

Why not put this inside reservations?

Because catalog, reservations, payments, and future features can all reuse the same web helpers.

### `static/index.html`

This is a small browser UI. It is not a full frontend application with React or Vue. It uses plain HTML, CSS, and JavaScript.

That is enough for this project because the backend design is the main topic.

---

## 7. Core Workflow Step by Step

### Workflow 1: Listing Films

Request:

```text
GET /api/v1/films
```

Steps:

1. Browser calls `/api/v1/films`.
2. `catalog.HTTPHandler` receives the request.
3. It calls `Library.All()`.
4. The library returns the list of films.
5. The handler writes JSON.

No Redis is involved.

### Workflow 2: Listing Seats

Request:

```text
GET /api/v1/films/inception/seats
```

Steps:

1. Browser asks for seat status.
2. Reservation HTTP handler extracts `filmID`.
3. Handler calls `desk.Seats(ctx, filmID)`.
4. Desk validates film ID.
5. Desk calls `ClaimBook.ListByFilm`.
6. Cache repository scans Redis keys for that film.
7. It reads each claim JSON.
8. It converts Redis records into domain claims.
9. Handler converts claims into API response objects.
10. Browser renders held and confirmed seats.

### Workflow 3: Holding a Seat

Request:

```text
POST /api/v1/films/inception/seats/A1/claims
```

Body:

```json
{
  "customer_id": "customer-1"
}
```

Steps:

1. Handler decodes JSON.
2. Handler creates `HoldCommand`.
3. Desk normalizes input.
4. Desk validates required fields.
5. Desk creates a `SeatClaim` with:
   - unique claim ID,
   - film ID,
   - seat code,
   - customer ID,
   - state `held`,
   - expiry time.
6. Desk calls `ClaimBook.Reserve`.
7. Redis Lua script checks whether seat key exists.
8. If seat exists, Redis returns failure.
9. If seat does not exist, Redis creates both:
   - seat key,
   - claim lookup key.
10. Handler returns `201 Created`.

### Workflow 4: Confirming a Claim

Request:

```text
PUT /api/v1/claims/{claimID}/confirm
```

Steps:

1. Handler decodes customer ID.
2. Desk validates claim ID and customer ID.
3. Repository finds claim lookup key.
4. Repository reads seat claim.
5. Desk checks whether claim belongs to same customer.
6. Desk changes state to `confirmed`.
7. Repository stores the claim with no TTL.
8. Handler returns confirmed claim.

### Workflow 5: Releasing a Claim

Request:

```text
DELETE /api/v1/claims/{claimID}
```

Steps:

1. Handler decodes customer ID.
2. Desk finds the claim.
3. Desk checks owner.
4. Desk rejects release if claim is already confirmed.
5. Repository deletes the seat key and claim key.
6. Handler returns `204 No Content`.

---

## 8. Code Explanation

This section explains the important files in a beginner-friendly way.

### `cmd/main.go`

This file is the starting point.

#### Config Loading

```go
settings := config.Load()
```

This reads environment variables and creates one config object.

Why do this?

Because the app should not hardcode Redis address, HTTP port, or hold TTL. Local development and Docker deployment have different settings.

What if we did not do this?

We would need to edit code every time deployment settings changed.

#### Logger Setup

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: settings.LogLevel,
}))
```

This creates structured JSON logging.

Why structured logs?

Plain text logs are easy for humans but harder for machines. JSON logs can be searched and filtered by fields like `method`, `path`, `status`, and `request_id`.

#### Redis Connection

```go
redisStore, err := cache.OpenRedis(ctx, settings.Redis)
```

The app connects to Redis during startup.

Why fail startup if Redis is unavailable?

Because reservation features cannot work correctly without Redis. It is better to fail loudly than to start a broken service.

#### Feature Construction

```go
movies := catalog.NewLibrary()
claimBook := reservations.NewCacheClaimBook(redisStore, settings.Reservation.HoldTTL)
reservationDesk := reservations.NewDesk(claimBook, settings.Reservation.HoldTTL)
```

This creates the application features.

The catalog feature is simple and in-memory.

The reservation feature uses a `ClaimBook` backed by Redis.

#### Route Registration

```go
catalog.NewHTTPHandler(movies).Mount(mux)
reservations.NewHTTPHandler(reservationDesk, logger).Mount(mux)
```

Each feature mounts its own routes.

Why?

It keeps `main.go` from becoming a giant file full of route details.

#### Middleware

```go
handler := web.Chain(
    mux,
    web.RecoverPanic(logger),
    web.AttachRequestID(),
    web.LogRequests(logger),
)
```

Middleware wraps the actual routes.

This means every request gets:

- panic recovery,
- request ID,
- structured logging.

#### HTTP Server Timeouts

```go
ReadHeaderTimeout
ReadTimeout
WriteTimeout
IdleTimeout
```

Timeouts protect the server from slow or stuck clients.

What if we did not set timeouts?

A malicious or broken client could hold connections open too long and waste server resources.

### `internal/features/reservations/domain.go`

This file defines the language of the business.

Important concepts:

- `SeatClaim`: the central domain object.
- `ClaimHeld`: temporary reservation.
- `ClaimConfirmed`: confirmed reservation.
- `HoldCommand`: input for holding a seat.
- `ConfirmCommand`: input for confirming.
- `ReleaseCommand`: input for releasing.

Why separate commands from claims?

A command is what the user wants to do. A claim is the stored result.

This separation makes business logic easier to reason about.

The file also defines errors:

- `ErrSeatTaken`
- `ErrWrongCustomer`
- `ErrClaimNotFound`
- `ErrClaimFinalized`
- `ErrInvalidInput`

These errors are business-level errors. The HTTP layer later translates them into status codes.

### `internal/features/reservations/service.go`

This file contains the business use cases.

The central type is:

```go
type Desk struct {
    book    ClaimBook
    holdTTL time.Duration
    now     func() time.Time
}
```

I use the word `Desk` because conceptually this is like a reservation desk in a cinema. The desk knows the rules. It does not know Redis internals.

#### `Hold`

This method:

1. Normalizes input.
2. Validates required fields.
3. Creates a claim.
4. Asks storage to reserve the seat.

Why put validation here?

Because validation is business logic. Even if we add a CLI or gRPC API later, the same validation should apply.

#### `Seats`

This lists claims for a film.

It is intentionally thin because listing is mostly a read operation.

#### `Confirm`

This method:

1. Validates input.
2. Finds the claim.
3. Verifies the customer owns it.
4. Changes state to confirmed.
5. Saves it.

The ownership check is important. Without it, anyone who knows a claim ID could confirm another user's seat.

#### `Release`

This method:

1. Finds the claim.
2. Verifies owner.
3. Rejects confirmed claims.
4. Deletes held claims.

The decision to reject release for confirmed claims is deliberate. In real systems, canceling confirmed tickets is a separate business process, usually involving refunds and audit logs.

### `internal/features/reservations/cache_repository.go`

This is the bridge between business logic and Redis-backed cache.

The repository stores claims using two keys:

```text
cinema:v1:seat:{filmID}:{seatCode}
cinema:v1:claim:{claimID}
```

#### Why Two Keys?

The seat key answers:

> Is this seat already reserved?

The claim key answers:

> Given a claim ID, which seat does it point to?

Without the claim key, confirming a claim would require scanning all seats.

#### `Reserve`

This calls:

```go
SetJSONPairIfAbsent(...)
```

That method uses a Redis Lua script to create both keys only if the seat key does not exist.

Why Lua?

Because we need two writes to happen as one atomic operation.

What if we wrote the seat key first and the claim key second?

If the process crashed between those writes, Redis could contain a seat claim with no reverse lookup. That would be an inconsistent state.

#### `ListByFilm`

This scans seat keys for a film and reads each claim.

This is simple and works well for small datasets. But at high scale, it is a bottleneck.

#### `Find`

This uses the claim key to find the seat key, then reads the claim record.

#### `Confirm`

This writes the claim with TTL `0`, meaning no expiry.

#### `Delete`

This deletes both the seat key and the claim key.

### `internal/features/reservations/http.go`

This file translates HTTP requests into business commands.

The HTTP layer should not contain the deep business rules. It should:

1. Decode request.
2. Extract path values.
3. Call the service.
4. Translate errors into HTTP responses.

#### Error Mapping

Example:

```go
ErrSeatTaken -> 409 Conflict
ErrWrongCustomer -> 403 Forbidden
ErrClaimNotFound -> 404 Not Found
ErrInvalidInput -> 400 Bad Request
```

This is important because clients need meaningful responses.

If every error were `500 Internal Server Error`, clients would not know whether they made a bad request or the server failed.

### `internal/platform/cache/redis.go`

This file wraps Redis.

The interface includes:

- `GetString`
- `GetJSON`
- `SetString`
- `SetJSON`
- `Delete`
- `Keys`
- `SetJSONPairIfAbsent`
- `Ping`

Why wrap Redis?

Because the reservation domain should not depend on Redis client details. It should depend on behavior.

This also makes testing easier. In tests, we can use an in-memory implementation.

### `internal/platform/config/config.go`

This reads environment variables.

Important settings:

- `PORT`
- `REDIS_ADDR`
- `REDIS_DB`
- `REDIS_POOL_SIZE`
- `HOLD_TTL`
- `STATIC_DIR`
- `LOG_LEVEL`

Why use defaults?

Defaults make local development easy. If I run `go run cmd/main.go`, the app uses sensible values.

### `internal/platform/web`

This package handles common HTTP concerns.

#### Request ID

Every request gets a unique request ID.

Why?

If a user reports an error, logs can be searched by request ID.

#### Logging

Every request logs method, path, status, duration, and request ID.

#### Recovery

If a panic happens, the middleware catches it and returns a controlled `500` response.

What if we did not recover panics?

One panic could crash the entire server process.

### `static/index.html`

This is the browser UI.

It:

- Generates a customer ID.
- Loads films.
- Renders seats.
- Sends hold/confirm/release requests.
- Polls seat status every 2 seconds.

The UI is intentionally small and dependency-free.

The trade-off is that polling is simple but not the best at scale.

---

## 9. Redis in This Project

### Why Redis Is Used

Redis solves three problems here:

1. Fast reads/writes.
2. Automatic expiry with TTL.
3. Atomic coordination for duplicate prevention.

### Key Design

Seat key:

```text
cinema:v1:seat:{filmID}:{seatCode}
```

Example:

```text
cinema:v1:seat:inception:A1
```

Claim key:

```text
cinema:v1:claim:{claimID}
```

Example:

```text
cinema:v1:claim:885f06d3-97ed-48ce-8a68-00e1b942e920
```

The seat key stores full claim JSON.

The claim key stores the corresponding seat key.

### TTL Behavior

When a claim is held:

- Both keys get TTL.
- Default TTL is 2 minutes.

When a claim is confirmed:

- Both keys are stored without TTL.
- The reservation remains.

When a claim is released:

- Both keys are deleted.

### Performance Benefits

Redis is fast because:

- Data is in memory.
- Commands are simple.
- Atomic Lua script runs inside Redis.
- Network payloads are small.

### Critical Analysis: What If Redis Is Removed?

If Redis is removed, we need another concurrency control mechanism.

#### Option 1: In-Memory Map

Good:

- Very fast.
- Simple.

Bad:

- Works only for one API instance.
- Data lost on restart.
- Cannot coordinate across containers.

#### Option 2: PostgreSQL Unique Constraint

Good:

- Durable.
- Strong consistency.
- Better for confirmed bookings.

Bad:

- Higher latency than Redis.
- TTL expiration requires background jobs or scheduled cleanup.

#### Option 3: Distributed Lock Service

Good:

- Explicit locking.

Bad:

- More operational complexity.
- Locks can be misused.

#### Option 4: Message Queue

Good:

- Can serialize all seat claims through one worker.

Bad:

- Adds latency.
- More moving parts.

### Redis Trade-Offs

Redis is excellent for temporary seat holds. It is less ideal as the only permanent record for confirmed purchases.

For production, I would use:

- Redis for temporary holds.
- PostgreSQL for confirmed bookings.
- A transactional outbox for events.
- Payment integration with idempotency keys.

---

## 10. Docker Explained Practically

### Why Docker Is Used

Docker makes the runtime predictable.

This project needs:

- Go app.
- Redis.
- Correct environment variables.
- Network connection between API and Redis.

Docker Compose describes those pieces.

### Dockerfile

The Dockerfile uses a multi-stage build.

Stage 1:

- Uses Go image.
- Downloads dependencies.
- Builds static binary.

Stage 2:

- Uses small Alpine image.
- Copies only the binary and static files.
- Runs as non-root user.

Why multi-stage?

The final image does not need the Go compiler. Removing it makes the image smaller and safer.

### Docker Compose

Compose defines:

- `api`: the Go app.
- `cache`: Redis.
- `cache-admin`: Redis Commander UI.

Redis has a health check. The API waits for Redis to become healthy.

### What If Docker Was Not Used?

Then each developer would need to install Redis manually.

That can work, but it creates setup friction.

Without Docker:

- Redis version may differ.
- Local ports may conflict.
- Setup instructions become longer.
- Reproducing issues becomes harder.

### Other Deployment Options

Alternatives:

- Run binary directly on a VM.
- Deploy container to Kubernetes.
- Use AWS ECS/Fargate.
- Use Fly.io/Render/Railway for small demos.
- Use managed Redis like AWS ElastiCache.

For a portfolio project, Docker Compose is a strong practical choice.

---

## 11. Design Decisions and Trade-offs

### Why Feature-Based Architecture?

Feature-based architecture groups code by business capability.

Pros:

- Easy to find related code.
- Easier to add new domains.
- Avoids giant generic handler/service/repository folders.

Cons:

- Some shared patterns may be duplicated if not disciplined.
- Beginners may find it less familiar than MVC.

### Why Use Interfaces?

`ClaimBook` is an interface.

Why?

The reservation service should not care whether storage is Redis, memory, or SQL.

But too many interfaces can be bad. I introduced an interface only at a boundary where swapping implementation is realistic.

### Why Use Redis Lua Script?

Because we need to write two keys atomically.

Alternative:

- Use `SET NX` for seat key, then set claim key.

Problem:

- Crash between operations creates inconsistency.

Lua is slightly more complex but safer.

### Why Not Use a Full Database?

The current business problem is temporary high-speed reservation. Redis fits that well.

But for confirmed bookings, a durable DB is better.

The senior-engineer answer is:

> Redis is appropriate for temporary claims. It should not be the only source of truth for paid confirmed tickets.

### Why Polling Instead of WebSockets?

Polling is simple and reliable for a small demo.

WebSockets are better for real-time updates at scale, but require more infrastructure and connection management.

For this project, polling is acceptable. For production, I would reconsider it.

---

## 12. Alternative Architectures

### Option A: Simple Monolith with PostgreSQL

Everything in one Go app. PostgreSQL stores films, seats, bookings.

Use a unique constraint:

```sql
UNIQUE(film_id, seat_code)
```

Pros:

- Durable.
- Easy to reason about.
- Good for business records.

Cons:

- Temporary TTL behavior is less natural.
- High-contention seat holds may put pressure on database.

### Option B: Go API + Redis + PostgreSQL

Redis handles temporary holds. PostgreSQL stores confirmed bookings.

Pros:

- Best practical architecture for many real systems.
- Redis handles speed.
- PostgreSQL handles durability.

Cons:

- More complexity.
- Need to coordinate Redis and DB state carefully.

### Option C: Microservices

Separate services:

- Catalog service.
- Reservation service.
- Payment service.
- Notification service.

Pros:

- Teams can own services independently.
- Each service can scale separately.

Cons:

- Much more operational overhead.
- Distributed transactions become difficult.
- Overkill for this project.

### Option D: Event-Driven Reservation

Requests publish events to a queue. Workers process them.

Pros:

- Smooths traffic spikes.
- Can serialize processing.

Cons:

- User may wait longer for confirmation.
- More moving parts.

### My Chosen Architecture

For this project, I chose:

```text
Single Go service + Redis + feature-based modules
```

Why?

It is simple enough to understand, but strong enough to demonstrate real backend design.

---

## 13. Failure Scenarios and Edge Cases

### Redis Down at Startup

Behavior:

- App logs error.
- App exits.

Reason:

Reservation correctness depends on Redis.

### Redis Down During Runtime

Behavior:

- Readiness endpoint fails.
- Reservation requests return server errors.

Improvement:

- Add retries with backoff.
- Add circuit breaker.
- Use managed Redis with failover.

### Duplicate Seat Hold

Behavior:

- First request wins.
- Later requests get `409 Conflict`.

This is expected behavior.

### Wrong Customer Confirm

Behavior:

- API returns `403 Forbidden`.

Why?

The claim belongs to another customer.

### Hold Expires Before Confirm

Behavior:

- Redis deletes keys automatically.
- Confirm returns not found.

This is correct because the seat is no longer held.

### Confirmed Claim Release Attempt

Behavior:

- API returns conflict.

Why?

Confirmed bookings should not be casually deleted. Real cancellation should be separate.

### API Process Crashes

Held claims remain in Redis until TTL expires.

Confirmed claims remain in Redis.

If Redis is configured with persistence, data may survive Redis restart. But for production, confirmed claims should also be in a durable database.

### Client Retries

If a client retries hold request after timeout, it may receive `409` if the first request actually succeeded.

Improvement:

- Add idempotency keys so clients can safely retry.

### Seat List Race Conditions

A user may see a seat as available, click it, and get conflict because another user claimed it milliseconds earlier.

That is normal. The UI should handle conflicts gracefully.

---

## 14. Scalability and Future Improvements

### Vertical Scaling

Vertical scaling means using a bigger machine.

Benefits:

- More CPU.
- More memory.
- More network bandwidth.

This helps until Redis or polling design becomes the bottleneck.

### Horizontal Scaling

Horizontal scaling means running multiple API instances.

This app supports horizontal API scaling because state is in Redis.

Example:

```text
Load Balancer
  -> API instance 1
  -> API instance 2
  -> API instance 3
      all share Redis
```

### Scaling Redis

Options:

- Bigger Redis instance.
- Redis replica for reads.
- Redis Cluster for sharding.
- Managed Redis service.

The current key design can be sharded by film ID if needed.

### Improve Seat Listing

The biggest improvement would be changing seat-list storage.

Better options:

#### Maintain a Redis Set per Film

```text
cinema:v1:film:{filmID}:seats
```

This set contains seat keys or seat codes.

Pros:

- Faster listing.

Cons:

- Must keep set consistent when TTL expires.

#### Store a Single Hash per Film

```text
cinema:v1:film:{filmID}:seat-map
```

Fields:

```text
A1 -> claim JSON
A2 -> claim JSON
```

Pros:

- One Redis key per film.

Cons:

- Per-seat TTL is harder.

#### Use Pub/Sub or WebSockets

Instead of every browser polling, server pushes updates.

Pros:

- Lower repeated read load.
- More real-time.

Cons:

- More complex connection management.

### Add Durable Database

Confirmed bookings should eventually be stored in PostgreSQL.

Pattern:

1. Redis holds seat temporarily.
2. User pays.
3. PostgreSQL stores confirmed booking.
4. Redis claim is marked confirmed or cleaned up.

### Add Idempotency

Clients should send:

```text
Idempotency-Key: abc123
```

Then retries can safely return the original result.

### Add Authentication

Do not trust request body customer IDs in production.

Instead:

- User logs in.
- Backend extracts customer ID from token/session.
- Request body cannot spoof identity.

### Add Metrics

Useful metrics:

- Request count.
- Error count.
- Latency percentiles.
- Redis command latency.
- Seat hold success/conflict ratio.
- Active held claims.

### Add Load Testing

Use tools like:

- k6.
- vegeta.
- wrk.

Benchmark:

- Hold endpoint.
- Confirm endpoint.
- Seat-list endpoint.
- Mixed realistic traffic.

---

## 15. How to Run the Project

### Prerequisites

You need:

- Go installed.
- Docker installed.
- Docker Compose available.

### Run Redis Only

```bash
docker compose up -d cache
```

### Run API Locally

```bash
go run cmd/main.go
```

Open:

```text
http://localhost:8080
```

### Run Everything with Docker

```bash
docker compose up --build
```

### Health Check

```bash
curl http://localhost:8080/healthz
```

Expected:

```json
{"status":"ok"}
```

### Readiness Check

```bash
curl http://localhost:8080/readyz
```

Expected:

```json
{"status":"ready"}
```

### Create a Seat Claim

```bash
curl -X POST http://localhost:8080/api/v1/films/inception/seats/A1/claims \
  -H 'Content-Type: application/json' \
  -d '{"customer_id":"customer-1"}'
```

### Confirm a Claim

```bash
curl -X PUT http://localhost:8080/api/v1/claims/<claim_id>/confirm \
  -H 'Content-Type: application/json' \
  -d '{"customer_id":"customer-1"}'
```

### Release a Held Claim

```bash
curl -X DELETE http://localhost:8080/api/v1/claims/<claim_id> \
  -H 'Content-Type: application/json' \
  -d '{"customer_id":"customer-1"}'
```

### Run Tests

```bash
go test ./...
```

### Run Vet

```bash
go vet ./...
```

---

## 16. Interview Readiness

### Question: What problem does this project solve?

Strong answer:

> It solves the limited-resource reservation problem. Multiple users may try to reserve the same seat concurrently, and the system must guarantee only one user succeeds. I used Redis atomic operations and TTL-based claims to prevent double booking while allowing temporary holds to expire automatically.

### Question: Why Redis?

Strong answer:

> Redis gives fast atomic writes and native TTL. Temporary seat holds are naturally TTL-based. The critical operation is "claim this seat only if it does not already exist." Redis can do that atomically and quickly.

### Question: Can this system scale horizontally?

Strong answer:

> The Go API can scale horizontally because it is mostly stateless. Multiple API instances can share the same Redis instance. The main bottleneck becomes Redis and the seat-list polling endpoint.

### Question: Where will it break first?

Strong answer:

> The likely first bottleneck is the seat listing endpoint. The UI polls every two seconds, and the backend scans Redis keys for a film. With thousands of active users, repeated scans can become expensive. I would improve this with per-film indexes, WebSockets, or cached seat maps.

### Question: Is Redis enough for production?

Strong answer:

> Redis is good for temporary holds, but I would not use it as the only durable source of truth for paid bookings. In production I would add PostgreSQL for confirmed bookings and keep Redis for fast temporary reservation coordination.

### Question: How do you prevent two users from booking the same seat?

Strong answer:

> The reservation repository creates a seat key using an atomic Redis script. If the seat key already exists, the script returns failure. Because Redis executes the script atomically, two requests cannot both create the same seat claim.

### Question: What happens if a user holds a seat and disappears?

Strong answer:

> The held claim has a TTL. Redis automatically deletes it after the hold window. That frees the seat without requiring a background cleanup job.

### Question: What if the API crashes after holding a seat?

Strong answer:

> The hold remains in Redis until TTL expiry. Since the hold operation writes both the seat key and claim key atomically, we avoid partial state. If the claim was confirmed, it remains stored without TTL, but production would also persist that confirmed booking to a durable database.

### Question: What improvements would you make next?

Strong answer:

> I would add authentication, PostgreSQL persistence for confirmed bookings, idempotency keys, load tests, metrics, and a better seat-list strategy. I would also replace polling with WebSockets or server-sent events for high-scale live updates.

---

## 17. Final Engineering Assessment

This project is a strong foundation for an SDE1 portfolio project because it demonstrates a real backend problem: safe reservation under concurrency.

The strongest parts are:

- Clear domain model around claims.
- Atomic Redis-based seat reservation.
- TTL-based expiry.
- Ownership checks.
- Structured error responses.
- Middleware and config.
- Dockerized runtime.

The biggest limitations are:

- Redis is currently the only reservation store.
- No real authentication.
- No payment workflow.
- Seat-list polling can become expensive.
- No production metrics.
- No load-test results.

If I were explaining this in an interview, I would be honest:

> This is not a full production ticketing platform. It is a focused, production-shaped prototype of the hardest part: preventing double booking under concurrent access. The next step would be making confirmed bookings durable, adding authentication, and optimizing live seat status delivery.

That honesty is important. Good engineers do not pretend a demo is production-ready. They understand exactly what it does, why it works, and where it breaks.

---

## Appendix A: Practical Capacity Estimate Recap

Estimated under modest single-instance conditions:

| Category | Estimated RPS | Confidence |
| --- | ---: | --- |
| Health/catalog | 5,000-20,000+ | Medium |
| Hold claim | 1,000-5,000 | Medium-low without benchmark |
| Confirm/release | 1,000-5,000 | Medium-low without benchmark |
| Seat listing | 100-2,000 | Low; depends strongly on seats and polling |

These are not guarantees. Real capacity must be measured.

To measure:

```bash
go test ./...
docker compose up --build
k6 run load-test.js
```

The most honest engineering statement is:

> The design is capable of handling meaningful concurrency, but the exact load limit must be benchmarked on target hardware with realistic traffic.

---

## Appendix B: Suggested Next Milestones

1. Add PostgreSQL for confirmed bookings.
2. Add authentication and derive customer ID from auth context.
3. Add idempotency keys.
4. Add metrics endpoint.
5. Add k6 load tests.
6. Replace polling with WebSockets or server-sent events.
7. Add per-film seat indexes to avoid repeated scans.
8. Add CI pipeline.
9. Add OpenAPI documentation.
10. Add deployment guide for a cloud environment.

At that point, the project would move from "good portfolio backend project" to "very strong SDE1 system design project."

---

## Appendix C: Endpoint-by-Endpoint Operational Analysis

This appendix looks at each endpoint the way I would review it before a production launch. The beginner-friendly explanation tells us what the endpoint does. The operational explanation asks: what resources does it use, how does it fail, and what happens when many people call it at once?

### `GET /healthz`

Purpose:

This endpoint answers a very simple question:

> Is the HTTP server process alive?

It does not check Redis. It does not check business correctness. It only says the Go server is running and able to respond.

Why do this?

In deployment platforms, a health check is often used to know whether the process should be restarted. If the server is not responding at all, the platform can kill and restart it.

Cost:

Very low. It returns a small JSON object from memory.

Expected scalability:

Very high. This endpoint should handle thousands of requests per second on modest hardware because it does almost no work.

Failure behavior:

If this endpoint fails, the Go process is probably down, blocked, or overloaded so badly that it cannot respond.

What a senior engineer would ask:

- Should this endpoint include dependency checks?
- Should Kubernetes liveness and readiness use different endpoints?

My answer:

Yes, they should be separate. This project correctly separates basic liveness from readiness. `healthz` is about process health. `readyz` is about dependency readiness.

### `GET /readyz`

Purpose:

This endpoint answers:

> Can this API currently serve reservation traffic?

It checks Redis by calling `Ping`.

Why do this?

If Redis is down, the API process might still be alive, but reservation features cannot work. A load balancer should avoid sending traffic to an instance that cannot reach its required dependency.

Cost:

One Redis ping.

Expected scalability:

This endpoint is cheap, but it should not be called excessively. Health systems usually call readiness every few seconds, not thousands of times per second.

Failure behavior:

If Redis is unreachable, this endpoint returns `503 Service Unavailable`.

What if we did not have readiness?

The API might receive traffic even though it cannot complete reservation operations. Users would see random failures instead of the infrastructure detecting the unhealthy instance.

### `GET /api/v1/films`

Purpose:

Returns available films.

Current implementation:

The film list is in memory inside `catalog.Library`.

Cost:

Very low. It copies a small slice and writes JSON.

Expected scalability:

Very high. This can likely handle far more traffic than reservation endpoints.

Limit:

The film list is hardcoded. That is fine for a focused reservation project, but not enough for a real cinema platform.

Production alternative:

Store films in a database and cache them in memory or Redis. Film data changes much less often than seat status, so it is a good caching candidate.

### `GET /api/v1/films/{filmID}/seats`

Purpose:

Returns seats that are currently held or confirmed for a film.

Current implementation:

The repository scans Redis for keys matching:

```text
cinema:v1:seat:{filmID}:*
```

Then it reads claim JSON for each matching key.

Cost:

Variable. The cost grows with:

- Number of reserved seats.
- Number of active users polling.
- Redis latency.
- JSON decoding cost.

Why this is fragile:

The browser polls this endpoint every 2 seconds. Polling is easy to implement, but it creates constant background traffic even when nothing changes.

Example:

If 3,000 users are watching seats for the same film:

```text
3,000 users / 2 seconds = 1,500 requests per second
```

If each response scans and reads 200 reserved seats, that can become a lot of Redis work.

Improvement:

Maintain a compact film-level seat map, or push changes to clients through WebSockets/server-sent events.

What if this endpoint is slow?

Seat display becomes stale. Users might see a seat as open for longer than reality. The hold endpoint still protects correctness, but user experience suffers because more users click unavailable seats and get conflicts.

### `POST /api/v1/films/{filmID}/seats/{seatCode}/claims`

Purpose:

Creates a temporary hold on a seat.

Current implementation:

The service creates a `SeatClaim`, then the Redis repository runs an atomic Lua script that creates two keys only if the seat key is absent.

Cost:

One Lua script execution in Redis plus JSON encoding.

Why this endpoint is critical:

This is the correctness boundary. Even if the UI is stale, even if 1,000 users click the same seat, this endpoint must allow only one winner.

Expected scalability:

Good, as long as Redis is healthy. The work per request is small and bounded.

Failure modes:

- Redis down: server error.
- Seat already exists: `409 Conflict`.
- Bad JSON: `400 Bad Request`.
- Missing customer ID: `400 Bad Request`.

What if the client retries?

Without idempotency keys, a retry might create a new claim for a different seat request, or receive conflict if the first request already succeeded. For production, idempotency keys are important.

### `PUT /api/v1/claims/{claimID}/confirm`

Purpose:

Converts a temporary hold into a confirmed reservation.

Current implementation:

The repository finds the claim key, reads the seat claim, the service checks ownership, then repository writes the claim without TTL.

Cost:

Several Redis operations:

- Get claim lookup.
- Get seat claim.
- Write seat claim.
- Write claim lookup.

Expected scalability:

Good for moderate traffic. Payment systems usually reduce confirm traffic compared to browsing traffic because not every browsing user confirms.

Major missing piece:

There is no payment integration. In a real system, confirmation should happen only after payment authorization or capture.

What if confirmation succeeds but payment fails?

This project does not model that scenario. In production, payment and confirmation need a state machine.

### `DELETE /api/v1/claims/{claimID}`

Purpose:

Releases a held claim.

Current implementation:

The service finds the claim, checks ownership, rejects confirmed claims, then deletes both Redis keys.

Cost:

Small number of Redis operations.

Important behavior:

Confirmed claims cannot be released through this endpoint.

Why?

Releasing a confirmed booking is a cancellation/refund operation, not just deleting a temporary hold.

What if the hold already expired?

The claim key is gone, so the endpoint returns not found.

---

## Appendix D: More Detailed Capacity Reasoning

Capacity estimates are easy to fake. I do not want to pretend we know exact numbers without benchmarks. So this section explains how I reason about capacity before running formal load tests.

### The Basic Formula

For any endpoint:

```text
throughput = available workers / average time per request
```

In Go, the server can create many goroutines, so "available workers" is not a fixed thread count like in some older web servers. But requests still compete for:

- CPU time.
- Network sockets.
- Redis connections.
- Memory.
- Kernel resources.

If an endpoint spends most of its time waiting on Redis, the Redis connection pool and Redis latency become the key limits.

### Redis Latency Example

Assume Redis round-trip latency is 1 millisecond.

A simple reservation hold uses roughly one Redis script call. In a perfect world, one connection could do about:

```text
1 second / 1 millisecond = 1,000 operations per second
```

With a pool of 20 connections, the theoretical upper bound is much higher:

```text
20 x 1,000 = 20,000 operations per second
```

But real systems are not perfect. We must subtract:

- JSON encoding.
- HTTP parsing.
- Logging.
- Redis CPU.
- Network jitter.
- Go scheduling.
- Client behavior.

So a practical estimate of 1,000-5,000 hold requests per second on modest hardware is more honest.

### Why Seat Listing Is Different

Seat listing is not one Redis operation.

It does:

1. Scan matching keys.
2. For each key, get JSON.
3. Decode each JSON payload.
4. Build response array.

If a film has 20 reserved seats, this is cheap.

If a film has 5,000 reserved seats, this is expensive.

If 2,000 clients poll that endpoint every 2 seconds, it becomes much more expensive.

The formula becomes roughly:

```text
seat-list work = poll requests per second x reserved seats per film
```

That multiplication is what worries me.

### Browser Polling Math

Current UI polling interval:

```text
2 seconds
```

Request rate:

```text
active users / 2
```

If active users are:

- 100 users -> 50 seat-list RPS.
- 1,000 users -> 500 seat-list RPS.
- 10,000 users -> 5,000 seat-list RPS.

For 100 users, the current design is fine.

For 1,000 users, it may still work if Redis and the API are strong and seat counts are small.

For 10,000 users, I would redesign seat updates.

### What Load Test Would I Run?

I would create separate tests:

1. Hold contention test:
   - Many users try same seat.
   - Expected: exactly one success, rest conflicts.

2. Hold distribution test:
   - Many users reserve different seats.
   - Expected: high throughput.

3. Seat-list test:
   - Many users poll same film.
   - Expected: find saturation point.

4. Mixed realistic test:
   - 80% seat-list.
   - 15% hold.
   - 4% release.
   - 1% confirm.

Why mixed?

Real traffic is rarely one endpoint only.

### Metrics I Would Capture

I would not only measure requests per second. I would measure:

- p50 latency.
- p95 latency.
- p99 latency.
- error rate.
- Redis CPU.
- Redis memory.
- Redis command latency.
- Go process CPU.
- Go memory.
- number of goroutines.
- connection pool wait time.

p99 matters because users feel tail latency. If most users get 20 ms but 1% get 5 seconds, the system still feels broken to some people.

---

## Appendix E: Production Hardening Checklist

This project is production-shaped, but not production-complete. Here is the checklist I would use before real users and money.

### Correctness

- Add authentication.
- Derive customer ID from auth, not request body.
- Add idempotency keys for hold and confirm.
- Store confirmed bookings in PostgreSQL.
- Add payment state machine.
- Add cancellation/refund workflow.
- Add audit logs for confirmed booking changes.

### Reliability

- Use managed Redis or Redis with failover.
- Add Redis timeout settings.
- Add retry policy for safe operations.
- Add circuit breaker for Redis failures.
- Add graceful shutdown.
- Add readiness behavior that considers dependency latency, not only ping success.

### Observability

- Add Prometheus metrics.
- Add tracing.
- Add structured business events.
- Add dashboards.
- Alert on high conflict rate, Redis latency, API p99 latency, and error rate.

### Security

- Add authentication.
- Add authorization checks.
- Validate film and seat existence.
- Rate-limit abusive clients.
- Avoid exposing internal customer IDs in public seat-list responses.
- Add CORS policy if frontend and backend are separate.

### Performance

- Replace polling for large events.
- Optimize seat-list data model.
- Benchmark Redis Lua script under contention.
- Tune Redis pool size.
- Add response compression if payloads grow.

### Deployment

- Add CI pipeline.
- Add container vulnerability scanning.
- Add environment-specific config.
- Add migration strategy if PostgreSQL is introduced.
- Add blue/green or rolling deploy strategy.

---

## Appendix F: Beginner Glossary

### API

A defined way for one program to talk to another program.

### Backend

The server-side application that handles business logic and data.

### Cache

A fast storage layer often used for temporary or frequently accessed data.

### Claim

In this project, a temporary or confirmed reservation for a seat.

### Concurrency

Multiple things happening at the same time.

### Conflict

An HTTP `409` response meaning the request could not be completed because it conflicts with current state. Example: trying to hold an already-held seat.

### Docker

A tool for packaging and running applications in isolated containers.

### Endpoint

A specific URL route on an API, such as `/api/v1/films`.

### Goroutine

A lightweight concurrent execution unit in Go.

### HTTP

The protocol used by web browsers and APIs.

### JSON

A common text format for structured data.

### Middleware

Code that wraps request handling to add common behavior such as logging or panic recovery.

### Redis

A fast in-memory data store used here for atomic seat claims and TTL expiry.

### Request ID

A unique ID assigned to a request so logs can be correlated.

### RPS

Requests per second. A common measure of throughput.

### TTL

Time To Live. A duration after which data automatically expires.

---

## Appendix G: How I Would Explain This Project in a Resume Discussion

If I had only 30 seconds:

> I built a Go-based cinema reservation system that prevents double booking using Redis atomic operations. Users can hold seats temporarily, confirm them, or release them. The system uses versioned APIs, TTL-based holds, structured logging, Docker Compose, health checks, and a feature-based architecture.

If I had 2 minutes:

> The core challenge is high-concurrency seat reservation. If many users click the same seat at the same time, only one should win. I modeled the business concept as a seat claim. A claim starts as held and expires using Redis TTL. When confirmed, it becomes permanent in Redis. I use a Redis Lua script to atomically create both the seat key and reverse claim lookup key, which prevents partial state and double booking. The Go service is stateless, so it can scale horizontally behind a load balancer as long as instances share Redis. The biggest scalability concern is the seat-list endpoint because the UI currently polls every two seconds and the backend scans Redis keys by film. In production, I would add PostgreSQL for confirmed bookings, authentication, idempotency, metrics, and a better real-time seat update strategy.

If an interviewer challenges the design:

> I agree this is not a complete production ticketing platform. Redis is excellent for temporary holds, but I would not rely on it alone for paid confirmed bookings. I would use Redis for high-speed temporary claims and PostgreSQL for durable confirmed orders. I would also replace client polling with WebSockets or server-sent events for large traffic. The current project intentionally focuses on the hardest concurrency problem first.

This answer is strong because it does not oversell. It shows technical understanding and engineering judgment.