/**
 * The 「思考」 switch's persisted value (platform knobs subproject, decided
 * 2026-09-16): whether a question is asked with the planner's thinking on
 * or off. Split out of `conversationStore.tsx` to keep that file under the
 * harness's file-length guard, and because `localStorage` access needs its
 * own try/catch at every read and write - a private window, cleared site
 * data, or a browser that blocks site data can all make either call throw.
 *
 * The default is off, matching the platform's own configured default
 * (`ORCHESTRA_PLANNER_THINKING`, `services/platform/internal/infra/config`)
 * - measured 2026-09-16, docs/specs/shortlisting.md: thinking off gives
 * correct@1 67 / correct@shown 71 at a mean 1377ms and never hits
 * max_tokens, against 67/70 at a mean 6223ms with thinking on.
 */

import { useCallback, useEffect, useRef, useState } from "react";

const STORAGE_KEY = "orchestra.conversation.thinking";

/** Reads the persisted switch value, `false` when nothing is stored or `localStorage` throws. */
export function readThinkingPreference(): boolean {
  try {
    return globalThis.localStorage.getItem(STORAGE_KEY) === "true";
  } catch {
    return false;
  }
}

/** Persists the switch value. Silently does nothing if `localStorage` throws. */
export function writeThinkingPreference(value: boolean): void {
  try {
    globalThis.localStorage.setItem(STORAGE_KEY, value ? "true" : "false");
  } catch {
    // Per this module's own doc comment: a viewer with no working
    // localStorage keeps working, just without the switch remembered
    // across a reload.
  }
}

/** `useThinkingSwitch`'s own return: the value to render, a setter for `QuestionForm`'s `onChange`, and a ref for `ask` to read without being recreated on every toggle. */
export interface ThinkingSwitch {
  readonly thinking: boolean;
  readonly thinkingRef: { readonly current: boolean };
  readonly setThinking: (value: boolean) => void;
}

/**
 * The 「思考」 switch's state, factored out of `ConversationProvider`
 * (`conversationStore.tsx`) to keep that component under
 * `harness/quality/eslint`'s `max-lines-per-function` guard. Initialised
 * from `readThinkingPreference`, so a reload keeps what was last chosen;
 * `thinkingRef` mirrors `thinking` the same way `conversationStore.tsx`'s
 * own `conversationsRef` mirrors its state, so `ask` can read the value a
 * question was actually sent with without depending on `thinking` itself
 * and being recreated on every toggle.
 */
export function useThinkingSwitch(): ThinkingSwitch {
  const [thinking, setThinkingState] = useState<boolean>(() => readThinkingPreference());
  const thinkingRef = useRef(thinking);

  useEffect(() => {
    thinkingRef.current = thinking;
  }, [thinking]);

  const setThinking = useCallback((value: boolean): void => {
    setThinkingState(value);
    writeThinkingPreference(value);
  }, []);

  return { thinking, thinkingRef, setThinking };
}
