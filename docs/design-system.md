# Speako design system

Produced with `/ui-ux-pro-max`, then corrected where the generic recommendation did not fit the product.
This document is the input to session **B1**; `web/src/design/tokens.css` is its executable half.

## Position

Speako sells lessons that cost real money to an adult who has already failed at English at least once. The
interface must feel **energetic and encouraging**, never childish - "playful" here means confident color and
chunky shapes, not cartoons.

Two rejections from the generic recommendation, on purpose:

- The engine proposed a teal/orange *kids education* palette and **Baloo 2 + Comic Neue**. That register
  undercuts a R$ 600 lesson pack and reads as a children's app. Rejected.
- It also proposed a full-screen GSAP page-transition overlay. Rejected: an extra animation library for one
  effect, on a site whose landing has a 120 KB JS budget.

Kept from the engine: the *Hero → Problem → Solution → Testimonials → CTA* landing order (social proof before
the ask), the "Modern Professional" pairing that puts **Poppins** on headings, the 150-300 ms interaction
timing, and the pre-delivery checklist.

## Color

Every value below was measured with the WCAG formula, not eyeballed.

### Brand - violet

| Token | Hex | Use |
|---|---|---|
| `brand-50` | `#F5F3FF` | tinted section backgrounds |
| `brand-100` | `#EDE9FE` | selected slot, badge background |
| `brand-600` | `#7C3AED` | **primary actions**, links, active nav |
| `brand-700` | `#6D28D9` | hover, and the chunky shadow under a primary button |
| `brand-800` | `#5B21B6` | pressed |

`brand-600` on white: **5.70:1** - passes AA for normal text. White on `brand-600`: **5.70:1** - primary
buttons are safe. White on `brand-700` (hover): **7.10:1**.

### Accent - amber

Amber is the "achievement" color: credits, streaks, highlights. It is **never text on white** (2.15:1 - fails
badly). Use it as a fill with ink on top.

| Pair | Ratio | Verdict |
|---|---|---|
| `#F59E0B` text on white | 2.15:1 | forbidden |
| ink `#101828` on `amber-500` | 8.26:1 | correct usage |
| ink on `amber-100` `#FEF3C7` | 15.94:1 | best for badges |

### Neutrals

| Token | Hex | On white | Use |
|---|---|---|---|
| `ink` | `#101828` | 17.75:1 | body and headings |
| `muted` | `#475467` | 7.69:1 | secondary text |
| `subtle` | `#667085` | 4.97:1 | captions, the smallest text allowed |
| `hint` | `#98A2B3` | 2.58:1 | **decorative only** - placeholders, disabled, icons paired with a label |
| `line` | `#EAECF0` | - | borders, dividers |
| `surface` | `#F9FAFB` | - | cards on white, alternating sections |

### Semantic - two tokens each

The bright semantic colors fail as text on white (`success` 2.62:1, `danger` 3.76:1). So each has a fill and a
text variant, and they are not interchangeable:

| Meaning | `-fill` (backgrounds, icons, bars) | `-text` (words) | Text ratio |
|---|---|---|---|
| success | `#12B76A` | `#067647` | 5.69:1 |
| warning | `#F79009` | `#B54708` | 5.43:1 |
| danger | `#F04438` | `#B42318` | 6.57:1 |
| info | `#2E90FA` | `#175CD3` | 5.99:1 |

Color never carries meaning alone: a cancelled booking gets an icon and the word "cancelada", not just red.

## Typography

- **Display** Poppins 600/700 - headings, prices, numbers that should feel loud
- **Body** Plus Jakarta Sans Variable - everything else; geometric enough to match Poppins, humanist enough to
  read at 15px on a phone
- Both self-hosted through `@fontsource` (latin subsets only). **No Google Fonts CDN** - it is a third-party
  request, an LGPD data transfer, and a render-blocking dependency for no benefit.

