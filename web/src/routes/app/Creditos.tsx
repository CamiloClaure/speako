import { useEffect, useState } from "react";
import { Link } from "react-router";
import { api, friendlyMessage, type Credits } from "../../api/client";
import { Button, Card, CreditPill, EmptyState, ErrorNote, Skeleton } from "../../design/primitives";
import { formatDateTime } from "../../design/format";

export function Creditos() {
  const [credits, setCredits] = useState<Credits | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.credits().then(setCredits).catch((e) => setError(friendlyMessage(e)));
  }, []);

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6 px-5 py-10">
      <h1 className="text-3xl">Seus créditos</h1>
      {error && <ErrorNote>{error}</ErrorNote>}

      {credits === null ? (
        <Skeleton className="h-32" />
      ) : credits.balance === 0 ? (
        <EmptyState
          title="Você não tem créditos"
          description="Cada crédito vale uma aula particular de 60 minutos."
          action={
            <Link to="/#aulas">
              <Button>Ver pacotes</Button>
            </Link>
          }
        />
      ) : (
        <Card className="flex flex-col gap-4">
          <CreditPill balance={credits.balance} expiringSoon={credits.expiring_soon} />

          {credits.next_expiry && (
            <p className="text-muted">
              Próximo vencimento: <strong>{formatDateTime(credits.next_expiry)}</strong>
            </p>
          )}

          {/* Expiring credits are the one thing worth interrupting someone about. */}
          {credits.expiring_soon > 0 && (
            <div className="rounded-md bg-amber-50 px-4 py-3">
              <p className="font-semibold text-warning-text">
                {credits.expiring_soon}{" "}
                {credits.expiring_soon === 1 ? "crédito vence" : "créditos vencem"} em breve
              </p>
              <p className="mt-1 text-sm text-muted">
                Agende suas aulas antes do vencimento para não perdê-los.
              </p>
              <Link to="/painel/agendar" className="mt-3 inline-block">
                <Button size="sm">Agendar agora</Button>
              </Link>
            </div>
          )}

          <Link to="/painel/agendar">
            <Button className="w-full">Agendar aula</Button>
          </Link>
        </Card>
      )}

      <Card>
        <h2 className="font-display text-lg font-bold">Como funcionam os créditos</h2>
        <ul className="mt-3 flex list-disc flex-col gap-2 pl-5 text-muted">
          <li>Um crédito equivale a uma aula particular de 60 minutos.</li>
          <li>Cancelou com 24h ou mais de antecedência? O crédito volta para você.</li>
          <li>Com menos de 24h, o crédito é consumido.</li>
          <li>Cada pacote tem um prazo de validade contado a partir da compra.</li>
        </ul>
      </Card>
    </div>
  );
}
