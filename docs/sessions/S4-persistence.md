# S4 - Persistence adapter

**Branch:** `be/s4-persistence`
**You own:** `api/internal/adapters/sqlite/**`, `api/cmd/speako-cli/**`, `api/internal/config/**`

## Goal

Make every port from S2 real on SQLite, with explicit SQL and no framework, plus the CLI the teacher falls
back to.

## Tasks

1. `config`: parse every variable in `.env.example` once, validate, and fail startup on a missing required
   value. One struct, no global.
2. Open the database with `modernc.org/sqlite` (pure Go, `CGO_ENABLED=0` must still build) and set
   `journal_mode=WAL`, `foreign_keys=ON`, `busy_timeout=5000`, `synchronous=NORMAL`.
3. Embedded migrations: numbered `.sql` files in `migrations/` + a ~30-line runner that records applied
   versions in a `schema_migrations` table. No migration library.
4. Schema exactly as specified in the plan. Do not forget:
   - `CREATE UNIQUE INDEX bookings_slot ON bookings(teacher_id, starts_at) WHERE status = 'scheduled'`
   - `orders.idempotency_key UNIQUE`, `payment_events.provider_event_id UNIQUE`
   - `entitlements UNIQUE(user_id, product_id)`
   - indexes on `bookings(teacher_id, starts_at)`, `credit_entries(user_id, created_at)`,
     `orders(user_id, created_at)`
5. Implement every repository. Explicit parameterized SQL next to the method. Map rows to domain values;
   never leak `sql.NullString` outward.
6. `TxManager` using a `*sql.Tx` carried in the context, so a use case can compose repo calls atomically.
7. CLI verbs: `migrate`, `seed`, `grant-credit <email> <n> [why]`, `refund <order-id>`,
   `export-leads [path.csv]`.
   `seed` creates: the teacher, weekday availability 09:00-12:00 and 14:00-18:00 (60-min slots), two materials
   and three packs (4/8/12 lessons, 90-day validity).

## Definition of done

- [ ] One shared contract test suite runs the **same assertions** against `app/fakes` and the SQLite repos.
      Where they disagree, the fake is wrong - fix the fake.
- [ ] A concurrency test fires 20 goroutines booking the same slot and asserts exactly one wins, the rest get
      `slot_taken`, and the credit ledger nets to a single debit.
- [ ] `make seed && make run` starts and `/healthz` answers.
- [ ] `CGO_ENABLED=0 go build ./...` succeeds.
- [ ] `make arch` passes.

## Traps

- SQLite has one writer. Keep transactions short; never hold one open across an HTTP call to Mercado Pago.
- Store times as unix seconds INTEGER, money as INTEGER cents. No TEXT dates, no REAL money.
