# S7 - Scheduling API

**Branch:** `be/s7-scheduling`
**You own:** the scheduling and availability handlers, `internal/app/jobs` wiring for reminders

## Goal

Turn the scheduling domain into endpoints that behave correctly under concurrency and across timezones.

## Tasks

1. `GET /api/v1/slots?from&to&tz` - public. Reject a range longer than 60 days with `validation_failed`.
   Return UTC instants plus a `label` already formatted for the requested timezone.
2. `POST /api/v1/bookings` - inside one transaction: check credits → domain `CanBook` → insert booking →
   debit ledger. Translate a unique-index violation on the slot into `slot_taken` (409), never a 500.
3. `DELETE /api/v1/bookings/{id}` - ask the domain for the outcome, apply refund or consumption, record
   `cancelled_at` and who cancelled.
4. Teacher endpoints: availability GET/PUT, exceptions POST/DELETE, bookings list by range, attendance
   marking (`completed | no_show`).
5. Attach the meeting URL from config to every booking at creation time.

## Definition of done

- [ ] Concurrency test: 20 simultaneous bookings of the same slot → exactly one 201, nineteen 409 `slot_taken`,
      exactly one credit debited in total.
- [ ] Timezone test: a student in `America/Manaus` and the teacher in `America/Sao_Paulo` see the same instant
      with different labels, and booking lands on the right UTC second.
- [ ] Cancellation boundary verified **through HTTP**, not only in the domain test.
- [ ] Slots exclude blocked exception days and already-booked slots.
- [ ] `make arch` passes.
