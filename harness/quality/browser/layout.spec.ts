import { expect, test, type Page } from "@playwright/test";

/**
 * Layout gate.
 *
 * axe passes on a screen nobody can use: white on black is high contrast and
 * correctly labelled even when the input boxes are invisible and the buttons
 * are a few pixels tall. These are the properties that screen actually broke,
 * written as numbers rather than opinions.
 *
 * Nothing here is about taste. A control you cannot see, cannot hit, or that
 * pushes the page sideways is broken by measurement.
 */

/** Apple HIG is 44pt, Material is 48dp; 44 is the lower of the two. */
const MIN_TARGET_PX = 44;

/** WCAG 1.4.11, the boundary of a user interface component. */
const MIN_BOUNDARY_CONTRAST = 3;

/** The narrowest phone worth supporting. */
const NARROW_VIEWPORT = { width: 375, height: 812 };

/** One control, as the browser reports it. Colours stay as CSS strings. */
interface Control {
  readonly label: string;
  readonly height: number;
  readonly edges: readonly string[];
  /** Background colours from the control outwards, as written. */
  readonly behind: readonly string[];
}

/** Relative luminance of one sRGB channel. */
function channel(value: number): number {
  const v = value / 255;

  return v <= 0.039_28 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
}

/**
 * An `rgb()` or `rgba()` string as [r, g, b], or null when it is transparent.
 *
 * Parsed here rather than in the browser: an evaluate callback is serialised
 * and cannot close over anything, so every helper it needs would have to be
 * redefined inside it on each call.
 *
 * @param value - a computed CSS colour.
 * @returns the triple, or null.
 */
function parse(value: string): number[] | null {
  const match = /rgba?\(([^)]+)\)/u.exec(value);

  if (match === null) return null;

  const parts = (match[1] ?? "").split(",").map((part) => Number(part.trim()));

  if (parts.length >= 4 && (parts[3] ?? 1) < 0.1) return null;

  return [parts[0] ?? 0, parts[1] ?? 0, parts[2] ?? 0];
}

/** WCAG relative luminance of an [r, g, b] triple. */
function luminance(rgb: readonly number[]): number {
  return (
    0.2126 * channel(rgb[0] ?? 0) + 0.7152 * channel(rgb[1] ?? 0) + 0.0722 * channel(rgb[2] ?? 0)
  );
}

/** WCAG contrast ratio of two colours, 1 to 21. */
function contrast(left: readonly number[], right: readonly number[]): number {
  const a = luminance(left);
  const b = luminance(right);

  return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
}

/** The best contrast any edge of a control reaches against what is behind it. */
function bestEdge(control: Control): number {
  const background = firstOpaque(control.behind);

  if (background === null) return 0;

  const ratios = control.edges
    .map((edge) => parse(edge))
    .filter((edge) => edge !== null)
    .map((edge) => contrast(edge, background));

  return Math.max(1, ...ratios);
}

/**
 * The first colour in a chain that is not transparent.
 *
 * @param chain - colours from the element outwards.
 * @returns the triple, or null when the whole chain is transparent.
 */
function firstOpaque(chain: readonly string[]): number[] | null {
  for (const colour of chain) {
    const parsed = parse(colour);

    if (parsed !== null) return parsed;
  }

  return null;
}

/**
 * Measure every control on the current page.
 *
 * The browser side only reads the DOM and returns colours as written; every
 * decision, including which of them is opaque, happens in Node. An evaluate
 * callback is serialised and cannot close over anything, so a helper used
 * inside one has to be redefined on every call.
 *
 * The chain behind each control is returned whole rather than resolved in the
 * browser, because an app that declares no background renders on whatever
 * canvas the user agent paints - which is what made the sign-in inputs
 * invisible in dark mode while every check passed in light.
 *
 * @param page
 * @returns one entry per interactive element.
 */
