# B5 - Teacher admin

**Branch:** `fe/b5-admin`
**You own:** `web/src/routes/admin/**`, `web/src/features/admin/**`

## Goal

Four screens so the teacher never needs a developer for a weekly change.

## Tasks

1. `/admin/agenda`: weekly availability editor (per weekday, ranges, slot length) and date exceptions
   (block a day, or a custom window). Saving shows exactly how many slots it creates next week.
2. `/admin/aulas`: bookings by range, mark `completed` or `no_show`, cancel with a reason (which refunds the
   student's credit), and the student's contact.
3. `/admin/produtos`: create and edit materials and packs. Price entered in reais, sent as integer cents -
   the conversion is tested. PDF upload via presigned PUT with a progress state. Activate/deactivate rather
   than delete.
4. `/admin/vendas`: orders with status and totals for the period, plus a leads CSV export button.
5. Role gate in the router **and** rely on the API's own check - the client gate is convenience, not security.

## Definition of done

- [ ] The teacher can publish a new material and change next week's availability without touching a terminal.
- [ ] Entering `47,00` or `47.00` both produce `4700` cents; a test covers it.
- [ ] A non-teacher hitting `/admin/*` gets bounced by the router and 403 from the API.
- [ ] Every destructive action confirms first and says what will happen to the student's credit.
