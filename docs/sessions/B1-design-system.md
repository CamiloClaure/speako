# B1 - Design system

**Branch:** `fe/b1-design`
**You own:** `web/src/design/**`, `docs/design-system.md`
**Input:** `docs/design-system.md` is produced first in Claude Code via `/ui-ux-pro-max`. Follow it exactly -
it is the brand decision, not a suggestion.

## Goal

A small set of primitives that make every later screen fast to build and impossible to make ugly.

## Tasks

1. Extend `web/src/design/tokens.css` per `docs/design-system.md` (the brand scale, spacing, elevation,
   motion). Tailwind v4 `@theme` - there is no `tailwind.config.js` and there should not be one.
2. Primitives, each with every state (default, hover, active, focus-visible, disabled, loading):
   `Button` (primary / secondary / ghost × sm / md / lg, chunky shadow that collapses on press),
   `Input`, `Textarea`, `Select`, `Checkbox`, `Card`, `Badge`, `Modal` (focus trap, Esc, scroll lock),
   `Toast`, `Skeleton`, `EmptyState`, `Avatar`, `Tabs`, `PriceTag` (formats int cents as `R$ 47,00`),
   `CreditPill`, `SlotButton`.
3. `/dev/design` route rendering every component in every state. Excluded from the production build.
4. `formatCents`, `formatDateTime(tz)` and `formatRelative` helpers in `web/src/design/format.ts` - the only
   place money and dates get formatted.

## Definition of done

- [ ] axe reports zero violations on `/dev/design`.
- [ ] Every interactive element is keyboard reachable with a visible focus ring; tab order is sane in `Modal`.
- [ ] Amber is used only as a fill behind dark ink or at display sizes - never as small text on white.
- [ ] `npm run lint` and `npm run build` pass.
- [ ] No component fetches data or knows an endpoint exists.
