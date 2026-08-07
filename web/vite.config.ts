import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// The API is a separate Go service. In dev we proxy /api so cookies stay
// same-origin; in production the SPA is static on a CDN and talks to
// api.speako.com.br with credentials.
// Only `vite/client` is in tsconfig's `types`, so declare the one Node global
// we read rather than pulling @types/node in for two lookups.
declare const process: { env: Record<string, string | undefined> };

// dev-tunnel.sh picks whatever ports are free and passes them down, so neither
// side can be hardcoded: a tunnel aimed at 5173 while Vite sat elsewhere would
// serve the wrong process, and a proxy aimed at 8080 while the API sat
// elsewhere would answer every call with a connection refused.
const webPort = Number(process.env.SPEAKO_WEB_PORT ?? 5173);
const apiPort = Number(process.env.SPEAKO_API_PORT ?? 8080);

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: webPort,
    // Fail rather than drift to the next free port. Vite's default hunt would
    // leave the tunnel pointing at nothing.
    strictPort: true,
    // A cloudflared quick tunnel arrives with a random *.trycloudflare.com
    // Host header, which Vite rejects by default.
    allowedHosts: [".trycloudflare.com"],
    proxy: {
      "/api": { target: `http://localhost:${apiPort}`, changeOrigin: true },
    },
  },
  build: {
    target: "es2022",
    // ponytail: no sourcemaps shipped; turn on `hidden` and upload them to the
    // error tracker the day one exists (S16).
    sourcemap: false,
  },
});
