import { useCallback, useEffect, useMemo, useRef, useState, type JSX, type ReactNode } from "react";

import { postPlan, type PlanRequest, type PlanResult } from "@/shared/api/client";
import { PlanRequestError } from "@/shared/api/planRequestError";
import { nextTurnId } from "@/shared/lib/turnId";

import { toAlternatives } from "./alternatives";
import {
  ConversationStoreContext,
  type ConversationState,
  type ConversationStoreValue,
} from "./conversationContext";
import { useThinkingSwitch } from "./thinkingPreference";
import { toContextTurns, type ContextTurn } from "./toContextTurns";
import type { Turn } from "./turn";

const GENERIC_FAILURE = "質問の送信に失敗しました。時間をおいて試してください。";

/**
 * What the error bar says when a plan fails. A `PlanRequestError` carries
 * the platform's own message (since a service's 4xx became a `none`
 * result, a failure here is a platform or service fault worth naming);
 * anything else - a network error, a parse error - keeps the generic line.
 */
function failureMessage(failure: unknown): string {
  if (failure instanceof PlanRequestError) {
    return `${GENERIC_FAILURE}（${failure.serverMessage}）`;
  }

  return GENERIC_FAILURE;
}

const EMPTY_CONVERSATION: ConversationState = { turns: [], pending: false, error: null };

/**
 * The turn `ask` (below) appends before its request resolves. A chip click
 * carries both `preferred` and `label`, and is a choice, not a new question
 * (docs/specs/shortlisting.md, section 4, H5): the transcript must not show
 * `query` a second time, but `nearestQuestion` (`TurnList`) still needs
 * `query` on this turn so any answer's own chip resends the same original
 * question. An ordinary question - no `preferred`, or `preferred` with no
 * `label` - appends the plain question turn it always has.
 */
function newQuestionTurn(query: string, preferred?: string, label?: string): Turn {
  return preferred === undefined || label === undefined
    ? { id: nextTurnId(), role: "question", text: query }
    : { id: nextTurnId(), role: "choice", text: query, label };
}

/**
 * Marks the alternatives turn a chip click answered - `chosen` becomes the
 * picked alternative's `operationId`, which is what makes that turn's
 * other chips disabled and this one selected (`AlternativesRow`). Applied
 * in the same update as adding the question/choice turn (below), before
 * the re-plan request is even sent, so the chip a person clicked cannot be
 * clicked a second time while that request is in flight.
 *
 * Ordinary questions - `alternativesTurnId` undefined - leave every turn
 * as it is.
 */
function markChosen(
  turns: readonly Turn[],
  alternativesTurnId: string | undefined,
  preferred: string | undefined,
): readonly Turn[] {
  if (alternativesTurnId === undefined || preferred === undefined) {
    return turns;
  }

  return turns.map((turn) =>
    turn.id === alternativesTurnId && turn.role === "alternatives"
      ? { ...turn, chosen: preferred }
      : turn,
  );
}

/**
 * `postPlan`'s body: `query` plus whichever of `turns`/`workspaceId`/`preferred`
 * this call has, and `thinking` always - `true` or `false`, the switch's own
 * value at the moment `ask` was called, never omitted (the contract's
 * "omitted means the platform's configured default" is for other/future
 * callers, not this client: a person's own choice is explicit either way).
 */
function planRequestBody(
  query: string,
  contextTurns: readonly ContextTurn[],
  workspaceId: string | undefined,
  preferred: string | undefined,
  thinking: boolean,
): PlanRequest {
  return {
    query,
    ...(contextTurns.length === 0 ? {} : { turns: contextTurns }),
    ...(workspaceId === undefined ? {} : { workspaceId }),
    ...(preferred === undefined ? {} : { preferred }),
    thinking,
  };
}

/**
 * `current` plus the turn(s) a `postPlan` result appends: the answer turn
 * always, and - only when the result carries a non-empty `alternatives` -
 * a second turn right after it, reading 「違いましたか？」
 * (docs/specs/shortlisting.md, section 4, H5). `chosen` starts undefined:
 * `markChosen` (above) is what sets it, on a later call, once a chip on
 * this very turn is clicked.
 */
function answeredTurns(current: readonly Turn[], result: PlanResult): readonly Turn[] {
  const alternatives = result.kind === "result" ? toAlternatives(result.alternatives) : [];
  const answerTurn: Turn = { id: nextTurnId(), role: "answer", result };

  if (alternatives.length === 0) {
    return [...current, answerTurn];
  }

  return [
    ...current,
    answerTurn,
    { id: nextTurnId(), role: "alternatives", text: "違いましたか？", alternatives },
  ];
}

