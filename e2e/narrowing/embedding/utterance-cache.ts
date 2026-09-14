/**
 * A second document set beside the catalogue's own vectors (spec G3, AC-G-
 * 102; docs/plans/describing.md Task 2 Step 3): every utterance of an
 * operation, embedded as a document and cached on disk the same way
 * `cache.ts` caches the catalogue's own text - a separate file per
 * configuration *and* utterance layer (`setName`: "generated", "written" or
 * "both"), so this never reads or writes the files `cache.ts` owns and
 * `embedCatalogue`'s cache for a configuration is never invalidated by
 * anything done here (`utterance-cache.test.ts` asserts exactly that).
 *
 * The shape is `cache.ts`'s own: a JSON manifest of operation ids parallel
 * to a flat `Float32Array` of vectors, plus a hash covering the model, the
 * document prefix, the layer's name and every operation's utterances. The
 * one difference is that an operation can own several vectors here, not
 * one - `manifest.operationIds` repeats an id once per utterance, in the
 * order `catalog` and then each operation's own utterance list were walked,
 * so reading the file back groups contiguous runs of the same id into that
 * operation's vector list rather than overwriting a single slot.
 */
import { createHash } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

import type { FixtureOperation } from "../fixture/index.ts";
import { embedMany, type EmbeddingVector, type FetchLike } from "./client.ts";
import type { EmbeddingConfig } from "./configs.ts";

/** Beside `../.vectors` - a sibling cache, never the same files. */
export const DEFAULT_UTTERANCE_VECTORS_DIR = join(import.meta.dirname, "..", ".vectors");

/** Every utterance vector an operation owns, keyed by `operationId` - empty when it has no utterances. */
export type UtteranceVectors = ReadonlyMap<string, readonly EmbeddingVector[]>;

/** Every utterance an operation carries in one layer ("generated", "written" or the union), keyed by `operationId`. */
export type UtteranceTexts = ReadonlyMap<string, readonly string[]>;

