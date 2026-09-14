/**
 * The generated layer's disk cache (spec G4, AC-G-101): one JSON file per
 * operation under `e2e/narrowing/.utterances/`, named by a hash of the
 * model, the prompt and the operation's own `combinedTextOf` text - exactly
 * the shape `embedding/cache.ts` uses for the catalogue's vectors, but
 * per-operation rather than per-configuration, because a single changed
 * operation must regenerate only itself (spec section 5, plan Task 1 Step
 * 4) and a whole-catalogue hash cannot express that.
 *
 * `generateUtterances` is the seam `make narrowing` calls: served from disk
 * on every call after the first, so a second `make narrowing` reaches the
 * transport zero times (AC-G-101). No manifest - unlike `embedding/
 * cache.ts`, there is no ordered vector array to reconstruct, so each file
 * is self-contained and the hash alone is enough to find it.
 */
import { createHash } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

import { combinedTextOf } from "../lexical.ts";
import type { FixtureOperation } from "../fixture/index.ts";
import type { FetchLike } from "../embedding/client.ts";
import { UTTERANCE_MODEL, UTTERANCE_PROMPT, generateUtterancesFor } from "./generate.ts";

/** Where a cache entry lives: `<dir>/<hash>.json`. */
export const DEFAULT_UTTERANCES_DIR = join(import.meta.dirname, "..", ".utterances");

/** Every utterance generated so far for an operation, keyed by `operationId`. */
export type OperationUtterances = ReadonlyMap<string, readonly string[]>;

/** Where `generateUtterances` reads and writes its cache, and how it reaches the transport on a miss. */
export interface UtteranceCacheOptions {
  readonly dir?: string;
  readonly fetchImpl?: FetchLike;
  readonly baseUrl?: string;
  /** The prompt to hash and to send - defaults to `UTTERANCE_PROMPT`. Overridable only so a test can prove that changing it invalidates every entry (spec section 5). */
  readonly prompt?: string;
}

interface CacheEntry {
  readonly operationId: string;
  readonly utterances: readonly string[];
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isStringArray(value: unknown): value is readonly string[] {
  return Array.isArray(value) && value.every((item) => typeof item === "string");
}

/** `value` read back as a `CacheEntry`, or `undefined` when it is not shaped like one. */
function parseCacheEntry(value: unknown): CacheEntry | undefined {
  if (!isRecord(value)) return undefined;

  const { operationId, utterances } = value;

  if (typeof operationId !== "string") return undefined;
  if (!isStringArray(utterances)) return undefined;

  return { operationId, utterances };
}

/**
 * A hash of exactly what would be sent to the transport for `operation`:
 * the model, the prompt and `combinedTextOf` (spec section 5). Changing any
 * of those changes this hash and is a cache miss for this operation alone.
 */
function hashOf(prompt: string, operation: FixtureOperation): string {
  const hash = createHash("sha256");

  hash.update(UTTERANCE_MODEL);
  hash.update(" ");
  hash.update(prompt);
  hash.update(" ");
  hash.update(combinedTextOf(operation));

  return hash.digest("hex");
}

function entryPath(dir: string, hash: string): string {
  return join(dir, `${hash}.json`);
}

/** The cached utterances for `hash`, or `undefined` on any miss - no file, or one that does not parse. */
function readEntry(dir: string, hash: string): readonly string[] | undefined {
  const file = entryPath(dir, hash);

  if (!existsSync(file)) return undefined;

  return parseCacheEntry(JSON.parse(readFileSync(file, "utf-8")))?.utterances;
}

function writeEntry(
  dir: string,
  hash: string,
  operation: FixtureOperation,
  utterances: readonly string[],
): void {
  mkdirSync(dir, { recursive: true });

  const entry: CacheEntry = { operationId: operation.operationId, utterances };

  writeFileSync(entryPath(dir, hash), JSON.stringify(entry));
}

/**
 * `catalog`'s generated utterances: served from disk for every operation
 * whose hash is already cached, and generated through the transport for the
 * rest, one call per miss - never batched, because llama-swap answers one
 * chat completion at a time. A second call with the same catalogue and the
 * same prompt reaches the transport zero times (AC-G-101).
 */
export async function generateUtterances(
  catalog: readonly FixtureOperation[],
  options: UtteranceCacheOptions = {},
): Promise<OperationUtterances> {
  const dir = options.dir ?? DEFAULT_UTTERANCES_DIR;
  const prompt = options.prompt ?? UTTERANCE_PROMPT;
  const result = new Map<string, readonly string[]>();

  for (const operation of catalog) {
    const hash = hashOf(prompt, operation);
    const cached = readEntry(dir, hash);

    if (cached !== undefined) {
      result.set(operation.operationId, cached);
      continue;
    }

    const generated = await generateUtterancesFor(
      combinedTextOf(operation),
      prompt,
      options.fetchImpl,
      options.baseUrl,
    );

    writeEntry(dir, hash, operation, generated);
    result.set(operation.operationId, generated);
  }

  return result;
}
