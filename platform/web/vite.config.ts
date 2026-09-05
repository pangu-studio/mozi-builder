import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { resolve } from "node:path";
export default defineConfig({
  plugins: [react()],
  resolve: {
    dedupe: [
      "react",
      "react-dom",
      "react-router-dom",
      "antd",
      "@ant-design/icons",
      "zustand",
      "axios",
    ],
  },
  server: {
    proxy: { "/api/v2": "http://127.0.0.1:15180" },
    fs: { allow: [resolve(import.meta.dirname, "../..")] },
  },
  build: {
    rollupOptions: {
      input: {
        lab: resolve(import.meta.dirname, "lab.html"),
        console: resolve(import.meta.dirname, "index.html"),
        designer: resolve(import.meta.dirname, "designer/index.html"),
      },
    },
  },
});