/**
 * Sends `body` and folds the result (or failure) into `key`'s conversation
 * via `update`. Extracted from `ask` to keep `ConversationProvider` under
 * `harness/quality/eslint`'s `max-lines-per-function`.
 */
async function planAndUpdate(
  update: (key: string, updater: (current: ConversationState) => ConversationState) => void,
  key: string,
  body: PlanRequest,
): Promise<void> {
  try {
    const result = await postPlan(body);

    update(key, (current) => ({
      ...current,
      pending: false,
      turns: answeredTurns(current.turns, result),
    }));
  } catch (failure: unknown) {
    update(key, (current) => ({
      ...current,
      pending: false,
      error: failureMessage(failure),
    }));
  }
}

interface ConversationProviderProps {
  readonly children: ReactNode;
}

/**
 * Holds every conversation, keyed - `"chat"`, and one per workspace id
 * (`docs/specs/context.md` section 3a) - above whichever screen draws it.
 * Mounted once in `App`, the same way `SessionProvider` holds the session
 * above every screen that reads it, so switching screens (`MainContent`
 * unmounting `ChatPage`/`WorkspacePage`) never unmounts the conversation
 * itself - only the component that renders it.
 *
 * A conversation not yet asked anything is not stored at all; `getConversation`
 * returns the same `EMPTY_CONVERSATION` for any key nothing has touched, so a
 * workspace nobody has asked a question in costs nothing.
 */
export function ConversationProvider({ children }: ConversationProviderProps): JSX.Element {
  const [conversations, setConversations] = useState<Readonly<Record<string, ConversationState>>>(
    {},
  );

  // Mirrors `conversations` on every render, so `ask` (below) can read the
  // turns a question follows without depending on `conversations` itself -
  // `setConversations`'s functional updater runs on React's own schedule,
  // not synchronously when `update` is called, so it cannot be read back
  // through a closure the way `ask` briefly tried to.
  const conversationsRef = useRef(conversations);
  useEffect(() => {
    conversationsRef.current = conversations;
  }, [conversations]);

  // The 「思考」 switch (platform knobs subproject, decided 2026-09-16): one
  // value for every conversation - see `useThinkingSwitch`'s own doc.
  const { thinking, thinkingRef, setThinking } = useThinkingSwitch();

  const getConversation = useCallback(
    (key: string): ConversationState => conversations[key] ?? EMPTY_CONVERSATION,
    [conversations],
  );

  const update = useCallback(
    (key: string, updater: (current: ConversationState) => ConversationState): void => {
      setConversations((current) => ({
        ...current,
        [key]: updater(current[key] ?? EMPTY_CONVERSATION),
      }));
    },
    [],
  );

  const ask = useCallback(
    async (
      key: string,
      query: string,
      workspaceId?: string,
      preferred?: string,
      label?: string,
      alternativesTurnId?: string,
    ): Promise<void> => {
      // Read before `update` adds this question as its own turn - the turns
      // this question follows, not the one it is about to add.
      const contextTurns: readonly ContextTurn[] = toContextTurns(
        (conversationsRef.current[key] ?? EMPTY_CONVERSATION).turns,
      );

      update(key, (current) => ({
        ...current,
        error: null,
        pending: true,
        turns: [
          ...markChosen(current.turns, alternativesTurnId, preferred),
          newQuestionTurn(query, preferred, label),
        ],
      }));

      await planAndUpdate(
        update,
        key,
        planRequestBody(query, contextTurns, workspaceId, preferred, thinkingRef.current),
      );
    },
    [update, thinkingRef],
  );

  const submitForm = useCallback(
    (key: string, result: PlanResult): void => {
      update(key, (current) => ({
        ...current,
        turns: [...current.turns, { id: nextTurnId(), role: "answer", result }],
      }));
    },
    [update],
  );

  const newConversation = useCallback((key: string): void => {
    setConversations((current) => ({ ...current, [key]: EMPTY_CONVERSATION }));
  }, []);

  const value = useMemo<ConversationStoreValue>(
    () => ({ getConversation, ask, submitForm, newConversation, thinking, setThinking }),
    [getConversation, ask, submitForm, newConversation, thinking, setThinking],
  );

  return (
    <ConversationStoreContext.Provider value={value}>{children}</ConversationStoreContext.Provider>
  );
}
