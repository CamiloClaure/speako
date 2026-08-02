# S6 - Payments (Mercado Pago Checkout Pro)

**Branch:** `be/s6-payments`
**You own:** `api/internal/adapters/mercadopago/**`, the checkout and webhook handlers

## Goal

Take money reliably. This is the session where a bug costs real reais, so correctness beats brevity.

## Tasks

1. `PaymentGateway` implementation:
   - create a Checkout Pro preference with Pix, credit card (up to 12x), and boleto enabled
   - `external_reference` = our order id; `back_urls` point at `/checkout/{sucesso,pendente,erro}`;
     `notification_url` at the webhook
   - `FetchPayment(ref)` for reconciliation
2. `POST /api/v1/checkout`: create a `pending` order with an idempotency key derived from
   (user, product, minute) so a double-click cannot create two orders, then return the checkout URL.
3. Webhook handler:
   - verify the `x-signature` HMAC against `MP_WEBHOOK_SECRET` before parsing anything; reject with 401
   - insert into `payment_events` on `provider_event_id`; a duplicate insert means "already processed", return
     200 immediately
   - on approved: transition the order to `paid`, then **in one transaction** grant the entitlement (material)
     or write the credit entries (pack), then queue the confirmation email
   - always answer 200 quickly; do the work synchronously only because it is small, and say so with a
     `ponytail:` comment naming the queue as the upgrade path
4. `speako-cli reconcile`: re-fetch every order still `pending` after 30 minutes and settle it from the
   provider's answer. This is the safety net for a webhook that never arrived.

## Definition of done

- [ ] Sandbox purchase completes for **both** a test card and Pix.
- [ ] Replaying the same webhook 5 times yields exactly one entitlement and one credit batch - asserted in a
      test, not by hand.
- [ ] An unsigned or wrongly-signed webhook returns 401 and writes nothing.
- [ ] A pending Pix order left unpaid expires via `ExpireStaleOrders` and frees nothing it never granted.
- [ ] `MP_ACCESS_TOKEN` appears in no log, test or fixture.

## Traps

- Mercado Pago sends several notification types for one payment; key on the payment id, not the notification.
- Amounts cross the wire as decimals; convert to `int64` cents at the boundary and never earlier.
- Never trust the amount in the callback - re-read the order's own total before granting anything.
