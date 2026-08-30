import "@ant-design/v5-patch-for-react-19";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import "@edugrade/design-tokens/tokens.css";
import "@edugrade/ui/styles.css";
import "./styles.css";
import "./features/grading/workbench/grading-workbench.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>
);
