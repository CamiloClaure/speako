import { useEffect, useState } from "react";
import { Link } from "react-router";
import { api, friendlyMessage, type Booking, type Credits, type Material } from "../../api/client";
import { useSession } from "../../app/session";
import { Button, Card, CreditPill, EmptyState, ErrorNote, Skeleton } from "../../design/primitives";
import { formatDateTime } from "../../design/format";

export function Painel() {
  const { user } = useSession();
  const [credits, setCredits] = useState<Credits | null>(null);
  const [bookings, setBookings] = useState<Booking[] | null>(null);
  const [materials, setMaterials] = useState<Material[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([api.credits(), api.bookings(), api.materials()])
      .then(([c, b, m]) => {
        setCredits(c);
        setBookings(b);
        setMaterials(m);
      })
      .catch((e) => setError(friendlyMessage(e)));
  }, []);

  const next = bookings
    ?.filter((b) => b.status === "scheduled" && new Date(b.start) > new Date())
    .sort((a, b) => a.start.localeCompare(b.start))[0];

  return (
    <div className="mx-auto flex w-full max-w-4xl flex-col gap-6 px-5 py-10">
      <div>
        <h1 className="text-3xl">Olá, {user?.name ?? "aluno"}</h1>
        <p className="mt-1 text-muted">Seu painel de aulas e materiais.</p>
      </div>

      {error && <ErrorNote>{error}</ErrorNote>}

      <div className="grid gap-5 sm:grid-cols-2">
        <Card className="flex flex-col gap-4">
          <h2 className="font-display text-lg font-bold">Seus créditos</h2>
          {credits === null ? (
            <Skeleton className="h-10 w-40" />
          ) : (
            <>
              <CreditPill balance={credits.balance} expiringSoon={credits.expiring_soon} />
              {credits.expiring_soon > 0 && credits.next_expiry && (
                <p className="text-sm font-medium text-warning-text">
                  {credits.expiring_soon}{" "}
                  {credits.expiring_soon === 1 ? "crédito expira" : "créditos expiram"} em{" "}
                  {formatDateTime(credits.next_expiry)}.
                </p>
              )}
              {credits.balance > 0 ? (
                <Link to="/painel/agendar">
                  <Button className="w-full">Agendar aula</Button>
                </Link>
              ) : (
                <Link to="/#aulas">
                  <Button variant="secondary" className="w-full">Comprar pacote</Button>
                </Link>
              )}
            </>
          )}
        </Card>

        <Card className="flex flex-col gap-4">
          <h2 className="font-display text-lg font-bold">Próxima aula</h2>
          {bookings === null ? (
            <Skeleton className="h-10 w-48" />
          ) : next ? (
            <>
              <p className="font-semibold">{formatDateTime(next.start)}</p>
              <Link to="/painel/aulas" className="mt-auto">
                <Button variant="secondary" className="w-full">Ver minhas aulas</Button>
              </Link>
            </>
          ) : (
            <p className="text-muted">Nenhuma aula agendada.</p>
          )}
        </Card>
      </div>

      <section className="flex flex-col gap-3">
        <h2 className="text-2xl">Seus materiais</h2>
        {materials === null ? (
          <Skeleton className="h-24" />
        ) : materials.length === 0 ? (
          <EmptyState
            title="Você ainda não tem materiais"
            description="Os materiais que você comprar aparecem aqui para download."
            action={
              <Link to="/#materiais">
                <Button variant="secondary">Ver materiais</Button>
              </Link>
            }
          />
        ) : (
          <div className="grid gap-4 sm:grid-cols-2">
            {materials.map((m) => (
              <Card key={m.slug} className="flex items-center justify-between gap-3">
                <span className="font-semibold">{m.title}</span>
                <a href={`/api/v1/me/materials/${m.slug}/download`}>
                  <Button size="sm" variant="secondary">Baixar</Button>
                </a>
              </Card>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
