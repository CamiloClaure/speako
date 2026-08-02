# S1 - Domain layer

**Read first:** `AGENTS.md`, `docs/architecture.md`
**Branch:** `be/s1-domain`
**You own:** `api/internal/domain/**` - nothing else. Do not create adapters, handlers, SQL or a go.sum entry.

## Goal

Implement every business rule as pure Go with stdlib-only imports, plus table-driven tests that pin the
boundaries. When this session ends, someone should be able to read `internal/domain` and understand the whole
product without knowing a database exists.

## Tasks

1. `shared`: `Money` (int64 cents, BRL; `Add`, `Mul(qty)`, `String()` formats "R$ 1.234,56"; no floats, ever),
   `EmailAddress` (normalized lowercase, validated), `ID` (opaque string), `TimeSlot` (start/end UTC,
   `Overlaps`), and a `DomainError` sentinel type so callers can match with `errors.Is`.
2. `identity`: `User` (id, email, name, role student|teacher|admin, timezone as an IANA string validated with
   `time.LoadLocation`), role predicates. `Session` value with expiry check.
3. `catalog`: `Product` with `Kind` (`material` | `pack`). A material carries a file key; a pack carries
   `Credits int` and `ValidityDays int`. Constructor rejects: empty slug, non-positive price, a pack with zero
   credits, a material with no file key.
4. `ordering`: `Order` aggregate and its state machine - `pending → paid | failed | expired`,
   `paid → refunded`. Every other transition returns `ErrIllegalTransition`. Add
   `IsWithinWithdrawalWindow(now)` implementing the CDC art. 49 seven-day right of withdrawal from `paid_at`.
5. `access`: `Entitlement` and `GrantEntitlement(existing []Entitlement, ...)` which is idempotent - granting
   twice for the same (user, product) yields the same single entitlement, no error.
6. `credit`: append-only `Entry{Delta, Reason, RefID, ExpiresAt}` and a `Ledger` type.
   `Balance(now)` = sum of entries whose `ExpiresAt` is zero or after `now`. `Debit` returns
   `ErrInsufficientCredits` when it would drive the balance below zero. Debits consume the **soonest-expiring**
   credits first.
7. `scheduling`:
   - `AvailabilityRule{Weekday, StartMin, EndMin, SlotMin}` and `Exception{Date, Blocked, StartMin, EndMin}`.
   - `GenerateSlots(rules, exceptions, booked []TimeSlot, from, to time.Time, loc *time.Location) []TimeSlot` -
     rules are authored in `loc`, output is UTC, exceptions subtract or replace a day, booked slots are removed.
   - `Booking` with status `scheduled | completed | cancelled_by_student | cancelled_by_teacher | no_show`.
   - `CanBook(slotStart, now, minNotice)` → `ErrTooLateToBook`.
   - `CancelOutcome(booking, now, by, freeCancelHours)` → `RefundCredit` or `ConsumeCredit`. Teacher
     cancellations always refund.
8. `leads`: `Lead` with `Capture(email, source, consentAt, consentIP)` requiring a non-empty consent IP, and
   `Confirm(now)` for double opt-in.

## Definition of done

- [ ] `make arch` passes (proves stdlib-only, no `context`, no `net/http`, no `encoding/json`).
- [ ] `go test ./internal/domain/...` passes, table-driven, and covers these exact boundaries:
      balance never negative · expired credits excluded at the exact expiry second · soonest-expiry consumed
      first · cancel at exactly 24h refunds, at 23h59m59s consumes · booking at exactly minNotice is allowed ·
      every illegal order transition · slot generation across a month boundary and across a day with an
      exception · `Money` arithmetic with no float anywhere.
- [ ] `gofmt -l .` is empty.

## Traps

- Do **not** put `json:` tags on entities. Serialization is `internal/http/dto`'s job.
- Do **not** accept `context.Context`. The arch test will reject it.
- Timezone: generate slots by constructing wall-clock times in `loc` and converting to UTC. Never add a fixed
  offset.
- Prefer values over pointers for entities; return new values instead of mutating in place.
