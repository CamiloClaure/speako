/**
 * The only place money and dates get formatted. Money arrives from the API as
 * integer cents and must never touch a float on the way to the screen.
 */

const brl = new Intl.NumberFormat("pt-BR", {
  style: "currency",
  currency: "BRL",
});

export function formatCents(cents: number): string {
  return brl.format(cents / 100);
}

/** Installments as Brazilian shoppers expect to see them: "4x de R$ 99,98". */
export function formatInstallments(cents: number, max = 12): string | null {
  // Below R$ 50 a month the split stops being a selling point.
  const parts = Math.min(max, Math.max(1, Math.floor(cents / 5000)));
  if (parts < 2) return null;
  return `ou ${parts}x de ${formatCents(Math.round(cents / parts))}`;
}

export function userTimezone(): string {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "America/Sao_Paulo";
}

/** Renders a UTC instant in the viewer's own timezone. */
export function formatDateTime(iso: string, tz = userTimezone()): string {
  return new Intl.DateTimeFormat("pt-BR", {
    weekday: "short",
    day: "2-digit",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
    timeZone: tz,
  }).format(new Date(iso));
}

export function formatTime(iso: string, tz = userTimezone()): string {
  return new Intl.DateTimeFormat("pt-BR", {
    hour: "2-digit",
    minute: "2-digit",
    timeZone: tz,
  }).format(new Date(iso));
}

export function formatDayHeading(iso: string, tz = userTimezone()): string {
  return new Intl.DateTimeFormat("pt-BR", {
    weekday: "long",
    day: "2-digit",
    month: "long",
    timeZone: tz,
  }).format(new Date(iso));
}

/**
 * Whether a lesson can still be cancelled with the credit returned. The server
 * decides for real - this only picks which warning to show before confirming.
 */
export function refundsOnCancel(startsAt: string, freeCancelHours = 24): boolean {
  const hoursLeft = (new Date(startsAt).getTime() - Date.now()) / 3_600_000;
  return hoursLeft >= freeCancelHours;
}
