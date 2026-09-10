import type { OxlintConfig } from "vite-plus/lint";

/**
 * app-orchestra lint policy (Oxlint, type aware via tsgolint).
 *
 * Part of the Repository Harness. It is deliberately strict and error-only:
 * warnings are not a workflow an autonomous agent can be trusted with, so
 * every enabled rule is an error and CI treats a warning as a failure too.
 * Read harness/quality/README.md before changing anything in this file.
 *
 * Layer boundaries (Feature-Sliced Design) are enforced separately by
 * harness/guard/fsd.ts because they need more context than a lint rule has.
 * Suppression comments are policed by harness/guard/suppressions.sh.
 */

/** Browser globals that must be reached through src/shared only. */
const restrictedBrowserGlobals = [
  { name: "localStorage", message: "wrap browser storage in src/shared before using it" },
  { name: "sessionStorage", message: "wrap browser storage in src/shared before using it" },
  { name: "XMLHttpRequest", message: "only src/shared/api may talk to the network" },
];

/** The network boundary: only the shared HTTP client may call fetch. */
const restrictedNetworkGlobals = [
  {
    name: "fetch",
    message: "only src/shared/api may talk to the network; inject an HttpClient instead",
  },
];

export const lintPolicy: OxlintConfig = {
  plugins: [
    "eslint",
    "typescript",
    "oxc",
    "import",
    "promise",
    "unicorn",
    "react",
    "jsx-a11y",
    "vitest",
  ],
  options: {
    // Type aware linting: without it the rules below cannot see through types,
    // which is exactly where an agent's mistakes hide.
    typeAware: true,
    typeCheck: true,
  },
  ignorePatterns: [
    "**/node_modules/**",
    "**/dist/**",
    "**/coverage/**",
    // Generated from services/platform/api/openapi.yaml; regenerate instead of editing.
    "**/src/shared/api/gen/**",
  ],
  categories: {
    correctness: "error",
    suspicious: "error",
    perf: "error",
    pedantic: "error",
    restriction: "off",
    style: "off",
    nursery: "off",
  },
  jsPlugins: [{ name: "vite-plus", specifier: "vite-plus/oxlint-plugin" }],
  rules: {
    // --- escape hatches an agent must not reach for ------------------------
    "typescript/no-explicit-any": "error",
    "typescript/no-unsafe-argument": "error",
    "typescript/no-unsafe-assignment": "error",
    "typescript/no-unsafe-call": "error",
    "typescript/no-unsafe-member-access": "error",
    "typescript/no-unsafe-return": "error",
    "typescript/ban-ts-comment": "error",
    "typescript/no-non-null-assertion": "error",
    "typescript/no-unnecessary-type-assertion": "error",
    "typescript/consistent-type-assertions": [
      "error",
      { assertionStyle: "as", objectLiteralTypeAssertions: "never" },
    ],
    "eslint/no-eval": "error",
    "eslint/no-implied-eval": "error",
    "eslint/no-debugger": "error",
    "eslint/no-alert": "error",
    "eslint/no-console": "error",
    "eslint/no-var": "error",
    "eslint/prefer-const": "error",
    "eslint/eqeqeq": ["error", "always"],
    // Long functions are where agents hide complexity; comments and blank lines do not count.
    "eslint/max-lines-per-function": [
      "error",
      { max: 80, skipBlankLines: true, skipComments: true },
    ],
    "eslint/no-restricted-globals": [
      "error",
      ...restrictedBrowserGlobals,
      ...restrictedNetworkGlobals,
    ],

    // --- module shape ------------------------------------------------------
    "import/no-default-export": "error",
    "import/no-cycle": "error",
    "import/no-self-import": "error",
    // Side-effect imports are only legitimate for stylesheets.
    "import/no-unassigned-import": ["error", { allow: ["**/*.css"] }],
    "typescript/consistent-type-imports": "error",
    "typescript/explicit-function-return-type": ["error", { allowExpressions: true }],
    "typescript/switch-exhaustiveness-check": "error",
    // Deliberately off: it rejects every React and DOM type (props, events,
    // refs) and therefore cannot be satisfied by ordinary React code.
    "typescript/prefer-readonly-parameter-types": "off",
    "typescript/no-unsafe-type-assertion": "error",

    // --- async correctness (type aware) ------------------------------------
    "typescript/no-floating-promises": "error",
    "typescript/no-misused-promises": "error",
    "typescript/await-thenable": "error",
    "typescript/require-await": "error",
    "promise/catch-or-return": "off",
    // Sequential awaits (polling, ordered side effects) are a design choice, not a bug.
    "eslint/no-await-in-loop": "off",

    // --- react -------------------------------------------------------------
    // The automatic JSX runtime (tsconfig "jsx": "react-jsx") needs no React import.
    "react/react-in-jsx-scope": "off",
    "react/rules-of-hooks": "error",
    "react/exhaustive-deps": "error",
    "react/only-export-components": ["error", { allowConstantExport: true }],
    "react/jsx-key": "error",
    "react/jsx-no-useless-fragment": "error",

    // --- tests -------------------------------------------------------------
    "vitest/no-disabled-tests": "error",
    "vitest/no-focused-tests": "error",
    "vitest/expect-expect": "error",

    // --- toolchain ---------------------------------------------------------
    "vite-plus/prefer-vite-plus-imports": "error",
  },
  overrides: [
    {
      // The single sanctioned network boundary.
      files: ["**/src/shared/api/**"],
      rules: {
        "eslint/no-restricted-globals": ["error", ...restrictedBrowserGlobals],
      },
    },
    {
      // Repository tooling runs under Node and talks to the outside world.
      files: ["harness/**", "e2e/**"],
      env: { node: true },
      rules: {
        "eslint/no-restricted-globals": ["error", ...restrictedBrowserGlobals],
        "eslint/no-console": "off",
      },
    },
    {
      // Test suites are flat lists of cases; a describe block may be long.
      files: ["**/*.test.ts", "**/*.test.tsx"],
      rules: {
        "eslint/max-lines-per-function": "off",
      },
    },
    {
      // Vite and Vitest configs are consumed by the tooling as default exports.
      files: ["**/vite.config.ts", "**/vitest.config.ts", "**/playwright.config.ts"],
      rules: {
        "import/no-default-export": "off",
      },
    },
  ],
};
