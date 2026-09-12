import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import "@edugrade/design-tokens/tokens.css";
import "./styles.css";
import "./experience.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>
);
