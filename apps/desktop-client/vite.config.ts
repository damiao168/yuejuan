import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react()],
  clearScreen: false,
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          antd: ["antd"],
          motion: ["framer-motion"]
        }
      }
    }
  },
  server: {
    host: "127.0.0.1",
    port: 5180,
    strictPort: true
  }
});
