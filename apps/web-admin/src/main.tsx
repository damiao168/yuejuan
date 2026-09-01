import "@ant-design/v5-patch-for-react-19";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { ConfigProvider } from "antd";
import zhCN from "antd/locale/zh_CN";
import App from "./App";
import "@fontsource-variable/inter/wght.css";
import "@edugrade/design-tokens/tokens.css";
import "@edugrade/ui/styles.css";
import "./styles.css";
import "./features/grading/workbench/grading-workbench.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ConfigProvider locale={zhCN}>
      <App />
    </ConfigProvider>
  </StrictMode>
);
