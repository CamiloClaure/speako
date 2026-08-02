# B4 - Booking and checkout UX

**Branch:** `fe/b4-booking`
**You own:** `web/src/features/{booking,checkout}/**`, `web/src/routes/app/agendar*`, `web/src/routes/checkout/**`

## Goal

The two flows where confusion costs money: picking a time, and paying.

## Tasks

1. `/painel/agendar`: week view, slots as buttons, explicit timezone label
   ("horários no seu fuso: America/Sao_Paulo") with a way to change it. Past and taken slots are visibly
   unavailable, not hidden.
2. Confirmation modal showing the date in full, the duration, and credits before → after. If the balance is
   zero, the modal becomes a "buy a pack" path instead of a dead end.
3. `/painel/aulas`: upcoming and past lessons, join button active from 10 minutes before, cancel action whose
   consequence is stated **before** confirming:
   - ≥ 24h: "Você receberá seu crédito de volta."
   - < 24h: "Faltam menos de 24h - o crédito será consumido."
4. Checkout: `POST /checkout` → redirect to Mercado Pago. Return states:
   - `/checkout/sucesso` - what they bought and where to find it
   - `/checkout/pendente` - Pix explanation, polls `GET /me/orders` every 5s for up to 10 minutes, then offers
     a manual refresh instead of spinning forever
   - `/checkout/erro` - what to try next, with support contact
5. Handle `insufficient_credits`, `slot_taken`, `too_late_to_book` and `too_late_to_cancel` from the error
   envelope with specific pt-BR messages, never a generic "algo deu errado".

## Definition of done

- [ ] The Pix pending path resolves to success without a manual refresh in the happy case.
- [ ] Booking a slot that was taken while the modal was open shows `slot_taken` and refreshes the grid.
- [ ] Cancellation shows the correct consequence on both sides of the 24h boundary (test with a fixture at
      23h59m and 24h01m).
- [ ] The whole flow is usable with a keyboard.
