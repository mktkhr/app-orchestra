import { useCallback, useEffect, useMemo, useRef, useState, type JSX, type ReactNode } from "react";

import { postPlan, type PlanResult } from "@/shared/api/client";
import { nextTurnId } from "@/shared/lib/turnId";

import {
  ConversationStoreContext,
  type ConversationState,
  type ConversationStoreValue,
} from "./conversationContext";
import { toContextTurns, type ContextTurn } from "./toContextTurns";

const EMPTY_CONVERSATION: ConversationState = { turns: [], pending: false, error: null };

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
    async (key: string, query: string): Promise<void> => {
      // Read before `update` adds this question as its own turn - the turns
      // this question follows, not the one it is about to add.
      const contextTurns: readonly ContextTurn[] = toContextTurns(
        (conversationsRef.current[key] ?? EMPTY_CONVERSATION).turns,
      );

      update(key, (current) => ({
        ...current,
        error: null,
        pending: true,
        turns: [...current.turns, { id: nextTurnId(), role: "question", text: query }],
      }));

      try {
        const result = await postPlan(
          contextTurns.length === 0 ? { query } : { query, turns: contextTurns },
        );

        update(key, (current) => ({
          ...current,
          pending: false,
          turns: [...current.turns, { id: nextTurnId(), role: "answer", result }],
        }));
      } catch {
        update(key, (current) => ({
          ...current,
          pending: false,
          error: "質問の送信に失敗しました。時間をおいて試してください。",
        }));
      }
    },
    [update],
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
    () => ({ getConversation, ask, submitForm, newConversation }),
    [getConversation, ask, submitForm, newConversation],
  );

  return (
    <ConversationStoreContext.Provider value={value}>{children}</ConversationStoreContext.Provider>
  );
}
