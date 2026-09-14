/**
 * The pick module's public surface (TODO.md "Measure the pick, not only the
 * recall"). `gather-pick-shortlists.ts` and `gather-pick-scoring.ts` read
 * this file, not `client.ts` directly - the same convention `embedding/
 * index.ts` and `utterances/index.ts` follow.
 */
export {
  PICK_MODEL,
  PICK_SYSTEM_PROMPT,
  newPickGuardState,
  pick,
  type PickCandidate,
  type PickGuardState,
  type PickResult,
} from "./client.ts";
