import { AxeBuilder } from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import type { Result } from "axe-core";

import { signInAsAdmin } from "./session";

/**
 * Accessibility gate.
 *
 * The interface cannot be asserted beautiful, but most of what makes one
 * unusable is measurable and standardised. WCAG 2.2 AA, zero violations: this
 * says nothing about taste and everything about whether a person can read the
 * screen and hit the controls.
 *
 * Run under both colour schemes. The screen that prompted these gates passed
 * every check in light and was unreadable in dark, because the browser painted
 * a dark canvas while the component library kept drawing a light theme.
 */
const TAGS = ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"];

/** The screens a visitor reaches without an account. */
const PUBLIC_PAGES = [{ name: "sign in", path: "/" }];

/** One readable line per violation, with the elements it points at. */
function describe(violation: Result): string {
  const where = violation.nodes.map((node) => `      ${node.target.join(" ")}`).join("\n");

  return `${violation.id} [${String(violation.impact)}]: ${violation.help}\n${where}`;
}

for (const scheme of ["light", "dark"] as const) {
  test.describe(`${scheme} scheme`, () => {
    test.use({ colorScheme: scheme });

    for (const target of PUBLIC_PAGES) {
      test(`${target.name} has no accessibility violations`, async ({ page }) => {
        await signInAsAdmin(page);
        await page.goto(target.path);
        await page.waitForLoadState("networkidle");

        const results = await new AxeBuilder({ page }).withTags(TAGS).analyze();
        const found = results.violations.map(describe);

        expect(found, found.join("\n  ")).toEqual([]);
      });
    }
  });
}
