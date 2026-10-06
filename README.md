# High-Concurrency Flash Sale Inventory Reservation Microservice

A production-ready Go backend service designed for high-concurrency flash-sale scenarios. It safely reserves inventory in PostgreSQL without overselling, enforces reservation lifecycle state transitions, and handles automatic expiration via background workers.

---

## Technical Features

* **Concurrency Control:** Pessimistic row-level locking (`SELECT ... FOR UPDATE`) in PostgreSQL prevents race conditions under high concurrent traffic.
* **Database Driver:** Native Go `database/sql` with `github.com/lib/pq` driver (No ORM).
* **Automatic Expiration:** Reservations expire after 5 minutes. Locked stock is released lazily on check/confirm and periodically via a background worker (`FOR UPDATE SKIP LOCKED`).
* **API Documentation:** Interactive Swagger UI served at `/docs` and raw OpenAPI spec at `/swagger.json`.
* **CORS Enabled:** Cross-Origin Resource Sharing middleware included for frontend integration.

---

## Tech Stack

* **Language:** Go 1.24+
* **Framework:** Gin (`github.com/gin-gonic/gin`)
* **Database:** PostgreSQL 15+
* **Migrations:** `golang-migrate` (`github.com/golang-migrate/migrate/v4`)
* **Hot Reload:** Air (`github.com/air-verse/air`)
* **Containerization:** Docker & Docker Compose

---

## Project Layout

```text
.
├── cmd/
│   └── server/main.go            # Application entrypoint & graceful shutdown
├── internal/
│   ├── config/                   # Environment loader
│   ├── handler/                  # HTTP controllers (Gin)
│   ├── middleware/               # Logger, recovery, and CORS middleware
│   ├── model/                    # Domain models, request/response DTOs
│   ├── repository/               # SQL queries & transaction runner
│   ├── routes/                   # Route registration
│   ├── service/                  # Business logic & concurrency transactions
│   └── worker/                   # Background reservation cleanup worker
├── migrations/                   # SQL migration scripts
├── docs/                         # OpenAPI specification & Postman collection
├── tests/                        # Integration and concurrency tests
├── docker-compose.yml
├── Dockerfile
├── Makefile
└── README.md
```

---

## Quickstart Guide

### 1. Clone Repository

```bash
git clone https://github.com/muzaim/go-high-throughput-be.git
cd go-high-throughput-be
```

### 2. Prepare Environment Variables

Copy `.env.example` to `.env`:

```bash
cp .env.example .env
```

Default configuration inside `.env`:

```env
APP_PORT=8080
ENV=development

DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=flash_sale
DB_SSLMODE=disable

RESERVATION_EXPIRATION_MINUTES=5
CLEANUP_INTERVAL_SECONDS=10
```

---

## Running Application

### Option A: Using Docker Compose (Recommended)

Run application and PostgreSQL together in containers:

```bash
docker compose up --build -d
```

To stop containers:

```bash
docker compose down -v
```

---

### Option B: Running Locally

1. Start PostgreSQL server on port `5432` with database `flash_sale`.
2. Run database migrations:

```bash
make migrate-up
```

3. Start server with hot-reload:

```bash
make dev
```

Or start standard Go server:

```bash
make run
```

---

## API Documentation & Interactive Swagger UI

* **Swagger UI:** `http://localhost:8080/docs`
* **OpenAPI Spec:** `http://localhost:8080/swagger.json`
* **Postman Collection:** [`docs/postman_collection.json`](docs/postman_collection.json)

---

## API Endpoints Summary

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/api/v1/inventory/items` | Get list of all available items (for frontend dropdowns) |
| `GET` | `/api/v1/inventory/items/:id` | Get single item detail |
| `GET` | `/api/v1/inventory/stock?item_id=item_4021` | Check current item stock |
| `POST` | `/api/v1/inventory/reserve` | Reserve stock for 5 minutes |
| `POST` | `/api/v1/inventory/confirm` | Confirm active reservation & commit stock |
| `GET` | `/health` | Service health check |

---

## Database Schema & State Machine

### Items Table

$$\text{available\_stock} = \text{total\_stock} - \text{reserved\_stock}$$

* `total_stock`: Total physical stock count.
* `reserved_stock`: Stock currently locked by active reservations.

### State Transitions

```text
(Creation) -> ACTIVE -> CONFIRMED (Final)
                   \-> EXPIRED   (Final)
```

---

## Testing

Run integration tests:

```bash
make test
```

Run race detector:

```bash
make test-race
```
