import type { OxfmtConfig } from "vite-plus/fmt";

/**
 * app-orchestra formatting policy (Oxfmt).
 *
 * Part of the Repository Harness. Formatting is not a matter of taste here:
 * one canonical layout keeps diffs small and reviewable. Read
 * harness/quality/README.md before changing anything in this file.
 */
export const fmtPolicy: OxfmtConfig = {
  printWidth: 100,
  singleQuote: false,
  semi: true,
  trailingComma: "all",
  sortPackageJson: true,
  ignorePatterns: [
    "**/node_modules/**",
    "**/dist/**",
    "**/coverage/**",
    // Generated from services/platform/api/openapi.yaml; regenerate instead of editing.
    "**/src/shared/api/gen/**",
    // Go sources are formatted by gofmt / goimports (see harness/quality/go).
    "services/**",
    "harness/guard/archcheck/**",
    "harness/gen/**",
    // Lockfiles are machine written.
    "pnpm-lock.yaml",
  ],
};
