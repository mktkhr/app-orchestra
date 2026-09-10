/**
 * Frontend duplication guard.
 *
 * Reads harness/quality/duplication.txt and fails when two functions have the same
 * structure. See that file for the rule; the backend's equivalent is dupl,
 * enabled in harness/quality/go/golangci.yml.
 *
 * Structure means the sequence of AST node types, so a copy survives renaming
 * its variables and stays visible here. Run from the repository root:
 *
 *   node harness/guard/duplication.ts
 */
import { existsSync, readdirSync, readFileSync } from "node:fs";
import path from "node:path";

import { parseSync } from "oxc-parser";

interface Policy {
  readonly minNodes: number;
  readonly exceptions: ReadonlySet<string>;
}

interface Shape {
  readonly file: string;
  readonly line: number;
  readonly size: number;
  readonly signature: string;
}

const repositoryRoot = path.resolve(import.meta.dirname, "..", "..");
const sourceRoot = path.join(repositoryRoot, "web/src");

/** Node types that begin a unit worth comparing. */
const FUNCTION_TYPES = new Set([
  "FunctionDeclaration",
  "FunctionExpression",
  "ArrowFunctionExpression",
  "MethodDefinition",
]);

/** Reads the policy file. */
function loadPolicy(): Policy {
  const text = readFileSync(path.join(repositoryRoot, "harness/quality/duplication.txt"), "utf8");
  const exceptions = new Set<string>();
  let minNodes = 0;

  for (const raw of text.split("\n")) {
    const line = raw.trim();

    if (line === "" || line.startsWith("#")) continue;

    const [key, value] = line.split(/\s+/u);

    if (value === undefined) continue;
    if (key === "minNodes") minNodes = Math.trunc(Number(value));
    if (key === "except") exceptions.add(value);
  }

  if (minNodes <= 0) throw new Error("harness/quality/duplication.txt needs a positive minNodes");

  return { minNodes, exceptions };
}

/** Every .ts and .tsx source under a directory, generated code and tests excluded. */
function listSources(dir: string): string[] {
  const found: string[] = [];

  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);

    if (entry.isDirectory()) {
      if (entry.name !== "node_modules" && entry.name !== "gen") found.push(...listSources(full));
    } else if (/\.tsx?$/u.test(entry.name) && !entry.name.includes(".test.")) {
      found.push(full);
    }
  }

  return found;
}

/** 1-based line of a byte offset. */
function lineOf(source: string, offset: number): number {
  return source.slice(0, offset).split("\n").length;
}

/** Every node type under a node, in order. Names and values are deliberately dropped. */
function shapeOf(root: unknown): string[] {
  const types: string[] = [];
  const seen = new Set<object>();

  const walk = (node: unknown): void => {
    if (node === null || typeof node !== "object" || seen.has(node)) return;

    seen.add(node);

    if (Array.isArray(node)) {
      for (const item of node) walk(item);

      return;
    }

    if ("type" in node && typeof node.type === "string") types.push(node.type);

    for (const value of Object.values(node)) walk(value);
  };

  walk(root);

  return types;
}

/** The shape of every function in a parsed program. */
function shapes(program: unknown, file: string, source: string): Shape[] {
  const found: Shape[] = [];
  const seen = new Set<object>();

  const walk = (node: unknown): void => {
    if (node === null || typeof node !== "object" || seen.has(node)) return;

    seen.add(node);

    if (Array.isArray(node)) {
      for (const item of node) walk(item);

      return;
    }

    if ("type" in node && typeof node.type === "string" && FUNCTION_TYPES.has(node.type)) {
      const types = shapeOf(node);
      const start = "start" in node && typeof node.start === "number" ? node.start : 0;

      found.push({
        file,
        line: lineOf(source, start),
        size: types.length,
        signature: types.join(","),
      });
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
    console.log("duplication: no frontend sources yet");

    return 0;
  }

  const policy = loadPolicy();
  const byShape = new Map<string, Shape[]>();
  let checked = 0;

  for (const file of listSources(sourceRoot)) {
    const relative = path.relative(repositoryRoot, file).split(path.sep).join("/");

    if (policy.exceptions.has(relative)) continue;

    checked += 1;

    const source = readFileSync(file, "utf8");
    const { program, errors } = parseSync(file, source);

    if (errors.length > 0) continue;

    for (const shape of shapes(program, relative, source)) {
      if (shape.size < policy.minNodes) continue;

      byShape.set(shape.signature, [...(byShape.get(shape.signature) ?? []), shape]);
    }
  }

  const clones = [...byShape.values()].filter((group) => group.length > 1);

  if (clones.length > 0) {
    for (const group of clones) {
      const where = group.map((shape) => `${shape.file}:${shape.line}`).join(", ");

      console.log(
        `${group.length} functions share one structure of ${group[0]?.size ?? 0} nodes: ${where}`,
      );
    }

    console.log(`duplication: ${clones.length} duplicated structure(s)`);

    return 1;
  }

  console.log(`duplication: ${checked} file(s), no structure repeated`);

  return 0;
}

process.exitCode = run();
