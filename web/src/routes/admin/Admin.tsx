import { useCallback, useEffect, useState, type FormEvent } from "react";
import { adminApi, type AdminBooking, type AdminOrder } from "./adminApi";
import { friendlyMessage, type Product } from "../../api/client";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  ErrorNote,
  Field,
  PriceTag,
  Skeleton,
} from "../../design/primitives";
import { formatCents, formatDateTime } from "../../design/format";

/**
 * The teacher's side of the platform. Four screens, one file: the parent wires
 * each route. Every destructive action confirms first and states the
 * consequence in plain pt-BR before anything is sent.
 */

const WEEKDAYS = [
  "Domingo",
  "Segunda-feira",
  "Terça-feira",
  "Quarta-feira",
  "Quinta-feira",
  "Sexta-feira",
  "Sábado",
];

const TIME_INPUT =
  "h-11 rounded-md border-2 border-line px-3 text-base outline-none transition-colors " +
  "focus:border-brand-500 disabled:cursor-not-allowed disabled:opacity-50";

function minutesToHM(min: number): string {
  const h = Math.floor(min / 60);
  const m = min % 60;
  return `${String(h).padStart(2, "0")}:${String(m).padStart(2, "0")}`;
}

function hmToMinutes(hm: string): number {
  const [h, m] = hm.split(":").map(Number);
  return (h ?? 0) * 60 + (m ?? 0);
}

// --- AdminAgenda -------------------------------------------------------------

type Row = {
  weekday: number;
  label: string;
  active: boolean;
  start: string;
  end: string;
  slot: number;
};

function newRows(): Row[] {
  return WEEKDAYS.map((label, weekday) => ({
    weekday,
    label,
    active: false,
    start: "08:00",
    end: "09:00",
    slot: 60,
  }));
}

