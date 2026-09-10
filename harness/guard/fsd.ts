/**
 * Frontend architecture guard (Feature-Sliced Design).
 *
 * Reads harness/quality/architecture.json and checks every import in the web
 * sources against the layer rules:
 *
 *   1. a module may import only from its own layer or a lower one;
 *   2. a sliced layer (entities, features, widgets, pages) may not import a
 *      sibling slice of the same layer;
 *   3. another slice must be reached through its public API (the slice root
 *      index), never through a deep path;
 *   4. relative imports may not escape the slice they start in.
 *
 * It is deliberately independent of the linter so the architecture stays
 * enforceable even if the lint policy changes. Run from the repository root:
 *
 *   node harness/guard/fsd.ts
 */
import { existsSync, readdirSync, readFileSync } from "node:fs";
import path from "node:path";

import { parseSync } from "oxc-parser";

interface LayerPolicy {
  readonly name: string;
  readonly sliced: boolean;
}

interface WebPolicy {
  readonly dir: string;
  readonly layers: readonly LayerPolicy[];
  readonly segments: readonly string[];
}

interface ArchitecturePolicy {
  readonly web: WebPolicy;
}

/** Where a module sits in the architecture. */
export interface Location {
  readonly layer: string;
  readonly rank: number;
  /** The slice name for sliced layers, null for shared/app. */
  readonly slice: string | null;
}

export interface Violation {
  readonly file: string;
  readonly specifier: string;
  readonly reason: string;
}

const repositoryRoot = path.resolve(import.meta.dirname, "..", "..");
const policyFile = path.join(repositoryRoot, "harness", "quality", "architecture.json");
const sourceExtensions = new Set([".ts", ".tsx", ".js", ".jsx", ".mts", ".cts"]);

/** Loads and validates the architecture policy. */
function loadPolicy(): WebPolicy {
  const raw: unknown = JSON.parse(readFileSync(policyFile, "utf8"));

  if (!isArchitecturePolicy(raw)) {
    throw new TypeError(`${policyFile}: web policy is missing or malformed`);
  }

  return raw.web;
}

/** Structural check of the policy file. */
function isArchitecturePolicy(value: unknown): value is ArchitecturePolicy {
  if (typeof value !== "object" || value === null || !("web" in value)) {
    return false;
  }

  const { web } = value;

  return (
    typeof web === "object" &&
    web !== null &&
    "dir" in web &&
    typeof web.dir === "string" &&
    "layers" in web &&
    Array.isArray(web.layers) &&
    web.layers.every(
      (layer: unknown) =>
        typeof layer === "object" &&
        layer !== null &&
        "name" in layer &&
        typeof layer.name === "string" &&
        "sliced" in layer &&
        typeof layer.sliced === "boolean",
    )
  );
}

/** Lists every source file below dir, recursively. */
function listSources(dir: string): string[] {
  const files: string[] = [];

  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);

    if (entry.isDirectory()) {
      files.push(...listSources(full));
    } else if (sourceExtensions.has(path.extname(entry.name))) {
      files.push(full);
    }
  }

  return files.toSorted((a, b) => a.localeCompare(b));
}

/** Locates a path relative to the source root (e.g. "features/greet/ui/Form.tsx"). */
export function locate(relativePath: string, layers: readonly LayerPolicy[]): Location | null {
  const [layerName, second] = relativePath.split("/");
  const rank = layers.findIndex((layer) => layer.name === layerName);

  if (rank === -1 || layerName === undefined) {
    return null;
  }

  const layer = layers[rank];

  if (layer === undefined) {
    return null;
  }

  return { layer: layerName, rank, slice: layer.sliced ? (second ?? null) : null };
}

/** Resolves an import specifier to a path relative to the source root, or null when external. */
export function resolveSpecifier(importer: string, specifier: string): string | null {
  if (specifier.startsWith("@/")) {
    return specifier.slice(2);
  }

  if (specifier.startsWith(".")) {
    return path.posix.normalize(path.posix.join(path.posix.dirname(importer), specifier));
  }

  return null;
}

