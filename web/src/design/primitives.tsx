import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode } from "react";
import { formatCents, formatInstallments } from "./format";

/**
 * The primitives every screen is built from. The personality lives in the
 * chunky shadow that collapses on press; everything else stays quiet.
 */

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "secondary" | "ghost" | "danger";
  size?: "sm" | "md" | "lg";
  loading?: boolean;
};

const BUTTON_BASE =
  "inline-flex items-center justify-center gap-2 rounded-lg font-semibold " +
  "transition-transform duration-150 ease-out-soft " +
  "disabled:cursor-not-allowed disabled:opacity-50 disabled:shadow-none disabled:translate-y-0";

const VARIANTS = {
  primary: "bg-brand-600 text-white shadow-chunk-brand hover:bg-brand-700 active:translate-y-1 active:shadow-none",
  secondary: "bg-white text-ink border-2 border-line shadow-chunk hover:border-brand-300 active:translate-y-1 active:shadow-none",
  ghost: "bg-transparent text-brand-700 hover:bg-brand-50",
  danger: "bg-white text-danger-text border-2 border-line hover:border-danger-fill active:translate-y-1",
} as const;

// md is the default because 44px is the minimum comfortable touch target.
const SIZES = { sm: "h-9 px-4 text-sm", md: "h-11 px-5", lg: "h-13 px-7 text-lg" } as const;

export function Button({
  variant = "primary",
  size = "md",
  loading = false,
  disabled,
  children,
  className = "",
  ...rest
}: ButtonProps) {
  return (
    <button
      {...rest}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={`${BUTTON_BASE} ${VARIANTS[variant]} ${SIZES[size]} ${className}`}
    >
      {/* Reserving the spinner's width keeps the button from resizing mid-click. */}
      {loading && <Spinner />}
      {children}
    </button>
  );
}

function Spinner() {
  return (
    <span
      aria-hidden
      className="size-4 shrink-0 animate-spin rounded-full border-2 border-current border-t-transparent"
    />
  );
}

type FieldProps = InputHTMLAttributes<HTMLInputElement> & {
  label: string;
  hint?: string;
  error?: string;
};

export function Field({ label, hint, error, id, className = "", ...rest }: FieldProps) {
  const inputId = id ?? `f-${label.toLowerCase().replace(/\W+/g, "-")}`;
  const describedBy = error ? `${inputId}-err` : hint ? `${inputId}-hint` : undefined;
  return (
    <div className="flex flex-col gap-1.5">
      {/* A visible label, never a placeholder standing in for one. */}
      <label htmlFor={inputId} className="text-sm font-semibold text-ink">
        {label}
      </label>
      <input
        {...rest}
        id={inputId}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy}
        className={`h-12 rounded-md border-2 px-4 text-base outline-none transition-colors
          ${error ? "border-danger-fill" : "border-line focus:border-brand-500"} ${className}`}
      />
      {error ? (
        <p id={`${inputId}-err`} role="alert" className="text-sm font-medium text-danger-text">
          {error}
        </p>
      ) : hint ? (
        <p id={`${inputId}-hint`} className="text-sm text-subtle">
          {hint}
        </p>
      ) : null}
    </div>
  );
}

export function Card({ children, className = "" }: { children: ReactNode; className?: string }) {
  return (
    <div className={`rounded-xl border-2 border-line bg-canvas p-5 shadow-chunk ${className}`}>
      {children}
    </div>
  );
}

export function Badge({
  children,
  tone = "brand",
}: {
  children: ReactNode;
  tone?: "brand" | "accent" | "success" | "danger" | "muted";
}) {
  const tones = {
    brand: "bg-brand-100 text-brand-800",
    // Amber is a fill behind dark ink - never amber text on white.
    accent: "bg-accent-100 text-ink",
    success: "bg-green-100 text-success-text",
    danger: "bg-red-100 text-danger-text",
    muted: "bg-surface text-muted",
  } as const;
  return (
    <span className={`inline-flex items-center rounded-full px-3 py-1 text-xs font-bold ${tones[tone]}`}>
      {children}
    </span>
  );
}

export function CreditPill({ balance, expiringSoon }: { balance: number; expiringSoon?: number }) {
  return (
    <span className="inline-flex items-center gap-2 rounded-full bg-accent-100 px-4 py-2 font-bold text-ink tabular-nums">
      {balance} {balance === 1 ? "crédito" : "créditos"}
      {expiringSoon ? (
        <span className="text-xs font-semibold text-warning-text">
          {expiringSoon} expirando
        </span>
      ) : null}
    </span>
  );
}

export function PriceTag({ cents, installments = true }: { cents: number; installments?: boolean }) {
  const split = installments ? formatInstallments(cents) : null;
  return (
    <div className="flex flex-col">
      <span className="font-display text-2xl font-bold tabular-nums">{formatCents(cents)}</span>
      {split && <span className="text-sm text-subtle tabular-nums">{split}</span>}
    </div>
  );
}

export function Skeleton({ className = "" }: { className?: string }) {
  return <div className={`animate-pulse rounded-md bg-surface ${className}`} />;
}

export function EmptyState({
  title,
  description,
  action,
}: {
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return (
    <Card className="flex flex-col items-center gap-3 py-10 text-center">
      <h3 className="font-display text-xl">{title}</h3>
      <p className="max-w-sm text-muted">{description}</p>
      {action}
    </Card>
  );
}

export function ErrorNote({ children }: { children: ReactNode }) {
  return (
    <p role="alert" className="rounded-md bg-red-50 px-4 py-3 text-sm font-medium text-danger-text">
      {children}
    </p>
  );
}
