/**
 * Thin typed wrapper over the teacher's admin endpoints. Same fetch style and
 * ApiError handling as client.ts: session cookie on every request, failures
 * arrive as an ApiError carrying the server's error code so screens can react
 * to the specific case instead of showing "algo deu errado".
 */

import { ApiError, type Product } from "../../api/client";

const BASE = "/api/v1/admin";

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

// --- shapes returned by the admin API --------------------------------------

/** One weekday of recurring availability. Minutes since midnight, UTC. */
export type AvailabilityRule = {
  weekday: number;
  start_min: number;
  end_min: number;
  slot_min: number;
};

export type AdminBooking = {
  id: string;
  student_email: string;
  status: string;
  start: string;
  end: string;
};

export type AdminOrder = {
  id: string;
  status: string;
  amount_cents: number;
  created_at: string;
};

/** What the teacher fills in to put a product up for sale. */
export type NewProduct = {
  title: string;
  kind: "material" | "pack";
  price_cents: number;
  credits?: number;
  validity_days?: number;
};

// --- endpoints --------------------------------------------------------------

export const adminApi = {
  availability: () =>
    request<{ rules: AvailabilityRule[] }>("/availability").then((r) => r.rules ?? []),
  saveAvailability: (rules: AvailabilityRule[]) =>
    request<{ status: string }>("/availability", {
      method: "PUT",
      body: JSON.stringify({ rules }),
    }),

  bookings: (from: string, to: string) =>
    request<{ bookings: AdminBooking[] }>(
      `/bookings?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`,
    ).then((r) => r.bookings ?? []),
  setBookingStatus: (id: string, status: "completed" | "no_show") =>
    request<{ status: string }>(`/bookings/${id}/status`, {
      method: "POST",
      body: JSON.stringify({ status }),
    }),
  deleteBooking: (id: string) =>
    request<{ status: string }>(`/bookings/${id}`, { method: "DELETE" }),

  orders: () => request<{ orders: AdminOrder[] }>("/orders").then((r) => r.orders ?? []),

  products: () =>
    request<{ products: Product[] }>("/products").then((r) => r.products ?? []),
  createProduct: (product: NewProduct) =>
    request<{ status: string }>("/products", {
      method: "POST",
      body: JSON.stringify(product),
    }),
};
