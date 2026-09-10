import { fileURLToPath } from "node:url";

import react from "@vitejs/plugin-react";
import { defineConfig } from "vite-plus";

/**
 * Frontend acceptance tests render the real application root against a
 * scripted API. They resolve the application's "@/" alias to its source tree.
 */
export default defineConfig({
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("../../web/src", import.meta.url)),
    },
  },
  test: {
    environment: "happy-dom",
    include: ["src/**/*.test.{ts,tsx}"],
    setupFiles: ["src/setup.ts"],
    globals: false,
    restoreMocks: true,
  },
  plugins: [react()],
});
