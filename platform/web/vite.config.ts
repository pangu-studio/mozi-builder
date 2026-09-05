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
  server: { fs: { allow: [resolve(import.meta.dirname, "../..")] } },
  build: {
    rollupOptions: {
      input: {
        console: resolve(import.meta.dirname, "index.html"),
        designer: resolve(import.meta.dirname, "designer/index.html"),
      },
    },
  },
});
