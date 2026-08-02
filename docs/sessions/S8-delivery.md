# S8 - Files, email and scheduled jobs

**Branch:** `be/s8-delivery`
**You own:** `api/internal/adapters/{r2,resend}/**`, email templates, `api/internal/app/jobs/**`

## Goal

Deliver what people paid for, and tell them about it.

## Tasks

1. R2 `FileStorage`: presigned GET, 5-minute TTL, `Content-Disposition: attachment` with the product's real
   filename. The handler checks the entitlement **before** signing, then 302s.
2. Transactional emails in pt-BR, plain and readable, as `html/template` + a text alternative:
   magic link · purchase confirmed (material) · purchase confirmed (pack, with credit count and expiry) ·
   booking confirmed (with the meeting link and an .ics attachment) · booking cancelled · lesson reminder 24h
   before · lead double opt-in.
3. Jobs driven by a ticker in `cmd/api`: `SendBookingReminders` (exactly once per booking - record
   `reminder_sent_at`) and `ExpireStaleOrders`.
   Mark it: `// ponytail: in-process ticker, move to a queue the day we run more than one instance`.
4. Materials upload path for the admin: presigned PUT, size and content-type validated server-side.

## Definition of done

- [ ] A signed URL still works within its TTL after logout and 403s after it expires - both asserted.
- [ ] Reminders fire exactly once even if the ticker runs every minute; a restart does not re-send.
- [ ] Every template renders with real data in a test, and none contains a tracking pixel.
- [ ] The .ics has the correct UTC times and a sane `SUMMARY` in pt-BR.
- [ ] `make arch` passes.
