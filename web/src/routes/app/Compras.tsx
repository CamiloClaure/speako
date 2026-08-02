import { useEffect, useState } from "react";
import { Link } from "react-router";
import { api, friendlyMessage, type Order } from "../../api/client";
import { Badge, Button, Card, EmptyState, ErrorNote, Skeleton } from "../../design/primitives";
import { formatCents, formatDateTime } from "../../design/format";

const STATUS: Record<string, { label: string; tone: "success" | "muted" | "danger" | "accent" }> = {
  paid: { label: "Pago", tone: "success" },
  pending: { label: "Aguardando pagamento", tone: "accent" },
  failed: { label: "Não aprovado", tone: "danger" },
  expired: { label: "Expirado", tone: "muted" },
  refunded: { label: "Reembolsado", tone: "muted" },
};

export function Compras() {
  const [orders, setOrders] = useState<Order[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.orders().then(setOrders).catch((e) => setError(friendlyMessage(e)));
  }, []);

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 px-5 py-10">
      <h1 className="text-3xl">Minhas compras</h1>
      {error && <ErrorNote>{error}</ErrorNote>}

      {orders === null ? (
        <Skeleton className="h-28" />
      ) : orders.length === 0 ? (
        <EmptyState
          title="Nenhuma compra ainda"
          description="Suas compras de pacotes e materiais aparecem aqui."
          action={
            <Link to="/#aulas">
              <Button>Ver pacotes</Button>
            </Link>
          }
        />
      ) : (
        <div className="flex flex-col gap-3">
          {orders.map((o) => {
            const s = STATUS[o.status] ?? { label: o.status, tone: "muted" as const };
            return (
              <Card key={o.id} className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <p className="font-semibold tabular-nums">{formatCents(o.amount_cents)}</p>
                  {o.created_at && (
                    <p className="text-sm text-subtle">{formatDateTime(o.created_at)}</p>
                  )}
                </div>
                <Badge tone={s.tone}>{s.label}</Badge>
              </Card>
            );
          })}
        </div>
      )}

      {/* CDC art. 49 is a legal right, not a courtesy - say it where it applies. */}
      <p className="text-sm text-subtle">
        Compras de produtos digitais podem ser canceladas em até 7 dias corridos,
        conforme o art. 49 do Código de Defesa do Consumidor. Escreva para
        ola@speako.com.br para solicitar.
      </p>
    </div>
  );
}
