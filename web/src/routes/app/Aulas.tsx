import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router";
import { api, friendlyMessage, type Booking } from "../../api/client";
import { Badge, Button, Card, EmptyState, ErrorNote, Skeleton } from "../../design/primitives";
import { formatDateTime, refundsOnCancel, userTimezone } from "../../design/format";

export function Aulas() {
  const tz = userTimezone();
  const [bookings, setBookings] = useState<Booking[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [confirming, setConfirming] = useState<Booking | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => setBookings(await api.bookings()), []);

  useEffect(() => {
    load().catch((e) => setError(friendlyMessage(e)));
  }, [load]);

  async function cancel(b: Booking) {
    setBusy(true);
    setError(null);
    try {
      await api.cancelBooking(b.id);
      setConfirming(null);
      await load();
    } catch (err) {
      setError(friendlyMessage(err));
    } finally {
      setBusy(false);
    }
  }

  const now = Date.now();
  const upcoming = (bookings ?? []).filter(
    (b) => b.status === "scheduled" && new Date(b.start).getTime() > now,
  );
  const past = (bookings ?? []).filter(
    (b) => b.status !== "scheduled" || new Date(b.start).getTime() <= now,
  );

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 px-5 py-10">
      <h1 className="text-3xl">Minhas aulas</h1>
      {error && <ErrorNote>{error}</ErrorNote>}

      {bookings === null ? (
        <Skeleton className="h-28" />
      ) : upcoming.length === 0 ? (
        <EmptyState
          title="Nenhuma aula agendada"
          description="Use seus créditos para marcar a próxima."
          action={
            <Link to="/painel/agendar">
              <Button>Agendar aula</Button>
            </Link>
          }
        />
      ) : (
        <section className="flex flex-col gap-3">
          <h2 className="text-xl">Próximas</h2>
          {upcoming.map((b) => (
            <Card key={b.id} className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p className="font-semibold">{formatDateTime(b.start, tz)}</p>
                <p className="text-sm text-subtle">
                  {refundsOnCancel(b.start)
                    ? "Cancelamento gratuito até 24h antes"
                    : "Faltam menos de 24h - o crédito será consumido"}
                </p>
              </div>
              <div className="flex gap-2">
                {b.meeting_url && (
                  <a href={b.meeting_url} target="_blank" rel="noreferrer">
                    <Button size="sm">Entrar na aula</Button>
                  </a>
                )}
                <Button size="sm" variant="danger" onClick={() => setConfirming(b)}>
                  Cancelar
                </Button>
              </div>
            </Card>
          ))}
        </section>
      )}

      {past.length > 0 && (
        <section className="flex flex-col gap-3">
          <h2 className="text-xl">Anteriores</h2>
          {past.map((b) => (
            <Card key={b.id} className="flex items-center justify-between gap-3">
              <span className="text-muted">{formatDateTime(b.start, tz)}</span>
              <Badge tone={b.status === "completed" ? "success" : "muted"}>{statusLabel(b.status)}</Badge>
            </Card>
          ))}
        </section>
      )}

      {confirming && (
        <CancelDialog
          booking={confirming}
          busy={busy}
          onClose={() => setConfirming(null)}
          onConfirm={() => cancel(confirming)}
        />
      )}
    </div>
  );
}

/**
 * The consequence is stated before the student commits, in plain pt-BR, and it
 * changes on the 24h boundary. The server decides for real; this only makes
 * sure nobody is surprised by losing a credit.
 */
function CancelDialog({
  booking,
  busy,
  onClose,
  onConfirm,
}: {
  booking: Booking;
  busy: boolean;
  onClose: () => void;
  onConfirm: () => void;
}) {
  const refunds = refundsOnCancel(booking.start);
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Cancelar aula"
      className="fixed inset-0 z-50 flex items-end justify-center bg-ink/50 p-4 sm:items-center"
    >
      <Card className="w-full max-w-md shadow-lift">
        <h2 className="font-display text-xl font-bold">Cancelar esta aula?</h2>
        <p className="mt-2 text-muted">{formatDateTime(booking.start)}</p>
        <p
          className={`mt-4 rounded-md px-4 py-3 text-sm font-medium ${
            refunds ? "bg-green-50 text-success-text" : "bg-amber-50 text-warning-text"
          }`}
        >
          {refunds
            ? "Cancelar agora devolve seu crédito."
            : "Faltam menos de 24h - o crédito será consumido."}
        </p>
        <div className="mt-5 flex gap-3">
          <Button variant="secondary" className="flex-1" onClick={onClose} disabled={busy}>
            Manter aula
          </Button>
          <Button variant="danger" className="flex-1" onClick={onConfirm} loading={busy}>
            Cancelar aula
          </Button>
        </div>
      </Card>
    </div>
  );
}

function statusLabel(status: string): string {
  const labels: Record<string, string> = {
    completed: "Concluída",
    no_show: "Faltou",
    cancelled_by_student: "Cancelada por você",
    cancelled_by_teacher: "Cancelada pela professora",
    scheduled: "Agendada",
  };
  return labels[status] ?? status;
}
