import { createContext, useCallback, useContext } from "react";

import type { PlanResult } from "@/shared/api/client";

import type { Turn } from "./turn";

/** One conversation's state: its turns, and whether a question is in flight. */
export interface ConversationState {
  readonly turns: readonly Turn[];
  readonly pending: boolean;
  readonly error: string | null;
}

/**
 * `ask`'s extra knobs, beyond the question text itself. `preferred` carries
 * an alternative's `operationId` when the person chose one of a previous
 * `result`'s `alternatives` (docs/specs/shortlisting.md, section 4):
 * `/api/plan` narrows to that one operation and the planner fills in its
 * parameters, in place of an ordinary question.
 */
export interface AskOptions {
  readonly preferred?: string;
}

/** What a screen can do with the conversation it asks: read it, add to it, end it. */
export interface ConversationHandle extends ConversationState {
  readonly ask: (query: string, options?: AskOptions) => Promise<void>;
  /**
   * Turns a `ResultForm`'s successful `/api/invoke` result into an answer
   * turn - see `Conversation`'s previous `handleFormSubmitted` doc for why
   * this, and not `ask`, is what that path calls.
   */
  readonly submitForm: (result: PlanResult) => void;
  /** Empties this conversation only. No other key is touched. */
  readonly newConversation: () => void;
}

export interface ConversationStoreValue {
  getConversation: (key: string) => ConversationState;
  ask: (key: string, query: string, workspaceId?: string, preferred?: string) => Promise<void>;
  submitForm: (key: string, result: PlanResult) => void;
  newConversation: (key: string) => void;
}

export const ConversationStoreContext = createContext<ConversationStoreValue | null>(null);

const NOT_WRAPPED = "useConversation must be used inside a ConversationProvider";

/**
 * One conversation, by key, plus the means to add to it or end it. Every
 * reader sits under the one `ConversationProvider` `App` mounts, so a `null`
 * context here is a wiring mistake, not a state to render around - the same
 * contract `useSession` makes for `SessionProvider`.
 *
 * `workspaceId` is forwarded, unchanged, onto every `ask` this handle makes
 * - what `POST /api/plan` carries as its own `workspaceId`
 * (`docs/specs/offering.md`, O4), so `propose_panel` is offered only when
 * the caller (`ConversationPanel`) actually has one. Left undefined by the
 * chat screen, which sits on no particular workspace.
 */
export function useConversation(key: string, workspaceId?: string): ConversationHandle {
  const store = useContext(ConversationStoreContext);

  const ask = useCallback(
    (query: string, options?: AskOptions) => {
      if (store === null) {
        throw new Error(NOT_WRAPPED);
      }

      return store.ask(key, query, workspaceId, options?.preferred);
    },
    [store, key, workspaceId],
  );

  const submitForm = useCallback(
    (result: PlanResult) => {
      if (store === null) {
        throw new Error(NOT_WRAPPED);
      }

      store.submitForm(key, result);
    },
    [store, key],
  );

  const newConversation = useCallback(() => {
    if (store === null) {
      throw new Error(NOT_WRAPPED);
    }

    store.newConversation(key);
  }, [store, key]);

  if (store === null) {
    throw new Error(NOT_WRAPPED);
  }

  const state = store.getConversation(key);

  return { ...state, ask, submitForm, newConversation };
}