export function AdminAgenda() {
  const [rows, setRows] = useState<Row[]>(newRows);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  const load = useCallback(async () => {
    const rules = await adminApi.availability();
    setRows((prev) =>
      prev.map((row) => {
        const rule = rules.find((r) => r.weekday === row.weekday);
        if (!rule) return row;
        return {
          ...row,
          active: true,
          start: minutesToHM(rule.start_min),
          end: minutesToHM(rule.end_min),
          slot: rule.slot_min,
        };
      }),
    );
  }, []);

  useEffect(() => {
    load().catch((e) => setError(friendlyMessage(e)));
  }, [load]);

  function update(weekday: number, patch: Partial<Row>) {
    setRows((prev) => prev.map((r) => (r.weekday === weekday ? { ...r, ...patch } : r)));
    setSaved(false);
  }

  const slotsPerWeek = rows.reduce((total, r) => {
    if (!r.active) return total;
    const start = hmToMinutes(r.start);
    const end = hmToMinutes(r.end);
    if (end <= start || r.slot <= 0) return total;
    return total + Math.floor((end - start) / r.slot);
  }, 0);

  async function save() {
    const invalid = rows.find(
      (r) => r.active && (hmToMinutes(r.end) <= hmToMinutes(r.start) || r.slot <= 0),
    );
    if (invalid) {
      setError(`Confira o horário de ${invalid.label.toLowerCase()}: o fim deve vir depois do início.`);
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const rules = rows
        .filter((r) => r.active)
        .map((r) => ({
          weekday: r.weekday,
          start_min: hmToMinutes(r.start),
          end_min: hmToMinutes(r.end),
          slot_min: r.slot,
        }));
      await adminApi.saveAvailability(rules);
      setSaved(true);
    } catch (err) {
      setError(friendlyMessage(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 px-5 py-10">
      <h1 className="text-3xl">Agenda semanal</h1>
      <p className="text-muted">Marque seus dias e horários disponíveis para aulas.</p>
      {error && <ErrorNote>{error}</ErrorNote>}

      {rows.length === 0 ? (
        <Skeleton className="h-28" />
      ) : (
        <Card className="p-0">
          <div className="p-5">
            {rows.map((r) => (
              <div
                key={r.weekday}
                className="flex flex-wrap items-center gap-3 border-b border-line py-3 last:border-0"
              >
                <label className="flex w-40 shrink-0 items-center gap-2 font-semibold">
                  <input
                    type="checkbox"
                    checked={r.active}
                    onChange={(e) => update(r.weekday, { active: e.target.checked })}
                    className="size-5 accent-brand-600"
                  />
                  {r.label}
                </label>
                <input
                  type="time"
                  value={r.start}
                  disabled={!r.active}
                  onChange={(e) => update(r.weekday, { start: e.target.value })}
                  className={TIME_INPUT}
                  aria-label={`Início de ${r.label}`}
                />
                <span className="text-subtle">até</span>
                <input
                  type="time"
                  value={r.end}
                  disabled={!r.active}
                  onChange={(e) => update(r.weekday, { end: e.target.value })}
                  className={TIME_INPUT}
                  aria-label={`Fim de ${r.label}`}
                />
                <select
                  value={r.slot}
                  disabled={!r.active}
                  onChange={(e) => update(r.weekday, { slot: Number(e.target.value) })}
                  className={TIME_INPUT}
                  aria-label={`Duração da aula de ${r.label}`}
                >
                  {[30, 45, 60, 90].map((m) => (
                    <option key={m} value={m}>
                      {m} min por aula
                    </option>
                  ))}
                </select>
              </div>
            ))}
          </div>
          <div className="flex flex-wrap items-center justify-between gap-3 border-t border-line bg-surface p-5">
            <p className="font-semibold tabular-nums">
              {slotsPerWeek} {slotsPerWeek === 1 ? "aula" : "aulas"} por semana
            </p>
            <div className="flex items-center gap-3">
              {saved && <span className="text-sm font-medium text-success-text">Salvo</span>}
              <Button onClick={save} loading={saving}>
                Salvar agenda
              </Button>
            </div>
          </div>
        </Card>
      )}
    </div>
  );
}

// --- AdminAulas --------------------------------------------------------------

type BookingAction = "completed" | "no_show" | "cancel";

const CONFIRM_TITLES: Record<BookingAction, string> = {
  completed: "Concluir esta aula?",
  no_show: "Registrar falta?",
  cancel: "Cancelar esta aula?",
};

const CONFIRM_LABELS: Record<BookingAction, string> = {
  completed: "Concluir",
  no_show: "Registrar falta",
  cancel: "Cancelar aula",
};

/** The consequence is stated before the teacher commits, in plain pt-BR. */
function consequenceText(action: BookingAction): string {
  if (action === "cancel") return "Cancelar devolve o crédito ao aluno, sempre.";
  if (action === "no_show") return "Registrar falta consome o crédito do aluno.";
  return "Concluir a aula consome o crédito do aluno.";
}

function statusLabel(status: string): string {
  const labels: Record<string, string> = {
    scheduled: "Agendada",
    completed: "Concluída",
    no_show: "Faltou",
    cancelled_by_student: "Cancelada pelo aluno",
    cancelled_by_teacher: "Cancelada por você",
  };
  return labels[status] ?? status;
}

export function AdminAulas() {
  const [bookings, setBookings] = useState<AdminBooking[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [confirming, setConfirming] = useState<AdminBooking | null>(null);
  const [pendingAction, setPendingAction] = useState<BookingAction | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    const from = new Date();
    from.setHours(0, 0, 0, 0);
    const to = new Date(from.getTime() + 30 * 24 * 60 * 60 * 1000);
    const list = await adminApi.bookings(from.toISOString(), to.toISOString());
    setBookings([...list].sort((a, b) => a.start.localeCompare(b.start)));
  }, []);

  useEffect(() => {
    load().catch((e) => setError(friendlyMessage(e)));
  }, [load]);

  async function act(booking: AdminBooking, action: BookingAction) {
    setBusy(true);
    setError(null);
    try {
      if (action === "cancel") {
        await adminApi.deleteBooking(booking.id);
      } else {
        await adminApi.setBookingStatus(booking.id, action);
      }
      setConfirming(null);
      setPendingAction(null);
      await load();
    } catch (err) {
      setError(friendlyMessage(err));
    } finally {
      setBusy(false);
    }
  }

  function confirm(booking: AdminBooking, action: BookingAction) {
    setConfirming(booking);
    setPendingAction(action);
  }

  const upcoming = (bookings ?? []).filter((b) => b.status === "scheduled");

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 px-5 py-10">
      <h1 className="text-3xl">Próximas aulas</h1>
      {error && <ErrorNote>{error}</ErrorNote>}

      {bookings === null ? (
        <Skeleton className="h-28" />
      ) : upcoming.length === 0 ? (
        <EmptyState
          title="Nenhuma aula agendada"
          description="As aulas dos próximos 30 dias aparecem aqui."
        />
      ) : (
        <section className="flex flex-col gap-3">
          {upcoming.map((b) => (
            <Card key={b.id} className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p className="font-semibold">{formatDateTime(b.start)}</p>
                <p className="text-sm text-subtle">{b.student_email}</p>
              </div>
              <div className="flex gap-2">
                <Button size="sm" onClick={() => confirm(b, "completed")}>
                  Concluída
                </Button>
                <Button size="sm" variant="secondary" onClick={() => confirm(b, "no_show")}>
                  Faltou
                </Button>
                <Button size="sm" variant="danger" onClick={() => confirm(b, "cancel")}>
                  Cancelar
                </Button>
              </div>
            </Card>
          ))}
        </section>
      )}

      {bookings !== null && bookings.filter((b) => b.status !== "scheduled").length > 0 && (
        <section className="flex flex-col gap-3">
          <h2 className="text-xl">Encerradas</h2>
          {bookings
            .filter((b) => b.status !== "scheduled")
            .map((b) => (
              <Card key={b.id} className="flex items-center justify-between gap-3">
                <div>
                  <p className="font-semibold">{formatDateTime(b.start)}</p>
                  <p className="text-sm text-subtle">{b.student_email}</p>
                </div>
                <Badge tone={b.status === "completed" ? "success" : "muted"}>
                  {statusLabel(b.status)}
                </Badge>
              </Card>
            ))}
        </section>
      )}

      {confirming && pendingAction && (
        <ConfirmDialog
          booking={confirming}
          action={pendingAction}
          busy={busy}
          onClose={() => {
            setConfirming(null);
            setPendingAction(null);
          }}
          onConfirm={() => act(confirming, pendingAction)}
        />
      )}
    </div>
  );
}

function ConfirmDialog({
  booking,
  action,
  busy,
  onClose,
  onConfirm,
}: {
  booking: AdminBooking;
  action: BookingAction;
  busy: boolean;
  onClose: () => void;
  onConfirm: () => void;
}) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const returnsCredit = action === "cancel";

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={CONFIRM_TITLES[action]}
      className="fixed inset-0 z-50 flex items-end justify-center bg-ink/50 p-4 sm:items-center"
    >
      <Card className="w-full max-w-md shadow-lift">
        <h2 className="font-display text-xl font-bold">{CONFIRM_TITLES[action]}</h2>
        <p className="mt-2 text-muted">
          {booking.student_email} · {formatDateTime(booking.start)}
        </p>
        <p
          className={`mt-4 rounded-md px-4 py-3 text-sm font-medium ${
            returnsCredit ? "bg-green-50 text-success-text" : "bg-amber-50 text-warning-text"
          }`}
        >
          {consequenceText(action)}
        </p>
        <div className="mt-5 flex gap-3">
          <Button variant="secondary" className="flex-1" onClick={onClose} disabled={busy}>
            Voltar
          </Button>
          <Button variant="danger" className="flex-1" onClick={onConfirm} loading={busy}>
            {CONFIRM_LABELS[action]}
          </Button>
        </div>
      </Card>
    </div>
  );
}

