import { useEffect, useRef, useState } from "react";
import { Link, useSearchParams } from "react-router";
import { api, type Order } from "../../api/client";
import { Button, Card, Skeleton } from "../../design/primitives";

export function CheckoutSucesso() {
  return (
    <Result
      tone="success"
      title="Pagamento confirmado"
      body="Tudo certo. Seus créditos e materiais já estão liberados no painel."
      action={<Link to="/painel"><Button>Ir para o painel</Button></Link>}
    />
  );
}

export function CheckoutErro() {
  return (
    <Result
      tone="danger"
      title="O pagamento não foi concluído"
      body="Nada foi cobrado. Você pode tentar de novo, ou escolher outra forma de pagamento."
      action={
        <div className="flex flex-wrap gap-3">
          <Link to="/#aulas"><Button>Tentar de novo</Button></Link>
          <a href="mailto:ola@speako.com.br"><Button variant="secondary">Falar com a gente</Button></a>
        </div>
      }
    />
  );
}

/**
 * Pix confirms asynchronously, so this page polls instead of asking the student
 * to refresh. It gives up after ten minutes rather than spinning forever - an
 * endless spinner is how someone concludes they were charged for nothing.
 */
export function CheckoutPendente() {
  const [params] = useSearchParams();
  const orderId = params.get("order");
  const [state, setState] = useState<"waiting" | "paid" | "timeout">("waiting");
  const started = useRef(Date.now());

  useEffect(() => {
    if (!orderId) return;
    const TIMEOUT_MS = 10 * 60_000;

    const tick = async () => {
      if (Date.now() - started.current > TIMEOUT_MS) {
        setState("timeout");
        return;
      }
      try {
        const orders: Order[] = await api.orders();
        const mine = orders.find((o) => o.id === orderId);
        if (mine?.status === "paid") setState("paid");
      } catch {
        // A failed poll is not worth surfacing; the next one may succeed.
      }
    };

    void tick();
    const id = setInterval(tick, 5000);
    return () => clearInterval(id);
  }, [orderId]);

  if (state === "paid") return <CheckoutSucesso />;

  if (state === "timeout") {
    return (
      <Result
        tone="muted"
        title="Ainda não recebemos a confirmação"
        body="Pagamentos via Pix costumam levar alguns segundos, mas às vezes demoram mais. Assim que cair, seus créditos aparecem no painel automaticamente."
        action={
          <div className="flex flex-wrap gap-3">
            <Link to="/painel/compras"><Button>Ver minhas compras</Button></Link>
            <Button variant="secondary" onClick={() => window.location.reload()}>
              Verificar de novo
            </Button>
          </div>
        }
      />
    );
  }

  return (
    <div className="mx-auto flex w-full max-w-lg flex-col gap-5 px-5 py-20 text-center">
      <h1 className="text-3xl">Aguardando o pagamento</h1>
      <p className="text-muted">
        Assim que o Pix for compensado, liberamos tudo automaticamente. Pode
        deixar esta página aberta.
      </p>
      <Skeleton className="h-3 w-full" />
      <p className="text-sm text-subtle">Verificando a cada 5 segundos...</p>
    </div>
  );
}

function Result({
  tone,
  title,
  body,
  action,
}: {
  tone: "success" | "danger" | "muted";
  title: string;
  body: string;
  action: React.ReactNode;
}) {
  const tones = {
    success: "bg-green-50 border-green-200",
    danger: "bg-red-50 border-red-200",
    muted: "bg-surface",
  } as const;
  return (
    <div className="mx-auto flex w-full max-w-lg flex-col gap-5 px-5 py-20">
      <Card className={tones[tone]}>
        <h1 className="font-display text-2xl font-bold">{title}</h1>
        <p className="mt-3 text-muted">{body}</p>
        <div className="mt-5">{action}</div>
      </Card>
    </div>
  );
}
