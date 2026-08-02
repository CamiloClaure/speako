# B3 - Auth and member area

**Branch:** `fe/b3-member`
**You own:** `web/src/routes/app/**`, `web/src/features/{auth,materials,credits,orders}/**`

## Goal

Everything a student sees after paying. Build against the MSW fixtures from S3; do not wait for the backend.

## Tasks

1. `/entrar`: email field → "enviamos um link para seu e-mail" state, plus a Google button. Handle an expired
   or reused link with a clear recovery path, not a raw error.
2. Session bootstrap: one `GET /me` on load, a route guard that redirects unauthenticated users and returns
   them to the page they wanted after login.
3. `/painel`: credit balance, next lesson with the join button, most recent material, and one clear next
   action when any of those is empty.
4. `/painel/materiais`: purchased materials, download button, and a locked state for anything not purchased
   with a link to buy.
5. `/painel/creditos`: balance, expiry warning when credits expire within 14 days, and full ledger history in
   plain language ("1 crédito usado - aula de 12/03").
6. `/painel/compras`: order history with status, including a pending Pix order.

## Definition of done

- [ ] Every screen has loading, empty and error states - no bare spinners on a white page.
- [ ] A hard refresh mid-flow restores the same screen.
- [ ] Works at 360 px; touch targets ≥ 44 px.
- [ ] All money rendered through `formatCents`; no `toFixed(2)` anywhere.
- [ ] No business rule reimplemented in TypeScript - the UI displays what the API says.
