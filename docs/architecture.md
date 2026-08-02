# Architecture

## The one idea

Speako's business rules - what a credit is worth, when a cancellation refunds it, when an order may become
paid - are written in plain Go that imports nothing. Everything else (SQLite, HTTP, Mercado Pago, R2, React) is
a **detail** plugged in from the outside. Details change; rules should not have to.

```
                            cmd/api · cmd/speako-cli
                            composition root: reads config,
                            builds concrete adapters, injects
                                        │
             ┌──────────────────────────┼──────────────────────────┐
             ▼                                                     ▼
    internal/http (driving)                            internal/adapters (driven)
    router, middleware, DTOs                           sqlite · mercadopago · r2
    maps domain errors → status                        resend · google · system
             │                                                     ▲
             └────────────────► internal/app ◄─────────────────────┘
                                use cases + ports (interfaces)
                                          │
                                          ▼
                                  internal/domain
                          entities · value objects · policies
                                  (stdlib only)
```

Arrows are *dependencies*, and they all point inward. At runtime control flows the other way: an HTTP request
enters a handler, the handler calls a use case, the use case calls domain objects and ports, and an adapter
does the I/O.

## Why this shape for a project this small

Not ceremony - leverage. Three concrete payoffs:

1. **The rules are testable without infrastructure.** The cancellation-window boundary, credit expiry and the
   order state machine are pure functions over values. Their tests run in milliseconds with no database, which
   is what makes it cheap to keep them correct.
2. **Vendors are replaceable.** Mercado Pago sits behind `ports.PaymentGateway`. Swapping it for Asaas is a new
   file in `adapters/`, not a search-and-replace through handlers.
3. **SQLite is a decision, not a destiny.** Repositories are interfaces. The day write contention shows up in
   metrics, Postgres is a new adapter and the same contract test suite proves it behaves identically.

## The rule, and how it is enforced

| Layer | May import | Never |
|---|---|---|
| `internal/domain` | stdlib (minus the denylist), `internal/domain/*` | third-party, `context`, `net/http`, `database/sql`, `encoding/json`, `os`, `log` |
| `internal/app` | stdlib, `internal/domain`, `internal/app` | adapters, http, `database/sql`, `net/http`, `encoding/json` |
| `internal/http` | stdlib, domain, app, third-party | `internal/adapters` |
| `internal/adapters` | stdlib, domain, app, third-party | `internal/http` |
| `cmd/*` | everything | - |

`api/internal/arch/arch_test.go` reads the real import graph via `go list -json` and fails the build on any
violation. It is verified to actually catch leaks, including stdlib ones like `net/http` inside the domain.
Run it with `make arch`.

Two rules deserve their justification, since they surprise people:

- **`context` is banned in the domain.** A cancellation policy does not get cancelled. Context is a
  concurrency/transport concern and it enters at the `app` layer.
- **`encoding/json` is banned in the domain and app.** Serialization shape is a contract with the outside
  world (`docs/api.yaml`), so it lives in `internal/http/dto`. Entities never carry struct tags.

## Ports

Declared in `internal/app/ports`, by the layer that *needs* them, not the one that implements them:

```
Repositories   UserRepo · ProductRepo · OrderRepo · EntitlementRepo
               CreditLedger · BookingRepo · AvailabilityRepo · LeadRepo
Services       PaymentGateway · FileStorage · Mailer
Ambient        Clock · IDGen · TokenGen · TxManager
```

`Clock` and `IDGen` exist so use-case tests are deterministic - not as speculative abstraction. `TxManager`
exists because "debit a credit and create a booking" must be atomic, and that guarantee cannot live in a
repository method.

## Where each business rule lives

| Rule | Package |
|---|---|
| Money is `int64` cents, BRL, no floats anywhere | `domain/shared` |
| Credit balance = Σ non-expired ledger entries, never negative | `domain/credit` |
| Credit expiry from the pack's validity period | `domain/credit` |
| Minimum booking notice (default 12h) | `domain/scheduling` |
| Free cancellation window (default 24h); teacher cancels always refund | `domain/scheduling` |
| Slot generation from availability rules minus exceptions and bookings | `domain/scheduling` |
| Order state machine and CDC art. 49 seven-day withdrawal window | `domain/ordering` |
| Idempotent entitlement granting | `domain/access` |
| LGPD consent captured with timestamp and IP | `domain/leads` |

If a rule from this table appears in a handler, a SQL query or a React component, that is a bug regardless of
whether the tests pass.

## Time

UTC everywhere in code and storage (`INTEGER` unix seconds). The teacher authors availability in their own
timezone; students see slots in theirs. Conversion happens only at the edges via `time.LoadLocation`. Brazil
dropped DST in 2019 but still spans four zones - fixed offset arithmetic is always wrong.

## Frontend

The React SPA is a *client of the contract*, not a second backend. `docs/api.yaml` is frozen after S3 and
generates `web/src/api/types.ts`. No business rule is reimplemented in TypeScript: the UI may *display* that a
cancellation will burn a credit, but the server decides it.

Public marketing routes are prerendered to static HTML at build time for SEO; the member area and admin are
client-rendered. Everything ships to a CDN as static files, so there is no Node process in production.
