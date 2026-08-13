import { QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { appQueryClient } from "./query/client";
import { appRouter } from "./router/appRouter";

function App() {
  return (
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
      <QueryClientProvider client={appQueryClient}>
        <RouterProvider router={appRouter} />
      </QueryClientProvider>
    </ErrorBoundary>
  );
}

export default App;
