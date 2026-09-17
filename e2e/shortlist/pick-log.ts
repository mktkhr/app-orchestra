import { existsSync, readFileSync } from "node:fs";

import { asRecord } from "../src/helpers/wire.ts";

/**
 * Reads a mid pass's platform log file (the one `boot.ts`'s `logFile`
 * option writes: the platform's stdout and stderr, its JSON logs among
 * them) for `pick_ms` (docs/plans/midsizing.md Task 3; the field
 * `services/platform/internal/usecase/orchestrator_staging.go`'s
 * `planStaged` logs at info, "pick completed" - one line per staged
 * request). No test imports this file: it is plain disk I/O over a file
 * only a live platform run produces, exactly like `pass-io.ts`.
 */

/** Every `pick_ms` value logged in logPath, in file order - not every line of the file is JSON (a stray non-JSON line is skipped, not fatal). */
function pickMsValues(logPath: string): readonly number[] {
  const lines = readFileSync(logPath, "utf8").split("\n").filter(Boolean);

  return lines
    .map((line): unknown => {
      try {
        return JSON.parse(line);
      } catch {
        return undefined;
      }
    })
    .map((parsed) => (parsed === undefined ? undefined : asRecord(parsed)["pick_ms"]))
    .filter((value): value is number => typeof value === "number");
}

/**
 * The mean of every `pick_ms` line in the platform log at logPath, or
 * `undefined` when the file does not exist or holds no such line - the
 * report then says the number was not available rather than printing a
 * misleading 0 (docs/plans/midsizing.md Task 3).
 */
export function pickMeanMs(logPath: string): number | undefined {
  if (!existsSync(logPath)) return undefined;

  const values = pickMsValues(logPath);

  if (values.length === 0) return undefined;

  return values.reduce((a, b) => a + b, 0) / values.length;
}
