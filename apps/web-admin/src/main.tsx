import "@ant-design/v5-patch-for-react-19";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import { ErrorBoundary } from "./components/ErrorBoundary";
import "./styles.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ErrorBoundary
      fallback={
        <div style={{ padding: 48, textAlign: "center" }}>
          <p>应用出现异常，请重新加载页面后再试。</p>
          <button type="button" onClick={() => window.location.reload()}>
            重新加载
          </button>
        </div>
      }
    >
      <App />
    </ErrorBoundary>
  </StrictMode>
);
