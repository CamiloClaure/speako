import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
// Latin subsets only - pt-BR needs no devanagari or vietnamese glyphs.
import "@fontsource-variable/plus-jakarta-sans/wght.css";
import "@fontsource-variable/bricolage-grotesque/wght.css";
import "./design/tokens.css";
import { App } from "./App";

const root = document.getElementById("root");
if (!root) throw new Error("#root not found");

createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
