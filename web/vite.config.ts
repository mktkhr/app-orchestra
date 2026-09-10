import { fileURLToPath } from "node:url";

import react from "@vitejs/plugin-react";
import { defineConfig, lazyPlugins } from "vite-plus";

/**
 * Frontend application configuration: dev server, production build and unit tests.
 *
 * Lint, format and staged-file policies are NOT defined here. They live in the
 * root vite.config.ts and quality/ so that they apply to every package and are
 * reviewed as part of the Repository Harness rather than as a feature change.
 */
export default defineConfig({
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: "dist",
    sourcemap: true,
  },
  test: {
    environment: "happy-dom",
    include: ["src/**/*.test.{ts,tsx}"],
    setupFiles: ["src/test/setup.ts"],
    globals: false,
    restoreMocks: true,
  },
  plugins: lazyPlugins(() => [react()]) ?? [],
});
