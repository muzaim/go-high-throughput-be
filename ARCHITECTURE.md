# Architecture & Technical Design

## 1. Architectural Design & Database Locking Strategy

### System Structure & Code Organization

The backend is built with Go and Gin following a standard 3-tier layout:

* **Routes (`internal/routes`)**: Configures Gin router, CORS, logger/recovery middleware, health check, and mounts `/api/v1/inventory` endpoints.
* **Handler (`internal/handler`)**: Handles HTTP input validation, JSON request parsing, and error mapping to appropriate HTTP status codes.
* **Service (`internal/service`)**: Orchestrates business logic and explicit database transactions (`Transactor`).
* **Repository (`internal/repository`)**: Runs raw SQL queries via Go's standard `database/sql` driver without using an ORM.

```text
HTTP Request ──► Gin Handler ──► Service ──► Repository / Tx ──► PostgreSQL
```

### Database Schema & Tables

PostgreSQL handles all inventory state across two main tables:

* **`items`**: Stores item metadata, `total_stock`, and `reserved_stock`. Available stock is calculated on the fly (`total_stock - reserved_stock`).
  * `chk_total_stock_non_negative` (`total_stock >= 0`)
  * `chk_reserved_stock_non_negative` (`reserved_stock >= 0`)
  * `chk_reserved_not_exceed_total` (`reserved_stock <= total_stock`)
* **`reservations`**: Tracks reservation state (`ACTIVE`, `CONFIRMED`, `EXPIRED`), quantity, user ID, and expiration timestamp (`expires_at`).
  * `idx_reservations_item_status`: Index on `(item_id, status)` for fast status lookups.
  * `idx_reservations_expiration`: Partial index on `(expires_at, status) WHERE status = 'ACTIVE'` to optimize background worker cleanup queries.

### Purchase & Expiration Flow

```text
POST /reserve
  │
  ├──► Begin DB Transaction
  ├──► SELECT ... FOR UPDATE on items (lock row)
  ├──► Verify available stock (total - reserved >= quantity)
  ├──► Insert ACTIVE reservation (5 min expiration)
  ├──► UPDATE items SET reserved_stock = reserved_stock + quantity
  └──► Commit DB Transaction
```

1. **Reservation (`POST /api/v1/inventory/reserve`)**:
   - Opens an explicit database transaction (`ExecTx`).
   - Locks the target item row using `SELECT ... FOR UPDATE` via `FindByIDWithLock`.
   - Checks if `total_stock - reserved_stock >= requested_quantity`.
   - Inserts an `ACTIVE` reservation with an expiration time set to 5 minutes.
   - Increments `reserved_stock` on the `items` table.
   - Commits the transaction. If stock is insufficient or an error occurs, the transaction rolls back.

2. **Confirmation (`POST /api/v1/inventory/confirm`)**:
   - Opens a transaction and locks the reservation row using `SELECT ... FOR UPDATE`.
   - Verifies the reservation is still `ACTIVE` and not expired (`now < expires_at`). If expired, updates status to `EXPIRED` and releases reserved stock.
   - Marks status as `CONFIRMED` with `confirmed_at`.
   - Decrements both `total_stock` and `reserved_stock` on the `items` table.
   - Commits transaction.

3. **Background Cleanup Worker (`internal/worker`)**:
   - Runs every 10 seconds calling `CleanupExpiredReservations`.
   - Uses a CTE query with `FOR UPDATE SKIP LOCKED` to pick up expired active reservations (`expires_at <= NOW()`).
   - Updates status to `EXPIRED` and decrements `reserved_stock` in bulk without blocking ongoing customer reservations.

### Concurrency & Overselling Safeguards

Overselling is prevented by locking at the database row level rather than relying on application state:
1. **Row-level Locks (`SELECT FOR UPDATE`)**: Concurrent reserve requests on the same item are serialized by PostgreSQL. Only one transaction can hold the lock at a time.
2. **Database CHECK Constraints**: The `chk_reserved_not_exceed_total` constraint on the `items` table acts as a final hard stop at the database level if an invalid stock calculation ever runs.

