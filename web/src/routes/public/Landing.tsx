import { useEffect, useState } from "react";
import { Link } from "react-router";
import { api, friendlyMessage, type Product } from "../../api/client";
import { Badge, Button, Card, ErrorNote, Field, PriceTag, Skeleton } from "../../design/primitives";

/**
 * The page that sells. Order follows the pattern that puts proof before the
 * ask: promise, how it works, what is on sale, FAQ, then the CTA.
 */
export function Landing() {
  const [products, setProducts] = useState<Product[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .products()
      .then(setProducts)
      .catch((e) => setError(friendlyMessage(e)));
  }, []);

  const packs = products?.filter((p) => p.kind === "pack") ?? [];
  const materials = products?.filter((p) => p.kind === "material") ?? [];

  return (
    <div className="flex flex-col">
      <Hero />
      <HowItWorks />

      {/* The one section that breaks the column. Everything else on this page
          sat at max-w-5xl with identical padding, which is what made the page
          read as a stack of interchangeable slabs. The packs are what the page
          is selling, so this is the one that gets to be wider. */}
      <section id="aulas" className="mx-auto w-full max-w-6xl px-5 py-20">
        <h2 className="text-3xl">Pacotes de aulas particulares</h2>
        <p className="mt-2 text-muted">
          Compre um pacote, agende no horário que quiser. Os créditos são seus.
        </p>

        {error && <div className="mt-6"><ErrorNote>{error}</ErrorNote></div>}

        <div className="mt-8 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
          {products === null && !error
            ? [0, 1, 2].map((i) => <Skeleton key={i} className="h-52" />)
            : packs.map((p) => <PackCard key={p.slug} product={p} />)}
        </div>
      </section>

      <section id="materiais" className="bg-surface">
        <div className="mx-auto w-full max-w-5xl px-5 py-12">
          <h2 className="text-3xl">Materiais</h2>
          <p className="mt-2 text-muted">Para estudar sozinho, no seu ritmo.</p>
          <div className="mt-8 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
            {products === null && !error
              ? [0, 1].map((i) => <Skeleton key={i} className="h-40" />)
              : materials.map((p) => <MaterialCard key={p.slug} product={p} />)}
          </div>
        </div>
      </section>

      <LeadMagnet />
      <Faq />
    </div>
  );
}

function Hero() {
  return (
    <section className="mx-auto w-full max-w-5xl px-5 pb-12 pt-16 sm:pt-24">
      <Badge tone="accent">aulas 1:1 com professora de verdade</Badge>
      <h1 className="mt-5 max-w-3xl text-4xl leading-tight sm:text-6xl">
        Você já estudou inglês. Agora vamos fazer você <em className="not-italic text-brand-600">falar</em>.
      </h1>
      <p className="mt-5 max-w-xl text-lg text-muted">
        Aulas particulares por videochamada, no seu horário, com material feito
        para brasileiro. Sem turma, sem enrolação, sem contrato de fidelidade.
      </p>
      <div className="mt-8 flex flex-wrap gap-3">
        <Button size="lg" onClick={() => document.getElementById("aulas")?.scrollIntoView({ behavior: "smooth" })}>
          Ver pacotes de aulas
        </Button>
        <Link to="/entrar">
          <Button size="lg" variant="secondary">Já sou aluno</Button>
        </Link>
      </div>
    </section>
  );
}

/**
 * Three equal columns, each a numbered circle above a heading above two lines,
 * is the icon-tile card with a digit where the icon goes. The content really is
 * ordinal, so the numbering stays - what goes is the card shape and the equal
 * thirds. It reads as a sequence now, which is what it always was.
 *
 * The number sits inline with the heading rather than in a column of its own:
 * a label in a narrow left column with the title beside it is its own tell.
 */
function HowItWorks() {
  const steps = [
    ["Escolha seu pacote", "4, 8 ou 12 aulas. Pague com Pix ou cartão parcelado."],
    ["Agende quando quiser", "Você vê a agenda real da professora e escolhe o horário."],
    ["Entre na chamada", "O link chega junto com a confirmação. Só isso."],
  ];
  return (
    <section className="bg-brand-50">
      <div className="mx-auto w-full max-w-4xl px-5 py-16">
        <ol className="flex flex-col">
          {steps.map(([title, body], i) => (
            <li
              key={title}
              className="border-t-2 border-brand-200 py-6 first:border-t-0 first:pt-0"
            >
              <h3 className="flex items-baseline gap-3 font-display text-xl font-bold">
                <span className="text-3xl text-brand-600 tabular-nums">{i + 1}</span>
                {title}
              </h3>
              {/* Indented to the heading's text, not to the number - the body
                  belongs to the title, not to the digit. */}
              <p className="mt-1 max-w-xl text-muted sm:ml-11">{body}</p>
            </li>
          ))}
        </ol>
      </div>
    </section>
  );
}