/** Whether a resolved path points at a slice root (its public API) rather than inside it. */
function isPublicApi(target: string, location: Location): boolean {
  if (location.slice === null) {
    return true;
  }

  const sliceRoot = `${location.layer}/${location.slice}`;
  const rest = target.slice(sliceRoot.length);

  return rest === "" || rest === "/index" || rest === "/index.ts";
}

/** Checks one import edge and returns the violation, if any. */
export function checkImport(
  importer: string,
  specifier: string,
  layers: readonly LayerPolicy[],
): Violation | null {
  const target = resolveSpecifier(importer, specifier);

  if (target === null) {
    return null;
  }

  const from = locate(importer, layers);
  const to = locate(target, layers);

  if (from === null || to === null) {
    return {
      file: importer,
      specifier,
      reason: "module or import target does not belong to any declared layer",
    };
  }

  if (to.rank > from.rank) {
    return {
      file: importer,
      specifier,
      reason: `${from.layer} may not import ${to.layer} (dependencies point downwards)`,
    };
  }

  const sameSlice = from.layer === to.layer && from.slice === to.slice;

  if (sameSlice) {
    return null;
  }

  if (from.layer === to.layer && from.slice !== null) {
    return {
      file: importer,
      specifier,
      reason: `${from.layer}/${from.slice} may not import sibling slice ${to.layer}/${to.slice ?? ""}`,
    };
  }

  if (!isPublicApi(target, to)) {
    return {
      file: importer,
      specifier,
      reason: `reach ${to.layer}/${to.slice ?? ""} through its public API (index.ts), not ${target}`,
    };
  }

  return null;
}

/** Extracts every static and dynamic import specifier of a source file. */
function importsOf(file: string): string[] {
  const source = readFileSync(file, "utf8");
  const { module, errors } = parseSync(file, source);

  if (errors.length > 0) {
    throw new Error(`${file}: ${errors.map((error) => error.message).join("; ")}`);
  }

  return [
    ...module.staticImports.map((entry) => entry.moduleRequest.value),
    ...module.dynamicImports.map((entry) =>
      source.slice(entry.moduleRequest.start + 1, entry.moduleRequest.end - 1),
    ),
    ...module.staticExports.flatMap((entry) =>
      entry.entries.flatMap((item) =>
        item.moduleRequest === null ? [] : [item.moduleRequest.value],
      ),
    ),
  ];
}

/** Runs the guard over the whole web source tree. */
function run(): number {
  const policy = loadPolicy();
  const sourceRoot = path.join(repositoryRoot, policy.dir);

  // A guard reports on code. Before the first web file exists there is
  // nothing to report, and reading a missing directory is not a finding.
  if (!existsSync(sourceRoot)) {
    console.log(`fsd: ${policy.dir} does not exist yet`);

    return 0;
  }

  const violations: Violation[] = [];
  const files = listSources(sourceRoot);

  for (const file of files) {
    const importer = path.relative(sourceRoot, file).split(path.sep).join("/");

    if (importer.startsWith("test/")) {
      continue;
    }

    for (const specifier of importsOf(file)) {
      const violation = checkImport(importer, specifier, policy.layers);

      if (violation !== null) {
        violations.push(violation);
      }
    }
  }

  for (const violation of violations) {
    console.error(
      `${policy.dir}/${violation.file}: import "${violation.specifier}": ${violation.reason}`,
    );
  }

  if (violations.length > 0) {
    console.error(`fsd: ${violations.length} violation(s)`);

    return 1;
  }

  console.log(`fsd: ${files.length} files, ${policy.layers.length} layers, no violations`);

  return 0;
}

if (process.argv[1] !== undefined && path.resolve(process.argv[1]) === import.meta.filename) {
  process.exitCode = run();
}
