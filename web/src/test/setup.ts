import { cleanup } from "@testing-library/react";
import { afterEach } from "vite-plus/test";

// Unmount every rendered component between tests so state from one test
// cannot leak into the next.
afterEach(() => {
  cleanup();
});
