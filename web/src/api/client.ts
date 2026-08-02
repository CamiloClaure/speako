/**
 * Thin typed wrapper over fetch. No data-fetching library: the app has a
 * handful of endpoints and none of them need a cache.
 *
 * Every request carries the session cookie, and every failure arrives as an
 * ApiError carrying the server's error code so screens can react to the
 * specific case instead of showing "algo deu errado".
 */

const BASE = "/api/v1";

export type ErrorCode =
  | "unauthenticated"
  | "forbidden"
  | "not_found"
  | "validation_failed"
  | "insufficient_credits"
  | "slot_taken"
  | "too_late_to_book"
  | "too_late_to_cancel"
  | "payment_failed"
  | "rate_limited"
  | "internal";

export class ApiError extends Error {
  constructor(
    readonly code: ErrorCode,
    message: string,
    readonly status: number,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

/** pt-BR copy for the cases a student can actually act on. */
const MESSAGES: Partial<Record<ErrorCode, string>> = {
  unauthenticated: "Sua sessão expirou. Entre novamente.",
  insufficient_credits: "Você não tem créditos suficientes para esta aula.",
  slot_taken: "Alguém reservou este horário primeiro. Escolha outro.",
  too_late_to_book: "Este horário está muito próximo para ser agendado.",
  too_late_to_cancel: "Faltam menos de 24h para a aula.",
  rate_limited: "Muitas tentativas. Aguarde um minuto e tente de novo.",
  not_found: "Não encontramos o que você procura.",
  internal: "Tivemos um problema aqui. Tente novamente em instantes.",
};

export function friendlyMessage(err: unknown): string {
  if (err instanceof ApiError) return MESSAGES[err.code] ?? err.message;
  return "Não foi possível completar a ação. Verifique sua conexão.";
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(BASE + path, {
    ...init,
    credentials: "include",
    headers: {
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
      ...init?.headers,
    },
  });

  if (res.status === 204) return undefined as T;

  const text = await res.text();
  const body = text ? JSON.parse(text) : {};

  if (!res.ok) {
    const err = body?.error ?? {};
    throw new ApiError(err.code ?? "internal", err.message ?? res.statusText, res.status);
  }
  return body as T;
}

// --- shapes returned by the API ------------------------------------------

export type Product = {
  id: string;
  slug: string;
  title: string;
  description?: string;
  kind: "material" | "pack";
  price_cents: number;
  credits?: number;
  validity_days?: number;
  active: boolean;
};

export type User = {
  id: string;
  email: string;
  name: string;
  role: "student" | "teacher" | "admin";
  timezone: string;
};

export type Credits = {
  balance: number;
  expiring_soon: number;
  next_expiry?: string;
};

export type Slot = { start: string; end: string };

export type Booking = {
  id: string;
  status: string;
  start: string;
  end: string;
  meeting_url?: string;
};

export type Order = {
  id: string;
  status: string;
  amount_cents: number;
  created_at?: string;
  product_slug?: string;
};

export type Material = {
  slug: string;
  title: string;
  granted_at?: string;
};

// --- endpoints -------------------------------------------------------------

export const api = {
  products: () => request<{ products: Product[] }>("/products").then((r) => r.products ?? []),
  product: (slug: string) => request<Product>(`/products/${slug}`),

  slots: (from: string, to: string, tz: string) =>
    request<{ slots: Slot[] }>(
      `/slots?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}&tz=${encodeURIComponent(tz)}`,
    ).then((r) => r.slots ?? []),

  requestMagicLink: (email: string) =>
    request<{ status: string }>("/auth/magic-link", {
      method: "POST",
      body: JSON.stringify({ email }),
    }),

  verifyMagicLink: (token: string) =>
    request<{ status: string }>("/auth/magic-link/verify", {
      method: "POST",
      body: JSON.stringify({ token }),
    }),

  logout: () => request<{ status: string }>("/auth/logout", { method: "POST" }),

  me: () => request<User>("/me"),
  credits: () => request<Credits>("/me/credits"),
  orders: () => request<{ orders: Order[] }>("/me/orders").then((r) => r.orders ?? []),
  materials: () =>
    request<{ materials: Material[] }>("/me/materials").then((r) => r.materials ?? []),
  bookings: () =>
    request<{ bookings: Booking[] }>("/me/bookings").then((r) => r.bookings ?? []),

  book: (startsAt: string) =>
    request<{ status: string }>("/bookings", {
      method: "POST",
      body: JSON.stringify({ starts_at: startsAt }),
    }),

  cancelBooking: (id: string) =>
    request<{ status: string }>(`/bookings/${id}`, { method: "DELETE" }),

  checkout: (productSlug: string, quantity = 1) =>
    request<{ order_id: string; checkout_url: string }>("/checkout", {
      method: "POST",
      body: JSON.stringify({ product_slug: productSlug, quantity }),
    }),

  captureLead: (email: string, source: string) =>
    request<{ status: string }>("/leads", {
      method: "POST",
      body: JSON.stringify({ email, source }),
    }),
};
