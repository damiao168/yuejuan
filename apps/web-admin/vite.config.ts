import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          antd: ["antd"],
          charts: ["recharts"],
          motion: ["framer-motion"]
        }
      }
    }
  },
  server: {
    port: 5173
  },
  preview: {
    port: 4173
  }
});
