import { defineConfig } from "vite-plus";

/**
 * End-to-end tests start the built backend binary serving the built frontend
 * and talk to it over real TCP. They need `make build` to have run first.
 */
export default defineConfig({
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
    globals: false,
    testTimeout: 30_000,
    hookTimeout: 30_000,
  },
});