Scale (mobile → desktop): `12 · 14 · 16 · 18 · 20 · 24 · 32 · 40 · 56`. Body is 16px minimum on mobile, which
also stops iOS from auto-zooming form fields. Line height 1.5 for body, 1.15 with `-0.02em` tracking for
display. Prices and credit counts use `font-variant-numeric: tabular-nums` so numbers stop jumping.

## Shape and elevation

Radius `12 / 16 / 20 / 28`, pill for badges and credit counts. The personality lives in the shadow: a **hard
offset with no blur**, which collapses on press.

```css
.btn-primary {
  border-radius: var(--radius-lg);
  background: var(--color-brand-600);
  box-shadow: 0 4px 0 0 var(--color-brand-800);
  transition: transform 150ms var(--ease-out-soft), box-shadow 150ms var(--ease-out-soft);
}
.btn-primary:active { transform: translateY(4px); box-shadow: none; }
```

Elevation scale, used consistently: flat (0) for page sections, `--shadow-chunk` for cards and buttons,
`--shadow-lift` for modals and popovers. Nothing else.

## Motion

150-250 ms, `ease-out` entering and `ease-in` leaving, exits at ~65% of the enter duration. Animate only
`transform` and `opacity`. One or two moving elements per view.

Four animations exist in this product, and no more without a reason:

1. button press - the shadow collapse above
2. slot selection - 120 ms scale to 1.03 then settle
3. modal - fade plus 8px rise from the trigger's direction
4. toast - slide in from the bottom on mobile, top-right on desktop

`prefers-reduced-motion: reduce` disables all of them - already wired in `tokens.css`.

## Components (B1 builds these)

| Component | Notes that are easy to get wrong |
|---|---|
| `Button` | primary / secondary / ghost × sm(36) / md(44) / lg(52). `md` is the mobile default because 44px is the touch minimum. Loading state disables and keeps its width - no layout jump. |
| `Input` | visible label always, never placeholder-as-label. Helper text persists; errors appear **below the field** in `danger-text` with an icon. `type` set correctly so mobile keyboards match (`email`, `tel`). |
| `SlotButton` | three states - available, selected, taken. Taken stays visible and disabled, never hidden, so the grid does not reflow. Label reads "09:00 - 10:00", with the timezone stated once above the grid. |
| `CreditPill` | amber-100 fill, ink text, tabular numerals. Shows an expiry warning at ≤ 14 days. |
| `PriceTag` | formats int cents as `R$ 47,00`. Never `toFixed`. Installments as a second line: "ou 4x de R$ 11,75". |
| `Modal` | focus trap, Esc closes, scroll lock, focus returns to the trigger. Confirm dialogs state the consequence in the body, not the title. |
| `Toast` | `aria-live="polite"`, auto-dismiss 4s, never steals focus. |
| `EmptyState` | every empty list gets an illustration-free card with one sentence and one action. "Você ainda não tem créditos" + "Ver pacotes". |

Icons: Lucide, 1.5px stroke, 20px inline / 24px standalone. **No emoji as icons** anywhere.

## Layout

Mobile-first, breakpoints `375 / 768 / 1024 / 1440`. Content max-width 1200px, prose max-width 65ch. Spacing on
a 4px scale, section rhythm `24 / 40 / 64 / 96`. Bottom-fixed CTA bars respect `env(safe-area-inset-bottom)`.
Use `min-h-dvh`, never `100vh`.

## Landing structure

Hero (promise + primary CTA above the fold) → how it works in 3 steps → materials → lesson packs with prices →
proof (real students, name and photo) → FAQ → closing CTA. One primary CTA per screenful; everything else is
secondary or ghost.

## Checklist before any screen ships

- [ ] Contrast measured, not assumed - especially anything amber
- [ ] Focus ring visible on every interactive element, tab order matches visual order
- [ ] Touch targets ≥ 44px with ≥ 8px between them
- [ ] Loading, empty and error states exist
- [ ] Works at 360px with no horizontal scroll
- [ ] `prefers-reduced-motion` respected
- [ ] Money through `formatCents`, dates through `formatDateTime(tz)`
- [ ] No emoji icons, no `toFixed(2)`, no hardcoded hex in a component