function measure(page: Page): Promise<Control[]> {
  return page.evaluate(() => {
    const selector = "button, a[href], input, select, textarea";

    return [...document.querySelectorAll(selector)].map((element) => {
      const edges: string[] = [];
      const behind: string[] = [];
      let node: Element | null = element;

      // The visible edge may be drawn by the control or by a wrapper a level or
      // two up, which is how every component library renders an outlined field.
      for (let depth = 0; node !== null; depth += 1) {
        const style = getComputedStyle(node);

        if (depth < 3)
          edges.push(style.borderTopColor, style.borderBottomColor, style.outlineColor);

        behind.push(style.backgroundColor);
        node = node.parentElement;
      }

      const text = (element.textContent ?? "").trim().slice(0, 24);
      const named = element.id === "" ? text : `#${element.id}`;

      return {
        label: named === "" ? element.tagName.toLowerCase() : named,
        height: Math.round(element.getBoundingClientRect().height),
        edges,
        behind,
      };
    });
  });
}

/** The backgrounds the document itself declares, outermost last. */
function pageBackgrounds(page: Page): Promise<string[]> {
  return page.evaluate(() => [
    getComputedStyle(document.body).backgroundColor,
    getComputedStyle(document.documentElement).backgroundColor,
  ]);
}

/** Controls rendered too small to hit reliably. */
function tooSmall(controls: readonly Control[]): string[] {
  return controls
    .filter((control) => control.height > 0 && control.height < MIN_TARGET_PX)
    .map((control) => `${control.label} is ${control.height}px tall, minimum is ${MIN_TARGET_PX}`);
}

/** Controls with no edge distinguishable from what is behind them. */
function invisible(controls: readonly Control[]): string[] {
  return controls
    .filter((control) => bestEdge(control) < MIN_BOUNDARY_CONTRAST)
    .map(
      (control) =>
        `${control.label} has no edge that contrasts with the page: ` +
        `best is ${bestEdge(control).toFixed(2)}:1, minimum is ${MIN_BOUNDARY_CONTRAST}:1`,
    );
}

/** How far the document exceeds the viewport horizontally. */
function overflow(page: Page): Promise<number> {
  return page.evaluate(
    () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
  );
}

/**
 * Both schemes, every time.
 *
 * The interface that prompted these gates passed every check under `light` and
 * was unusable under `dark`: the browser painted a dark canvas while the
 * component library kept drawing a light theme, so the input outlines were
 * black on black. A gate that only runs in one scheme reports a screen that
 * works when half its users cannot read it. AC-F-701 adds five themes on top
 * of this, so it only gets more load-bearing.
 */
for (const scheme of ["light", "dark"] as const) {
  test.describe(`${scheme} scheme`, () => {
    test.use({ colorScheme: scheme });

    test("the application declares its own background", async ({ page }) => {
      await page.goto("/");
      await page.waitForLoadState("networkidle");

      const declared = firstOpaque(await pageBackgrounds(page));

      expect(
        declared,
        "nothing in the document declares a background, so the page renders on whatever " +
          "canvas the browser happens to paint and its colours are decided by the user agent",
      ).not.toBeNull();
    });

    test("every control is big enough to hit", async ({ page }) => {
      await page.goto("/");
      await page.waitForLoadState("networkidle");

      const report = tooSmall(await measure(page));

      expect(report, report.join("\n  ")).toEqual([]);
    });

    test("every control has a visible boundary", async ({ page }) => {
      await page.goto("/");
      await page.waitForLoadState("networkidle");

      const report = invisible(await measure(page));

      expect(report, report.join("\n  ")).toEqual([]);
    });

    test("the page does not scroll sideways on a narrow screen", async ({ page }) => {
      await page.setViewportSize(NARROW_VIEWPORT);
      await page.goto("/");
      await page.waitForLoadState("networkidle");

      const excess = await overflow(page);

      expect(excess, `the page is ${excess}px wider than the viewport`).toBeLessThanOrEqual(0);
    });
  });
}
