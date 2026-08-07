import { createContext, useCallback, useContext, useEffect, useState } from "react";
import type { ReactNode } from "react";
import { ApiError, api, type User } from "../api/client";

/**
 * Session state for the whole SPA. One /me call on load decides whether the
 * member area renders or the visitor is sent to sign in; there is no token in
 * JavaScript, the cookie is HttpOnly and the server is the authority.
 */

type SessionState = {
  user: User | null;
  loading: boolean;
  refresh: () => Promise<User | null>;
  signOut: () => Promise<void>;
};

const SessionContext = createContext<SessionState | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    try {
      const fetched = await api.me();
      setUser(fetched);
      return fetched;
    } catch (err) {
      // 401 is the ordinary "not signed in" answer, not a failure worth showing.
      if (!(err instanceof ApiError && err.code === "unauthenticated")) {
        console.error("session lookup failed", err);
      }
      setUser(null);
      return null;
    } finally {
      setLoading(false);
    }
  }, []);

  const signOut = useCallback(async () => {
    try {
      await api.logout();
    } finally {
      setUser(null);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return (
    <SessionContext.Provider value={{ user, loading, refresh, signOut }}>
      {children}
    </SessionContext.Provider>
  );
}

export function useSession(): SessionState {
  const ctx = useContext(SessionContext);
  if (!ctx) throw new Error("useSession must be used inside SessionProvider");
  return ctx;
}
