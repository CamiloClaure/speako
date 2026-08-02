import { BrowserRouter, Link, Navigate, Route, Routes, useLocation } from "react-router";
import type { ReactNode } from "react";
import { SessionProvider, useSession } from "./app/session";
import { Landing } from "./routes/public/Landing";
import { Entrar } from "./routes/public/Entrar";
import { Painel } from "./routes/app/Painel";
import { Agendar } from "./routes/app/Agendar";
import { Aulas } from "./routes/app/Aulas";
import { Creditos } from "./routes/app/Creditos";
import { Materiais } from "./routes/app/Materiais";
import { Compras } from "./routes/app/Compras";
import { Termos, Privacidade } from "./routes/public/Legal";
import { CheckoutSucesso, CheckoutPendente, CheckoutErro } from "./routes/checkout/Checkout";
import { AdminAgenda, AdminAulas, AdminProdutos, AdminVendas } from "./routes/admin/Admin";
import { Button, Skeleton } from "./design/primitives";

export function App() {
  return (
    <BrowserRouter>
      <SessionProvider>
        <div className="flex min-h-dvh flex-col">
          <Header />
          <main className="flex-1">
            <Routes>
              <Route path="/" element={<Landing />} />
              <Route path="/entrar" element={<Entrar />} />
              <Route path="/painel" element={<Protected><Painel /></Protected>} />
              <Route path="/painel/agendar" element={<Protected><Agendar /></Protected>} />
              <Route path="/painel/aulas" element={<Protected><Aulas /></Protected>} />
              <Route path="/painel/creditos" element={<Protected><Creditos /></Protected>} />
              <Route path="/painel/materiais" element={<Protected><Materiais /></Protected>} />
              <Route path="/painel/compras" element={<Protected><Compras /></Protected>} />
              <Route path="/checkout/sucesso" element={<CheckoutSucesso />} />
              <Route path="/checkout/pendente" element={<CheckoutPendente />} />
              <Route path="/checkout/erro" element={<CheckoutErro />} />
              <Route path="/admin/agenda" element={<TeacherOnly><AdminAgenda /></TeacherOnly>} />
              <Route path="/admin/aulas" element={<TeacherOnly><AdminAulas /></TeacherOnly>} />
              <Route path="/admin/produtos" element={<TeacherOnly><AdminProdutos /></TeacherOnly>} />
              <Route path="/admin/vendas" element={<TeacherOnly><AdminVendas /></TeacherOnly>} />
              <Route path="/termos" element={<Termos />} />
              <Route path="/privacidade" element={<Privacidade />} />
              <Route path="*" element={<NotFound />} />
            </Routes>
          </main>
          <Footer />
        </div>
      </SessionProvider>
    </BrowserRouter>
  );
}

/**
 * Route guard. This is convenience, not security - every protected endpoint
 * checks the session itself, and the cookie is HttpOnly.
 */
function Protected({ children }: { children: ReactNode }) {
  const { user, loading } = useSession();
  const location = useLocation();

  if (loading) {
    return (
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-4 px-5 py-10">
        <Skeleton className="h-10 w-56" />
        <Skeleton className="h-32" />
      </div>
    );
  }
  if (!user) {
    // Remember where they were headed so sign-in can return them there.
    return <Navigate to="/entrar" replace state={{ from: location.pathname }} />;
  }
  return <>{children}</>;
}

/**
 * Teacher-only routes. Like Protected, this is convenience only - the API
 * refuses /api/v1/admin/* for a student regardless of what renders here.
 */
function TeacherOnly({ children }: { children: ReactNode }) {
  const { user, loading } = useSession();
  if (loading) {
    return (
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-4 px-5 py-10">
        <Skeleton className="h-10 w-56" />
        <Skeleton className="h-32" />
      </div>
    );
  }
  if (!user) return <Navigate to="/entrar" replace />;
  if (user.role !== "teacher" && user.role !== "admin") return <Navigate to="/painel" replace />;
  return (
    <div className="flex flex-col">
      <nav className="border-b-2 border-line bg-surface">
        <div className="mx-auto flex w-full max-w-5xl gap-1 overflow-x-auto px-5 py-2">
          {([
            ["/admin/agenda", "Agenda"],
            ["/admin/aulas", "Aulas"],
            ["/admin/produtos", "Produtos"],
            ["/admin/vendas", "Vendas"],
          ] as const).map(([to, label]) => (
            <Link key={to} to={to} className="whitespace-nowrap rounded-md px-3 py-2 font-semibold text-muted hover:bg-canvas hover:text-ink">
              {label}
            </Link>
          ))}
        </div>
      </nav>
      {children}
    </div>
  );
}

function Header() {
  const { user, signOut } = useSession();
  return (
    <header className="sticky top-0 z-40 border-b-2 border-line bg-canvas/95 backdrop-blur">
      <div className="mx-auto flex w-full max-w-5xl items-center justify-between gap-4 px-5 py-3">
        <Link to="/" className="font-display text-2xl font-bold tracking-tight">
          Speako
        </Link>
        <nav className="flex items-center gap-2">
          {user ? (
            <>
              <Link to="/painel" className="px-3 py-2 font-semibold text-muted hover:text-ink">
                Painel
              </Link>
              <Link to="/painel/aulas" className="px-3 py-2 font-semibold text-muted hover:text-ink">
                Aulas
              </Link>
              {(user.role === "teacher" || user.role === "admin") && (
                <Link to="/admin/agenda" className="px-3 py-2 font-semibold text-brand-700 hover:text-brand-800">
                  Admin
                </Link>
              )}
              <Button size="sm" variant="ghost" onClick={() => void signOut()}>
                Sair
              </Button>
            </>
          ) : (
            <Link to="/entrar">
              <Button size="sm">Entrar</Button>
            </Link>
          )}
        </nav>
      </div>
    </header>
  );
}

function Footer() {
  return (
    <footer className="border-t-2 border-line bg-surface">
      <div className="mx-auto flex w-full max-w-5xl flex-wrap items-center justify-between gap-3 px-5 py-8 text-sm text-subtle">
        <span>© {new Date().getFullYear()} Speako</span>
        <nav className="flex gap-4">
          <Link to="/termos" className="hover:text-ink">Termos</Link>
          <Link to="/privacidade" className="hover:text-ink">Privacidade</Link>
        </nav>
      </div>
    </footer>
  );
}

function NotFound() {
  return (
    <div className="mx-auto flex w-full max-w-lg flex-col items-center gap-4 px-5 py-24 text-center">
      <h1 className="text-4xl">Página não encontrada</h1>
      <p className="text-muted">O link que você seguiu não existe mais.</p>
      <Link to="/">
        <Button>Voltar ao início</Button>
      </Link>
    </div>
  );
}
