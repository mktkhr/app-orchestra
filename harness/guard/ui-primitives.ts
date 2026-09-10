/**
 * Design system guard.
 *
 * Reads harness/quality/ui-primitives.txt and fails when a user interface element is
 * built by hand instead of taken from MUI. See that file for what the rule is
 * and why its first version produced the components it meant to prevent.
 *
 * Two ways to write a raw control, both checked: the JSX tag itself, and the
 * `component` prop that renders one through a container carrying no design.
 *
 * Independent of the linter, like harness/guard/fsd.ts beside it. Run from the
 * repository root:
 *
 *   node harness/guard/ui-primitives.ts
 */
import { existsSync, readdirSync, readFileSync } from "node:fs";
import path from "node:path";

import { parseSync } from "oxc-parser";

interface Policy {
  readonly elements: ReadonlySet<string>;
  readonly escapes: ReadonlySet<string>;
  readonly exceptions: ReadonlySet<string>;
}

interface Violation {
  readonly file: string;
  readonly line: number;
  readonly message: string;
}

const repositoryRoot = path.resolve(import.meta.dirname, "..", "..");
const sourceRoot = path.join(repositoryRoot, "web/src");

/** Reads the policy file into its three lists. */
function loadPolicy(): Policy {
  const text = readFileSync(path.join(repositoryRoot, "harness/quality/ui-primitives.txt"), "utf8");
  const elements = new Set<string>();
  const escapes = new Set<string>();
  const exceptions = new Set<string>();

  for (const raw of text.split("\n")) {
    const line = raw.trim();

    if (line === "" || line.startsWith("#")) continue;

    const [key, value] = line.split(/\s+/u);

    if (value === undefined) continue;
    if (key === "element") elements.add(value);
    if (key === "escape") escapes.add(value);
    if (key === "except") exceptions.add(value);
  }

  if (elements.size === 0) throw new Error("harness/quality/ui-primitives.txt lists no elements");

  return { elements, escapes, exceptions };
}

/** Every .tsx source under a directory, generated code and tests excluded. */
function listSources(dir: string): string[] {
  const found: string[] = [];

  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);

    if (entry.isDirectory()) {
      if (entry.name !== "node_modules" && entry.name !== "gen") found.push(...listSources(full));
    } else if (entry.name.endsWith(".tsx") && !entry.name.includes(".test.")) {
      found.push(full);
    }
  }

  return found;
}

/** 1-based line of a byte offset. */
function lineOf(source: string, offset: number): number {
  return source.slice(0, offset).split("\n").length;
}

/** The name of a JSX opening element, when it is a plain identifier. */
function tagName(node: object): string | null {
  if (!("name" in node)) return null;

  const name = node.name;

  if (typeof name !== "object" || name === null) return null;
  if (!("type" in name) || name.type !== "JSXIdentifier") return null;
  if (!("name" in name) || typeof name.name !== "string") return null;

  return name.name;
}

/**
 * The literal value of a JSX attribute named `component`.
 *
 * Read from the attribute node found by the walk rather than by indexing into
 * the element's attribute array, which the parser types loosely enough that
 * every access through it is an unchecked one.
 *
 * @param node - a JSXAttribute node.
 * @returns the value, or null when it is not a `component="..."` prop.
 */
function componentValue(node: object): string | null {
  if (!("name" in node) || !("value" in node)) return null;

  const { name, value } = node;

  if (typeof name !== "object" || name === null) return null;
  if (!("name" in name) || name.name !== "component") return null;
  if (typeof value !== "object" || value === null) return null;
  if (!("value" in value) || typeof value.value !== "string") return null;

  return value.value;
}

/** One thing worth looking at, found anywhere in a parsed program. */
interface Found {
  readonly kind: "element" | "component";
  readonly name: string;
  readonly start: number;
}

/**
 * Every raw tag and every `component` prop in a parsed program.
 *
 * Both are collected in one walk. A `component="button"` prop is an escape
 * wherever it appears, so it does not need to be tied back to its element.
 *
 * @param program - the parsed AST.
 * @returns what was found, with byte offsets.
 */
function interesting(program: unknown): Found[] {
  const found: Found[] = [];
  const seen = new Set<object>();

  const walk = (node: unknown): void => {
    if (node === null || typeof node !== "object" || seen.has(node)) return;

    seen.add(node);

    if (Array.isArray(node)) {
      for (const item of node) walk(item);

      return;
    }

    const start = "start" in node && typeof node.start === "number" ? node.start : 0;

    if ("type" in node && node.type === "JSXOpeningElement") {
      const tag = tagName(node);

      if (tag !== null) found.push({ kind: "element", name: tag, start });
    }

    if ("type" in node && node.type === "JSXAttribute") {
      const value = componentValue(node);

      if (value !== null) found.push({ kind: "component", name: value, start });
    }

    for (const value of Object.values(node)) walk(value);
  };

  walk(program);

  return found;
}

/** Runs the guard over the frontend sources. */
function run(): number {
  // A guard reports on code. Before the first frontend file exists there is
  // nothing to report, and reading a missing directory is not a finding.
  if (!existsSync(sourceRoot)) {
    console.log("ui-primitives: no frontend sources yet");

    return 0;
  }

  const policy = loadPolicy();
  const violations: Violation[] = [];
  let checked = 0;

  for (const file of listSources(sourceRoot)) {
    const relative = path.relative(repositoryRoot, file).split(path.sep).join("/");

    if (policy.exceptions.has(relative)) continue;

    checked += 1;

    const source = readFileSync(file, "utf8");
    const { program, errors } = parseSync(file, source);

    if (errors.length > 0) {
      violations.push({ file: relative, line: 1, message: errors[0]?.message ?? "parse error" });
      continue;
    }

    for (const item of interesting(program)) {
      const line = lineOf(source, item.start);

      if (item.kind === "element" && policy.elements.has(item.name)) {
        violations.push({
          file: relative,
          line,
          message: `<${item.name}> is not yours to build; use the MUI component`,
        });
      }

      if (item.kind === "component" && policy.escapes.has(item.name)) {
        violations.push({
          file: relative,
          line,
          message: `component="${item.name}" renders a raw control with no design; use the MUI component`,
        });
      }
    }
  }

  if (violations.length > 0) {
    for (const violation of violations)
      console.log(`${violation.file}:${violation.line}: ${violation.message}`);
    console.log(`ui-primitives: ${violations.length} violation(s)`);

    return 1;
  }

  console.log(`ui-primitives: ${checked} file(s), every control comes from MUI`);

  return 0;
}

process.exitCode = run();