function PackCard({ product }: { product: Product }) {
  return (
    <Card className="flex flex-col gap-4">
      <div>
        <h3 className="font-display text-xl font-bold">{product.title}</h3>
        <p className="mt-1 text-sm text-muted">
          {product.credits} aulas de 60 min
          {product.validity_days ? ` · válido por ${product.validity_days} dias` : ""}
        </p>
      </div>
      <PriceTag cents={product.price_cents} />
      <Link to={`/entrar?comprar=${product.slug}`} className="mt-auto">
        <Button className="w-full">Comprar</Button>
      </Link>
    </Card>
  );
}

function MaterialCard({ product }: { product: Product }) {
  return (
    <Card className="flex flex-col gap-4">
      <h3 className="font-display text-lg font-bold">{product.title}</h3>
      <PriceTag cents={product.price_cents} installments={false} />
      <Link to={`/entrar?comprar=${product.slug}`} className="mt-auto">
        <Button variant="secondary" className="w-full">Comprar</Button>
      </Link>
    </Card>
  );
}

function LeadMagnet() {
  const [email, setEmail] = useState("");
  const [consent, setConsent] = useState(false);
  const [state, setState] = useState<"idle" | "sending" | "sent">("idle");
  const [error, setError] = useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setState("sending");
    try {
      await api.captureLead(email, "landing-gratis");
      setState("sent");
    } catch (err) {
      setError(friendlyMessage(err));
      setState("idle");
    }
  }

  return (
    <section className="mx-auto w-full max-w-3xl px-5 py-24">
      <Card className="flex flex-col gap-4">
        <h2 className="text-2xl">Guia grátis: 50 erros que todo brasileiro comete</h2>
        {state === "sent" ? (
          <p className="text-muted">
            Enviamos um e-mail de confirmação. Clique no link para receber o guia.
          </p>
        ) : (
          <form onSubmit={submit} className="flex flex-col gap-4">
            <Field
              label="Seu melhor e-mail"
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              autoComplete="email"
            />
            {/* Never pre-checked: consent has to be an action, not a default. */}
            <label className="flex items-start gap-3 text-sm text-muted">
              <input
                type="checkbox"
                required
                checked={consent}
                onChange={(e) => setConsent(e.target.checked)}
                className="mt-1 size-5 shrink-0"
              />
              <span>
                Autorizo a Speako a enviar o material e e-mails sobre as aulas. Posso
                cancelar quando quiser. Veja a{" "}
                <Link to="/privacidade" className="font-semibold text-brand-700 underline">
                  política de privacidade
                </Link>
                .
              </span>
            </label>
            {error && <ErrorNote>{error}</ErrorNote>}
            <Button type="submit" loading={state === "sending"} disabled={!consent}>
              Quero o guia
            </Button>
          </form>
        )}
      </Card>
    </section>
  );
}

function Faq() {
  const items = [
    ["Preciso ter nível avançado?", "Não. As aulas são montadas a partir do seu nível atual, do zero ao avançado."],
    ["Como funciona o cancelamento?", "Cancelou com 24h ou mais de antecedência, o crédito volta para você. Com menos de 24h, o crédito é consumido."],
    ["Os créditos expiram?", "Sim, cada pacote tem um prazo de validade. Ele aparece no seu painel e avisamos antes de vencer."],
    ["Qual plataforma de vídeo?", "Google Meet. O link vem junto com a confirmação do agendamento."],
  ];
  return (
    <section className="bg-surface">
      <div className="mx-auto w-full max-w-3xl px-5 py-14">
        <h2 className="text-3xl">Perguntas frequentes</h2>
        <div className="mt-6 flex flex-col gap-3">
          {items.map(([q, a]) => (
            <details key={q} className="rounded-lg border-2 border-line bg-canvas p-4">
              <summary className="cursor-pointer font-semibold">{q}</summary>
              <p className="mt-2 text-muted">{a}</p>
            </details>
          ))}
        </div>
      </div>
    </section>
  );
}
