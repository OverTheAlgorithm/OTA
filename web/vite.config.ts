import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "path";

export default defineConfig(({ mode }) => ({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
      "@wizletter/shared": path.resolve(__dirname, "../packages/shared/src"),
    },
  },
  build: {
    sourcemap: false,
  },
  esbuild: {
    // 프로덕션 빌드에서 console.* 및 debugger 구문 완전 제거
    drop: mode === "production" ? ["console", "debugger"] : [],
  },
  server: {
    proxy: {
      "/api": "http://localhost:8080",
    },
  },
}));