---

## 2. Distributed Scaling & Failure Modes

### What Happens When Scaled to 10 Instances

```text
[ Clients ] ──► [ Load Balancer ] ──┬──► [ Instance 1 ] ──┐
                                   ├──► [ Instance 2 ] ──┼──► [ PostgreSQL ]
                                   └──► [ Instance N ] ──┘  (Conn saturation & row lock queue)
```

If the Go service runs across 10 instances pointing to one PostgreSQL database:

* **Statefulness**: The backend Go app is stateless. No in-memory cache or session sticky routing is needed. Any instance can process any request.
* **Database Connection Saturation**: If 10 instances each open an unmanaged number of database connections, PostgreSQL can run out of available connection slots (`max_connections`), causing client connection errors.
* **Row Lock Queueing**: On high-demand flash sale items, all 10 instances will attempt `SELECT ... FOR UPDATE` on the same item row. PostgreSQL will queue the locks sequentially, which increases transaction latency for that specific item.
* **Realtime Updates**: The service currently uses REST endpoints for stock checks (`GET /stock`). Frontend clients fetch updated stock manually or via periodic polling. There is no multi-node real-time event sync required in the current setup.

### Proposed Distributed Redesign

```text
                                 ┌──► [ Instance 1 ] ──┐
[ Clients ] ──► [ Load Balancer ] ┼──► [ Instance 2 ] ──┼──► [ PgBouncer ] ──► [ PostgreSQL ]
                                 └──► [ Instance N ] ──┘         │
                                            ▲                    │
                                            └─────── [ Redis ] ──┘ (Optional Pub/Sub for SSE)
```

For high-throughput scaling:

1. **Connection Pooling with PgBouncer**: Put PgBouncer in transaction mode between the Go instances and PostgreSQL to manage connection pooling cleanly without exhausting database limits.
2. **Pub/Sub for Realtime Events (If SSE is added back)**: If realtime push notifications are introduced in the future, use Redis Pub/Sub so that a purchase on Instance 1 publishes an event to Redis, which Instance 2 receives and relays to its connected browser clients.
3. **Read Caching**: Cache public stock query responses (`GET /stock`) in Redis with a short TTL (e.g. 1 second) to reduce direct read pressure on PostgreSQL during heavy flash sales.

---

## 3. Engineering Trade-offs & AI Transparency

### Key Trade-offs

* **PostgreSQL Row Locking vs. Redis Pre-decriments**: Chosen PostgreSQL row locking and explicit transactions over Redis atomic counters to keep data accurate and prevent overselling. This sacrifices raw throughput for strict data consistency.
* **Native `database/sql` vs. ORM**: Avoided ORMs to maintain direct control over SQL queries, transaction boundaries, and CTE locking syntax (`FOR UPDATE SKIP LOCKED`).
* **REST Polling vs. WebSocket/SSE Infrastructure**: Kept backend connections purely stateless with simple HTTP endpoints (`GET /stock`), avoiding the need for sticky sessions or distributed message broker state (like Redis Pub/Sub).
* **Backend Concurrency Focus vs. Frontend Complexity**: Focused execution on strict transaction handling, row locking, and cleanup workers rather than complex frontend optimistic state logic.

### AI Assistance & Engineering Validation

* **Initial AI Suggestion**: During development discussions, an AI tool suggested validating stock in application code before updating:
  ```go
  item := repo.FindByID(ctx, itemID)
  if item.AvailableStock() >= qty {
      repo.IncrementReservedStock(ctx, itemID, qty)
  }
  ```
* **Why It Was Flawed**: This pattern creates a race condition under heavy concurrency. Multiple concurrent requests read the same available stock before any update is saved, leading to overselling.
* **How It Was Fixed**: Used `SELECT ... FOR UPDATE` inside a PostgreSQL transaction to lock the item row before reading stock levels, ensuring atomic check-and-update behavior.
