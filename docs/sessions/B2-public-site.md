# B2 - Public site, SEO and lead capture

**Branch:** `fe/b2-public`
**You own:** `web/src/routes/public/**`, `web/prerender.*`, `web/public/**`, `web/index.html`

## Goal

The pages that sell. They must load fast on a mid-range Android over 4G and be indexable in pt-BR.

## Tasks

1. Routes: `/` · `/materiais` · `/materiais/:slug` · `/aulas` · `/sobre` · `/faq` · `/gratis` · `/termos` ·
   `/privacidade`. Copy in pt-BR, warm and direct, no marketing hyperbole.
2. Landing structure: promise → how it works (3 steps) → materials → lesson packs with prices → proof →
   FAQ → CTA. One primary CTA per screenful.
3. `/gratis`: lead magnet form (name + email + explicit LGPD consent checkbox with the purpose stated), then a
   "check your inbox" state. The checkbox is never pre-checked.
4. `/termos` and `/privacidade`: real content - CDC art. 49 seven-day withdrawal, credit expiry, cancellation
   policy, what data is collected and how to request deletion (LGPD).
5. Prerender the public routes to static HTML at build time (`react-dom/server` + a small build script; do not
   add a meta-framework). Per-route `<title>`, description, canonical, OG image, `lang="pt-BR"`,
   `sitemap.xml`, `robots.txt`, JSON-LD `Course` and `Person`.

## Definition of done

- [ ] `curl` on a built page returns the real copy in the HTML, not an empty `<div id="root">`.
- [ ] Lighthouse mobile ≥ 90 in all four categories.
- [ ] Landing ships ≤ 120 KB of gzipped JS.
- [ ] Renders correctly at 360 px wide with no horizontal scroll.
- [ ] Prices come from the API/fixtures, never hardcoded in a component.
