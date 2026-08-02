/**
 * Scaffold shell. Session B1 replaces the placeholder with the design-system
 * showcase at /dev/design; B2 adds the public routes and prerendering.
 */
export function App() {
  return (
    <main className="mx-auto flex min-h-dvh max-w-2xl flex-col justify-center gap-4 px-6">
      <p className="w-fit rounded-lg bg-accent-100 px-3 py-1 text-sm font-semibold text-ink">
        scaffold
      </p>
      <h1 className="text-4xl">Speako</h1>
      <p className="text-muted">
        Inglês de verdade, com professor de verdade. Materiais, pacotes de aulas
        e agendamento em um só lugar.
      </p>
      <button
        type="button"
        className="w-fit rounded-lg bg-brand-600 px-5 py-3 font-semibold text-white shadow-chunk-brand transition-transform duration-150 ease-out-soft active:translate-y-1 active:shadow-none"
      >
        Começar agora
      </button>
    </main>
  );
}
