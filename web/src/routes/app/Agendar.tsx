import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router";
import { api, friendlyMessage, type Credits, type Slot } from "../../api/client";
import { Button, Card, CreditPill, EmptyState, ErrorNote, Skeleton } from "../../design/primitives";
import { formatDayHeading, formatTime, userTimezone } from "../../design/format";

/**
 * Slot picker. The API returns UTC instants; every label here is rendered in
 * the student's own timezone, and the timezone is stated once so nobody books
 * 3am by accident.
 */
export function Agendar() {
  const tz = userTimezone();
  const [slots, setSlots] = useState<Slot[] | null>(null);
  const [credits, setCredits] = useState<Credits | null>(null);
  const [selected, setSelected] = useState<Slot | null>(null);
  const [booking, setBooking] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState<Slot | null>(null);

  const load = useCallback(async () => {
    const from = new Date();
    const to = new Date(Date.now() + 21 * 86_400_000);
    const [s, c] = await Promise.all([
      api.slots(from.toISOString(), to.toISOString(), tz),
      api.credits(),
    ]);
    setSlots(s);
    setCredits(c);
  }, [tz]);

  useEffect(() => {
    load().catch((e) => setError(friendlyMessage(e)));
  }, [load]);

  async function confirm() {
    if (!selected) return;
    setBooking(true);
    setError(null);
    try {
      await api.book(selected.start);
      setDone(selected);
      setSelected(null);
      await load();
    } catch (err) {
      setError(friendlyMessage(err));
      // Someone may have taken it while the dialog was open - refresh the grid.
      await load().catch(() => {});
      setSelected(null);
    } finally {
      setBooking(false);
    }
  }

  const byDay = groupByDay(slots ?? [], tz);

  return (
    <div className="mx-auto flex w-full max-w-4xl flex-col gap-6 px-5 py-10">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-3xl">Agendar aula</h1>
        {credits && <CreditPill balance={credits.balance} expiringSoon={credits.expiring_soon} />}
      </div>

      <p className="text-sm text-muted">
        Horários no seu fuso: <strong>{tz}</strong>
      </p>

      {done && (
        <Card className="border-green-200 bg-green-50">
          <p className="font-semibold text-success-text">
            Aula agendada para {formatDayHeading(done.start, tz)} às {formatTime(done.start, tz)}.
          </p>
          <Link to="/painel/aulas" className="mt-3 inline-block">
            <Button size="sm" variant="secondary">Ver minhas aulas</Button>
          </Link>
        </Card>
      )}

      {error && <ErrorNote>{error}</ErrorNote>}

      {credits?.balance === 0 && (
        <EmptyState
          title="Você não tem créditos"
          description="Compre um pacote de aulas para poder agendar."
          action={
            <Link to="/#aulas">
              <Button>Ver pacotes</Button>
            </Link>
          }
        />
      )}

      {slots === null ? (
        <div className="flex flex-col gap-4">
          {[0, 1, 2].map((i) => <Skeleton key={i} className="h-28" />)}
        </div>
      ) : byDay.length === 0 ? (
        <EmptyState
          title="Nenhum horário disponível"
          description="A agenda das próximas semanas está cheia. Tente novamente em alguns dias."
        />
      ) : (
        <div className="flex flex-col gap-6">
          {byDay.map(([day, daySlots]) => (
            <section key={day} className="flex flex-col gap-3">
              <h2 className="font-display text-lg font-bold capitalize">{day}</h2>
              <div className="flex flex-wrap gap-2">
                {daySlots.map((s) => (
                  <button
                    key={s.start}
                    type="button"
                    disabled={credits?.balance === 0}
                    onClick={() => setSelected(s)}
                    className="rounded-md border-2 border-line bg-canvas px-4 py-2 font-semibold tabular-nums
                      transition-colors hover:border-brand-500 disabled:cursor-not-allowed disabled:opacity-40"
                  >
                    {formatTime(s.start, tz)}
                  </button>
                ))}
              </div>
            </section>
          ))}
        </div>
      )}

      {selected && credits && (
        <ConfirmDialog
          slot={selected}
          tz={tz}
          balance={credits.balance}
          busy={booking}
          onCancel={() => setSelected(null)}
          onConfirm={confirm}
        />
      )}
    </div>
  );
}

function ConfirmDialog({
  slot,
  tz,
  balance,
  busy,
  onCancel,
  onConfirm,
}: {
  slot: Slot;
  tz: string;
  balance: number;
  busy: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onCancel();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onCancel]);

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Confirmar agendamento"
      className="fixed inset-0 z-50 flex items-end justify-center bg-ink/50 p-4 sm:items-center"
    >
      <Card className="w-full max-w-md shadow-lift">
        <h2 className="font-display text-xl font-bold">Confirmar aula</h2>
        <p className="mt-3 capitalize text-muted">{formatDayHeading(slot.start, tz)}</p>
        <p className="font-display text-2xl font-bold tabular-nums">
          {formatTime(slot.start, tz)} - {formatTime(slot.end, tz)}
        </p>
        {/* Say what it costs before they commit, not after. */}
        <p className="mt-4 rounded-md bg-surface px-4 py-3 text-sm tabular-nums">
          Créditos: <strong>{balance}</strong> → <strong>{balance - 1}</strong>
        </p>
        <div className="mt-5 flex gap-3">
          <Button variant="secondary" className="flex-1" onClick={onCancel} disabled={busy}>
            Voltar
          </Button>
          <Button className="flex-1" onClick={onConfirm} loading={busy}>
            Confirmar
          </Button>
        </div>
      </Card>
    </div>
  );
}

function groupByDay(slots: Slot[], tz: string): [string, Slot[]][] {
  const map = new Map<string, Slot[]>();
  for (const s of slots) {
    const key = formatDayHeading(s.start, tz);
    const list = map.get(key);
    if (list) list.push(s);
    else map.set(key, [s]);
  }
  return [...map.entries()];
}
