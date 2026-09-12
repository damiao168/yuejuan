import "@ant-design/v5-patch-for-react-19";
import "antd/dist/reset.css";
import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "@edugrade/design-tokens/tokens.css";
import "./styles.css";
import "./experience.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
