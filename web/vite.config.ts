import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 8080,
    strictPort: true,
    proxy: {
      "/api": {
        target: process.env.VITE_API_PROXY_TARGET ?? "http://localhost:8081",
        changeOrigin: true,
        ws: true,
        cookieDomainRewrite: "localhost",
      },
      "/debug": {
        target: process.env.VITE_API_PROXY_TARGET ?? "http://localhost:8081",
        changeOrigin: true,
        cookieDomainRewrite: "localhost",
      },
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
    target: "es2022",
    sourcemap: false,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes("node_modules")) {
            return;
          }
          if (id.includes("recharts")) {
            return "recharts";
          }
          if (id.includes("@tanstack/react-query")) {
            return "query";
          }
          if (id.includes("motion")) {
            return "motion";
          }
          if (id.includes("@msgpack/msgpack") || id.includes("cbor-x")) {
            return "codecs";
          }
          if (
            id.includes("/react/") ||
            id.includes("/react-dom/") ||
            id.includes("/react-router/") ||
            id.includes("\\react\\") ||
            id.includes("\\react-dom\\") ||
            id.includes("\\react-router\\")
          ) {
            return "vendor";
          }
        },
      },
    },
  },
});
