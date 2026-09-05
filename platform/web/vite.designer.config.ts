import { defineConfig } from "vite";
import { resolve } from "node:path";
import base from "./vite.config";
// Build a standalone child graph; sharing entry chunks with the host can stall
// module imports inside micro-app's iframe sandbox.
export default defineConfig({
  ...base,
  build: {
    emptyOutDir: false,
    rollupOptions: {
      input: { designer: resolve(import.meta.dirname, "designer/index.html") },
      output: {
        entryFileNames: "designer/assets/[name]-[hash].js",
        chunkFileNames: "designer/assets/[name]-[hash].js",
        assetFileNames: "designer/assets/[name]-[hash][extname]",
      },
    },
  },
});
