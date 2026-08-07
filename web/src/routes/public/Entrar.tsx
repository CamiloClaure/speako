import { useEffect, useState, type FormEvent, type ReactNode } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { api, friendlyMessage, type User } from "../../api/client";
import { useSession } from "../../app/session";
import { Button, Card, ErrorNote, Field } from "../../design/primitives";
import { GoogleSignInButton } from "../../features/GoogleSignInButton";

type Mode = "link" | "login" | "register";

/** A teacher's API calls all 403 on student endpoints, so she can't land on /painel. */
function landingPath(role: User["role"]): string {
  return role === "teacher" || role === "admin" ? "/admin/agenda" : "/painel";
}

/**
 * Three doors to the same session: a one-time link (which is also the
 * password-recovery path - there is deliberately no separate "forgot
 * password" flow), an email + password sign-in, and account creation.
 */
export function Entrar() {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const { refresh, user } = useSession();

  const token = params.get("token");
  const [mode, setMode] = useState<Mode>("link");

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [name, setName] = useState("");

  const [linkState, setLinkState] = useState<"idle" | "sending" | "sent" | "verifying">(
    token ? "verifying" : "idle",
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Arriving with a token in the URL means the student clicked the email link.
  useEffect(() => {
    if (!token) return;
    api
      .verifyMagicLink(token)
      .then(() => refresh())
      .then((signedIn) => {
        if (!signedIn) {
          // The magic link verified, but we could not confirm who signed in
          // (a transient /me failure) - landing on a dashboard would be a
          // guess about her role. Stay here and let her retry.
          setError("Não foi possível confirmar seu acesso. Tente novamente.");
          setLinkState("idle");
          return;
        }
        navigate(landingPath(signedIn.role), { replace: true });
      })
      .catch((err) => {
        setError(
          "Este link expirou ou já foi usado. Peça um novo abaixo - leva um minuto.",
        );
        setLinkState("idle");
        console.error(err);
      });
  }, [token, refresh, navigate]);

  // Login, register and Google all just update `user` via refresh() - this is
  // the one place that decides where a freshly signed-in session lands.
  useEffect(() => {
    if (user && !token) navigate(landingPath(user.role), { replace: true });
  }, [user, token, navigate]);

  function switchMode(next: Mode) {
    setMode(next);
    setError(null);
  }

  async function submitLink(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setLinkState("sending");
    try {
      await api.requestMagicLink(email);
      setLinkState("sent");
    } catch (err) {
      setError(friendlyMessage(err));
      setLinkState("idle");
    }
  }

  async function submitLogin(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      await api.login(email, password);
      await refresh();
    } catch (err) {
      setError(friendlyMessage(err));
    } finally {
      setBusy(false);
    }
  }

  async function submitRegister(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      await api.register(email, name, password);
      await refresh();
    } catch (err) {
      setError(friendlyMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mx-auto flex w-full max-w-md flex-col gap-6 px-5 py-16">
      <h1 className="text-3xl">Entrar</h1>

      {linkState === "verifying" ? (
        <Card>
          <p className="text-muted">Confirmando seu acesso...</p>
        </Card>
      ) : linkState === "sent" ? (
        <Card className="flex flex-col gap-3">
          <h2 className="font-display text-xl font-bold">Verifique seu e-mail</h2>
          <p className="text-muted">
            Enviamos um link de acesso para <strong>{email}</strong>. Ele vale por
            15 minutos e só pode ser usado uma vez.
          </p>
          <Button variant="ghost" onClick={() => setLinkState("idle")}>
            Usar outro e-mail
          </Button>
        </Card>
      ) : (
        <>
          <GoogleSignInButton onError={(err) => setError(friendlyMessage(err))} />

          <div className="flex gap-1 rounded-lg border-2 border-line p-1">
            <ModeTab active={mode === "link"} onClick={() => switchMode("link")}>
              Link mágico
            </ModeTab>
            <ModeTab active={mode === "login"} onClick={() => switchMode("login")}>
              Senha
            </ModeTab>
            <ModeTab active={mode === "register"} onClick={() => switchMode("register")}>
              Criar conta
            </ModeTab>
          </div>

          <Card>
            {mode === "link" && (
              <form onSubmit={submitLink} className="flex flex-col gap-5">
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
                <Button type="submit" loading={linkState === "sending"}>
                  Receber link de acesso
                </Button>
              </form>
            )}

            {mode === "login" && (
              <form onSubmit={submitLogin} className="flex flex-col gap-5">
                <Field
                  label="Seu e-mail"
                  type="email"
                  required
                  autoComplete="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
                <Field
                  label="Sua senha"
                  type="password"
                  required
                  autoComplete="current-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  hint="Esqueceu a senha? Peça um link de acesso por e-mail - ele também entra na sua conta."
                />
                {error && <ErrorNote>{error}</ErrorNote>}
                <Button type="submit" loading={busy}>
                  Entrar
                </Button>
              </form>
            )}

            {mode === "register" && (
              <form onSubmit={submitRegister} className="flex flex-col gap-5">
                <Field
                  label="Seu nome"
                  required
                  autoComplete="name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                />
                <Field
                  label="Seu e-mail"
                  type="email"
                  required
                  autoComplete="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
                <Field
                  label="Crie uma senha"
                  type="password"
                  required
                  autoComplete="new-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                />
                {error && <ErrorNote>{error}</ErrorNote>}
                <Button type="submit" loading={busy}>
                  Criar conta
                </Button>
              </form>
            )}
          </Card>
        </>
      )}

      <p className="text-center text-sm text-subtle">
        Primeira vez aqui? Use o link mágico ou crie uma conta com senha - como preferir.
      </p>
    </div>
  );
}

function ModeTab({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex-1 rounded-md px-3 py-2 text-sm font-semibold transition-colors ${
        active ? "bg-brand-600 text-white" : "text-muted hover:bg-surface"
      }`}
    >
      {children}
    </button>
  );
}