// --- AdminProdutos -----------------------------------------------------------

const KIND_LABELS: Record<string, string> = {
  material: "Material",
  pack: "Pacote de aulas",
};

/**
 * Accepts both "47,00" and "47.00", and even "1.234,56". Returns cents as an
 * integer, or null when the input is not a valid non-negative amount.
 */
function reaisToCents(input: string): number | null {
  const t = input.trim();
  if (!t) return null;
  const normalized = t.includes(",") ? t.replace(/\./g, "").replace(",", ".") : t;
  const value = Number(normalized);
  if (!Number.isFinite(value) || value < 0) return null;
  return Math.round(value * 100);
}

type ProductForm = {
  title: string;
  kind: "material" | "pack";
  price: string;
  credits: string;
  validity: string;
  description: string;
};

const EMPTY_FORM: ProductForm = {
  title: "",
  kind: "material",
  price: "",
  credits: "",
  validity: "",
  description: "",
};

export function AdminProdutos() {
  const [products, setProducts] = useState<Product[] | null>(null);
  const [form, setForm] = useState<ProductForm>(EMPTY_FORM);
  const [priceError, setPriceError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [created, setCreated] = useState(false);

  const load = useCallback(async () => setProducts(await adminApi.products()), []);

  useEffect(() => {
    load().catch((e) => setError(friendlyMessage(e)));
  }, [load]);

  async function create(e: FormEvent) {
    e.preventDefault();
    const priceCents = reaisToCents(form.price);
    if (priceCents === null) {
      setPriceError("Digite um valor válido, como 47,00.");
      return;
    }
    setPriceError(null);
    setSaving(true);
    setError(null);
    setCreated(false);
    try {
      await adminApi.createProduct({
        title: form.title.trim(),
        kind: form.kind,
        price_cents: priceCents,
        ...(form.kind === "pack"
          ? { credits: Number(form.credits), validity_days: Number(form.validity) }
          : {}),
        ...(form.description.trim() ? { description: form.description.trim() } : {}),
      });
      setCreated(true);
      setForm(EMPTY_FORM);
      setProducts(await adminApi.products());
    } catch (err) {
      setError(friendlyMessage(err));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 px-5 py-10">
      <h1 className="text-3xl">Produtos</h1>
      {error && <ErrorNote>{error}</ErrorNote>}

      {products === null ? (
        <Skeleton className="h-28" />
      ) : products.length === 0 ? (
        <EmptyState title="Nenhum produto ainda" description="Crie seu primeiro produto abaixo." />
      ) : (
        <section className="flex flex-col gap-3">
          <h2 className="text-xl">À venda</h2>
          {products.map((p) => (
            <Card key={p.id} className="flex flex-wrap items-center justify-between gap-3">
              <div className="flex flex-col gap-1">
                <p className="font-semibold">{p.title}</p>
                <div className="flex items-center gap-2">
                  <Badge tone={p.kind === "pack" ? "accent" : "brand"}>{KIND_LABELS[p.kind]}</Badge>
                  {p.kind === "pack" && p.credits !== undefined && (
                    <span className="text-sm text-subtle">
                      {p.credits} {p.credits === 1 ? "crédito" : "créditos"}
                    </span>
                  )}
                </div>
              </div>
              <PriceTag cents={p.price_cents} />
            </Card>
          ))}
        </section>
      )}

      <Card>
        <h2 className="font-display text-xl font-bold">Criar produto</h2>
        <form className="mt-4 flex flex-col gap-4" onSubmit={create}>
          <Field
            label="Nome"
            value={form.title}
            onChange={(e) => setForm({ ...form, title: e.target.value })}
            placeholder="Ex.: Pacote 10 aulas"
            required
          />
          <div className="flex flex-col gap-1.5">
            <label htmlFor="kind" className="text-sm font-semibold text-ink">
              Tipo
            </label>
            <select
              id="kind"
              value={form.kind}
              onChange={(e) => setForm({ ...form, kind: e.target.value as ProductForm["kind"] })}
              className="h-12 rounded-md border-2 border-line px-4 text-base outline-none transition-colors focus:border-brand-500"
            >
              <option value="material">Material</option>
              <option value="pack">Pacote de aulas</option>
            </select>
          </div>
          <Field
            label="Preço (R$)"
            value={form.price}
            onChange={(e) => setForm({ ...form, price: e.target.value })}
            placeholder="47,00"
            inputMode="decimal"
            error={priceError ?? undefined}
            hint="Aceitamos 47,00 e 47.00."
          />
          {form.kind === "pack" && (
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <Field
                label="Créditos"
                value={form.credits}
                onChange={(e) => setForm({ ...form, credits: e.target.value })}
                placeholder="10"
                inputMode="numeric"
                required
              />
              <Field
                label="Validade (dias)"
                value={form.validity}
                onChange={(e) => setForm({ ...form, validity: e.target.value })}
                placeholder="90"
                inputMode="numeric"
                required
              />
            </div>
          )}
          <Field
            label="Descrição (opcional)"
            value={form.description}
            onChange={(e) => setForm({ ...form, description: e.target.value })}
            placeholder="O que o aluno recebe?"
          />
          <div className="flex items-center gap-3">
            {created && <span className="text-sm font-medium text-success-text">Produto criado</span>}
            <Button type="submit" loading={saving}>
              Criar produto
            </Button>
          </div>
        </form>
      </Card>
    </div>
  );
}

// --- AdminVendas -------------------------------------------------------------

const ORDER_LABELS: Record<string, string> = {
  paid: "Pago",
  pending: "Pendente",
  failed: "Falhou",
  refunded: "Estornado",
};

function orderTone(status: string): "success" | "muted" | "danger" {
  if (status === "paid") return "success";
  if (status === "failed" || status === "refunded") return "danger";
  return "muted";
}

export function AdminVendas() {
  const [orders, setOrders] = useState<AdminOrder[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    adminApi
      .orders()
      .then(setOrders)
      .catch((e) => setError(friendlyMessage(e)));
  }, []);

  const total = (orders ?? [])
    .filter((o) => o.status === "paid")
    .reduce((sum, o) => sum + o.amount_cents, 0);

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 px-5 py-10">
      <h1 className="text-3xl">Vendas</h1>
      {error && <ErrorNote>{error}</ErrorNote>}

      <Card className="flex items-center justify-between">
        <span className="text-muted">Receita do período</span>
        <span className="font-display text-2xl font-bold tabular-nums">
          {formatCents(total)}
        </span>
      </Card>

      {orders === null ? (
        <Skeleton className="h-28" />
      ) : orders.length === 0 ? (
        <EmptyState title="Nenhuma venda ainda" description="As vendas aparecem aqui quando chegarem." />
      ) : (
        <section className="flex flex-col gap-3">
          {orders.map((o) => (
            <Card key={o.id} className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <p className="font-semibold tabular-nums">{formatCents(o.amount_cents)}</p>
                <p className="text-sm text-subtle">{o.id.slice(0, 8)} · {formatDateTime(o.created_at)}</p>
              </div>
              <Badge tone={orderTone(o.status)}>{ORDER_LABELS[o.status] ?? o.status}</Badge>
            </Card>
          ))}
        </section>
      )}
    </div>
  );
}
