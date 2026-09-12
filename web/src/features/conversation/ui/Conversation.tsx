import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import Stack from "@mui/material/Stack";
import type { JSX, ReactNode } from "react";

import type { PlanResult } from "@/shared/api/client";

import { useConversation } from "../model/conversationContext";
import { ExampleQuestions } from "./ExampleQuestions";
import { QuestionForm } from "./QuestionForm";
import { TurnList, type SaveControlSlotProps } from "./TurnList";

interface ConversationProps {
  /**
   * Which conversation this draws - `"chat"`, or a workspace id
   * (`docs/specs/context.md` section 3a). The conversation itself lives in
   * `ConversationProvider`, above whichever screen mounts this component, so
   * the same key always finds the same turns even after this component has
   * unmounted and remounted (opening a workspace and coming back).
   */
  readonly conversationKey: string;
  /**
   * Draws a result turn's "save to a workspace" control - forwarded
   * straight to `TurnList`. See that prop's doc for why this is a slot
   * rather than an import: `features/conversation` cannot reach into the
   * sibling `features/workspaces`. Left undefined by tests and any screen
   * that has no save control to offer.
   */
  readonly renderSaveControl?: ((props: SaveControlSlotProps) => ReactNode) | undefined;
}

/**
 * The chat conversation: the turn list, the question input, and - before the
 * first question - the example questions (AC-F-104). See
 * docs/specs/orchestration.md section 7.
 */
export function Conversation({
  conversationKey,
  renderSaveControl,
}: ConversationProps): JSX.Element {
  const { turns, pending, error, ask, submitForm, newConversation } =
    useConversation(conversationKey);

  const handleSubmit = (query: string): void => {
    void ask(query);
  };

  /**
   * `ResultForm`'s successful `/api/invoke` result, already shaped as the
   * `PlanResult` a `kind: "result"` answer would carry (see
   * `ResultForm`'s `onSubmitted` doc). Turned into a turn by the store, the
   * only place that owns turns and mints ids.
   */
  const handleFormSubmitted = (result: PlanResult): void => {
    submitForm(result);
  };

  return (
    <Stack spacing={3}>
      {turns.length === 0 ? (
        <ExampleQuestions onSelect={handleSubmit} disabled={pending} />
      ) : (
        <>
          <TurnList
            turns={turns}
            onFormSubmitted={handleFormSubmitted}
            renderSaveControl={renderSaveControl}
          />
          <Button variant="outlined" onClick={newConversation} sx={{ alignSelf: "flex-start" }}>
            新しい会話
          </Button>
        </>
      )}
      {error === null ? null : <Alert severity="error">{error}</Alert>}
      <QuestionForm onSubmit={handleSubmit} disabled={pending} />
    </Stack>
  );
}