interface UtteranceManifest {
  readonly operationIds: readonly string[];
  readonly dimension: number;
  readonly hash: string;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isStringArray(value: unknown): value is readonly string[] {
  return Array.isArray(value) && value.every((item) => typeof item === "string");
}

function parseManifest(value: unknown): UtteranceManifest | undefined {
  if (!isRecord(value)) return undefined;

  const { operationIds, dimension, hash } = value;

  if (!isStringArray(operationIds)) return undefined;
  if (typeof dimension !== "number") return undefined;
  if (typeof hash !== "string") return undefined;

  return { operationIds, dimension, hash };
}

/** A hash of the model, the document prefix, `setName`, and every operation's utterances in `catalog` order - changing any of those is a cache miss. */
function hashOf(
  config: EmbeddingConfig,
  setName: string,
  catalog: readonly FixtureOperation[],
  utterances: UtteranceTexts,
): string {
  const hash = createHash("sha256");

  hash.update(config.model);
  hash.update(" ");
  hash.update(config.documentPrefix);
  hash.update(" ");
  hash.update(setName);

  for (const operation of catalog) {
    hash.update(" | ");
    hash.update(operation.operationId);

    for (const utterance of utterances.get(operation.operationId) ?? []) {
      hash.update(" ");
      hash.update(utterance);
    }
  }

  return hash.digest("hex");
}

function fileId(config: EmbeddingConfig, setName: string): string {
  return `${config.id}-utterances-${setName}`;
}

function manifestPath(dir: string, id: string): string {
  return join(dir, `${id}.json`);
}

function vectorsPath(dir: string, id: string): string {
  return join(dir, `${id}.bin`);
}

/** `manifest.operationIds` grouped back into per-operation vector lists, in the order each id first appears - contiguous runs, because `writeCache` writes them that way. */
function groupByOperation(manifest: UtteranceManifest, flat: Float32Array): UtteranceVectors {
  const result = new Map<string, EmbeddingVector[]>();

  manifest.operationIds.forEach((operationId, index) => {
    const start = index * manifest.dimension;
    const vector = Array.from(flat.subarray(start, start + manifest.dimension));
    const existing = result.get(operationId);

    if (existing === undefined) result.set(operationId, [vector]);
    else existing.push(vector);
  });

  return result;
}

function readCache(dir: string, id: string, expectedHash: string): UtteranceVectors | undefined {
  const manifestFile = manifestPath(dir, id);
  const vectorFile = vectorsPath(dir, id);

  if (!existsSync(manifestFile) || !existsSync(vectorFile)) return undefined;

  const manifest = parseManifest(JSON.parse(readFileSync(manifestFile, "utf-8")));

  if (manifest === undefined) return undefined;
  if (manifest.hash !== expectedHash) return undefined;

  const buffer = readFileSync(vectorFile);
  const arrayBuffer = buffer.buffer.slice(buffer.byteOffset, buffer.byteOffset + buffer.byteLength);

  return groupByOperation(manifest, new Float32Array(arrayBuffer));
}

interface FlatUtterance {
  readonly operationId: string;
  readonly text: string;
}

/** Every utterance in `catalog` order, then each operation's own utterance order - the same order the manifest and the flat vector file are written in. */
function flatten(
  catalog: readonly FixtureOperation[],
  utterances: UtteranceTexts,
): readonly FlatUtterance[] {
  const flat: FlatUtterance[] = [];

  for (const operation of catalog) {
    for (const text of utterances.get(operation.operationId) ?? []) {
      flat.push({ operationId: operation.operationId, text });
    }
  }

  return flat;
}

function writeCache(
  dir: string,
  id: string,
  hash: string,
  flat: readonly FlatUtterance[],
  vectors: readonly EmbeddingVector[],
): UtteranceVectors {
  mkdirSync(dir, { recursive: true });

  const dimension = vectors[0]?.length ?? 0;
  const flatVectors = new Float32Array(flat.length * dimension);

  vectors.forEach((vector, index) => {
    flatVectors.set(vector, index * dimension);
  });

  const manifest: UtteranceManifest = {
    operationIds: flat.map((item) => item.operationId),
    dimension,
    hash,
  };

  writeFileSync(vectorsPath(dir, id), Buffer.from(flatVectors.buffer));
  writeFileSync(manifestPath(dir, id), JSON.stringify(manifest));

  const result = new Map<string, EmbeddingVector[]>();

  flat.forEach((item, index) => {
    const existing = result.get(item.operationId);
    const vector = vectors[index] ?? [];

    if (existing === undefined) result.set(item.operationId, [vector]);
    else existing.push(vector);
  });

  return result;
}

/** Where `embedUtteranceVectors` reads and writes its cache, and how it reaches the transport when it misses. */
export interface UtteranceVectorCacheOptions {
  readonly dir?: string;
  readonly fetchImpl?: FetchLike;
  readonly baseUrl?: string;
}

/**
 * Every utterance in `utterances` embedded as a document under `config`,
 * cached under `setName` beside `config`'s own catalogue vectors but never
 * overwriting them. Served from disk on an exact match; otherwise embedded
 * through the transport in one batch and written before being returned.
 */
export async function embedUtteranceVectors(
  config: EmbeddingConfig,
  setName: string,
  catalog: readonly FixtureOperation[],
  utterances: UtteranceTexts,
  options: UtteranceVectorCacheOptions = {},
): Promise<UtteranceVectors> {
  const dir = options.dir ?? DEFAULT_UTTERANCE_VECTORS_DIR;
  const id = fileId(config, setName);
  const hash = hashOf(config, setName, catalog, utterances);
  const cached = readCache(dir, id, hash);

  if (cached !== undefined) return cached;

  const flat = flatten(catalog, utterances);
  const texts = flat.map((item) => item.text);
  const vectors =
    texts.length === 0
      ? []
      : await embedMany(config, texts, "document", options.fetchImpl, options.baseUrl);

  return writeCache(dir, id, hash, flat, vectors);
}
