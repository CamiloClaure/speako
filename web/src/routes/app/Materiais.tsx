import { useEffect, useState } from "react";
import { Link } from "react-router";
import { api, friendlyMessage, type Material, type Product } from "../../api/client";
import { Button, Card, EmptyState, ErrorNote, PriceTag, Skeleton } from "../../design/primitives";

export function Materiais() {
  const [owned, setOwned] = useState<Material[] | null>(null);
  const [catalog, setCatalog] = useState<Product[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([api.materials(), api.products()])
      .then(([m, p]) => {
        setOwned(m);
        setCatalog(p.filter((x) => x.kind === "material"));
      })
      .catch((e) => setError(friendlyMessage(e)));
  }, []);

  const ownedSlugs = new Set((owned ?? []).map((m) => m.slug));
  const locked = (catalog ?? []).filter((p) => !ownedSlugs.has(p.slug));

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-8 px-5 py-10">
      <h1 className="text-3xl">Meus materiais</h1>
      {error && <ErrorNote>{error}</ErrorNote>}

      {owned === null ? (
        <Skeleton className="h-28" />
      ) : owned.length === 0 ? (
        <EmptyState
          title="Você ainda não tem materiais"
          description="Os materiais que você comprar ficam disponíveis aqui para sempre."
        />
      ) : (
        <section className="flex flex-col gap-3">
          {owned.map((m) => (
            <Card key={m.slug} className="flex flex-wrap items-center justify-between gap-3">
              <span className="font-semibold">{m.title}</span>
              {/* A plain link, not fetch: the API answers with a 302 to a signed URL. */}
              <a href={`/api/v1/me/materials/${m.slug}/download`}>
                <Button size="sm">Baixar PDF</Button>
              </a>
            </Card>
          ))}
        </section>
      )}

      {locked.length > 0 && (
        <section className="flex flex-col gap-3">
          <h2 className="text-xl">Disponíveis para compra</h2>
          {locked.map((p) => (
            <Card key={p.slug} className="flex flex-wrap items-center justify-between gap-4">
              <div>
                <p className="font-semibold">{p.title}</p>
                <PriceTag cents={p.price_cents} installments={false} />
              </div>
              <Link to={`/#materiais`}>
                <Button size="sm" variant="secondary">Comprar</Button>
              </Link>
            </Card>
          ))}
        </section>
      )}
    </div>
  );
}
