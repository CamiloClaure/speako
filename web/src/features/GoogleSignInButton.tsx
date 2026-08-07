import { useEffect, useRef } from "react";
import { api } from "../api/client";

/**
 * "Sign in with Google" button. Renders nothing at all when no client ID is
 * configured - docs/design-system.md bans third-party CDNs, so the Google
 * script must never load on a deployment that has Google sign-in off.
 */

const CLIENT_ID: string | undefined = import.meta.env.VITE_GOOGLE_CLIENT_ID;
const SCRIPT_SRC = "https://accounts.google.com/gsi/client";

// ponytail: declared locally instead of pulling in @types/google.accounts -
// this is the only shape the component touches.
declare global {
  interface Window {
    google?: {
      accounts: {
        id: {
          initialize(config: {
            client_id: string;
            callback: (response: { credential: string }) => void;
          }): void;
          renderButton(
            parent: HTMLElement,
            options: { theme?: string; size?: string; text?: string; shape?: string },
          ): void;
        };
      };
    };
  }
}

let scriptPromise: Promise<void> | null = null;

/** Injects the GSI script once, however many times this component mounts. */
function loadGsiScript(): Promise<void> {
  if (window.google?.accounts?.id) return Promise.resolve();
  if (!scriptPromise) {
    scriptPromise = new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.src = SCRIPT_SRC;
      script.async = true;
      script.defer = true;
      script.onload = () => resolve();
      script.onerror = () => reject(new Error("failed to load Google sign-in"));
      document.head.appendChild(script);
    });
  }
  return scriptPromise;
}

export function GoogleSignInButton({
  onSuccess,
  onError,
}: {
  onSuccess?: () => void;
  onError?: (err: unknown) => void;
}) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!CLIENT_ID) return;
    let cancelled = false;

    loadGsiScript()
      .then(() => {
        if (cancelled || !ref.current || !window.google) return;
        window.google.accounts.id.initialize({
          client_id: CLIENT_ID,
          callback: (response) => {
            api
              .googleSignIn(response.credential)
              .then(() => onSuccess?.())
              .catch((err) => onError?.(err));
          },
        });
        window.google.accounts.id.renderButton(ref.current, {
          theme: "outline",
          size: "large",
          text: "continue_with",
          shape: "pill",
        });
      })
      .catch((err) => onError?.(err));

    return () => {
      cancelled = true;
    };
  }, [onSuccess, onError]);

  if (!CLIENT_ID) return null;

  return <div ref={ref} className="flex justify-center" />;
}
