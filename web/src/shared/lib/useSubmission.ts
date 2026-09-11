import { useState } from "react";

/** Shown for any failed post-back to the platform: `/api/invoke` or `/api/plan`. */
const FAILURE_MESSAGE = "送信に失敗しました。時間をおいて試してください。";

interface Submission {
  readonly submitting: boolean;
  readonly error: string | null;
  /**
   * Runs `task`, tracking `submitting` around it and turning any thrown
   * error into `error`'s Japanese message. `task` reports its own result
   * (through whatever callback the caller was given) before returning -
   * `run` only owns the surrounding state.
   */
  readonly run: (task: () => Promise<void>) => Promise<void>;
}

/**
 * The submit-and-report shape every control that posts an answer back to
 * the platform shares: `ResultForm` (`POST /api/invoke`) and `ResultChoice`
 * (`POST /api/plan` with an answer) both turn one async call into the same
 * `submitting`/`error` state and the same failure message. Pulling that out
 * here - rather than each component keeping its own `useState` pair and
 * `try`/`catch`/`finally` - is what keeps `guard-duplication` from seeing
 * two copies of the same control flow (`harness/quality/duplication.txt`);
 * only the request itself, and what to do with its result, stays in each
 * component.
 */
export function useSubmission(): Submission {
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const run = async (task: () => Promise<void>): Promise<void> => {
    setError(null);
    setSubmitting(true);

    try {
      await task();
    } catch {
      setError(FAILURE_MESSAGE);
    } finally {
      setSubmitting(false);
    }
  };

  return { submitting, error, run };
}
