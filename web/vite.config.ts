import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// The API is a separate Go service. In dev we proxy /api so cookies stay
// same-origin; in production the SPA is static on a CDN and talks to
// api.speako.com.br with credentials.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    proxy: {
      // ponytail: hardcoded because the local API port never moves; make it
      // configurable the day someone actually runs it elsewhere.
      "/api": { target: "http://localhost:8080", changeOrigin: true },
    },
  },
  build: {
    target: "es2022",
    // ponytail: no sourcemaps shipped; turn on `hidden` and upload them to the
    // error tracker the day one exists (S16).
    sourcemap: false,
  },
});
