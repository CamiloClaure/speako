import { useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { api, friendlyMessage } from "../../api/client";
import { useSession } from "../../app/session";
import { Button, Card, ErrorNote, Field } from "../../design/primitives";

/**
 * Sign in and sign up are the same door: submitting an address that has no
 * account creates one when the link is redeemed.
 */
export function Entrar() {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const { refresh, user } = useSession();

  const token = params.get("token");
  const [email, setEmail] = useState("");
  const [state, setState] = useState<"idle" | "sending" | "sent" | "verifying">(
    token ? "verifying" : "idle",
  );
  const [error, setError] = useState<string | null>(null);

  // Arriving with a token in the URL means the student clicked the email link.
  useEffect(() => {
    if (!token) return;
    api
      .verifyMagicLink(token)
      .then(refresh)
      .then(() => navigate("/painel", { replace: true }))
      .catch((err) => {
        setError(
          "Este link expirou ou já foi usado. Peça um novo abaixo - leva um minuto.",
        );
        setState("idle");
        console.error(err);
      });
  }, [token, refresh, navigate]);

  useEffect(() => {
    if (user && !token) navigate("/painel", { replace: true });
  }, [user, token, navigate]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setState("sending");
    try {
      await api.requestMagicLink(email);
      setState("sent");
    } catch (err) {
      setError(friendlyMessage(err));
      setState("idle");
    }
  }

  return (
    <div className="mx-auto flex w-full max-w-md flex-col gap-6 px-5 py-16">
      <h1 className="text-3xl">Entrar</h1>

      {state === "verifying" ? (
        <Card>
          <p className="text-muted">Confirmando seu acesso...</p>
        </Card>
      ) : state === "sent" ? (
        <Card className="flex flex-col gap-3">
          <h2 className="font-display text-xl font-bold">Verifique seu e-mail</h2>
          <p className="text-muted">
            Enviamos um link de acesso para <strong>{email}</strong>. Ele vale por
            15 minutos e só pode ser usado uma vez.
          </p>
          <Button variant="ghost" onClick={() => setState("idle")}>
            Usar outro e-mail
          </Button>
        </Card>
      ) : (
        <Card>
          <form onSubmit={submit} className="flex flex-col gap-5">
            <Field
              label="Seu e-mail"
              type="email"
              required
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              hint="Enviamos um link de acesso. Sem senha para lembrar."
            />
            {error && <ErrorNote>{error}</ErrorNote>}
            <Button type="submit" loading={state === "sending"}>
              Receber link de acesso
            </Button>
          </form>
        </Card>
      )}

      <p className="text-center text-sm text-subtle">
        Primeira vez aqui? Use o mesmo formulário - sua conta é criada ao abrir o link.
      </p>
    </div>
  );
}
