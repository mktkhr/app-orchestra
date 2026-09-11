import Alert from "@mui/material/Alert";
import Stack from "@mui/material/Stack";
import { useState, type JSX, type ReactNode } from "react";

import { postPlan, type PlanResult } from "@/shared/api/client";
import { nextTurnId } from "@/shared/lib/turnId";

import type { Turn } from "../model/turn";
import { ExampleQuestions } from "./ExampleQuestions";
import { QuestionForm } from "./QuestionForm";
import { TurnList, type SaveControlSlotProps } from "./TurnList";

interface ConversationProps {
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
export function Conversation({ renderSaveControl }: ConversationProps = {}): JSX.Element {
  const [turns, setTurns] = useState<readonly Turn[]>([]);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const ask = async (query: string): Promise<void> => {
    setError(null);
    setPending(true);
    setTurns((current) => [...current, { id: nextTurnId(), role: "question", text: query }]);

    try {
      const result = await postPlan({ query });

      setTurns((current) => [...current, { id: nextTurnId(), role: "answer", result }]);
    } catch {
      setError("質問の送信に失敗しました。時間をおいて試してください。");
    } finally {
      setPending(false);
    }
  };

  const handleSubmit = (query: string): void => {
    void ask(query);
  };

  /**
   * `ResultForm`'s successful `/api/invoke` result, already shaped as the
   * `PlanResult` a `kind: "result"` answer would carry (see
   * `ResultForm`'s `onSubmitted` doc). Turned into a turn here, the only
   * place that owns `turns` and mints ids.
   */
  const handleFormSubmitted = (result: PlanResult): void => {
    setTurns((current) => [...current, { id: nextTurnId(), role: "answer", result }]);
  };

  return (
    <Stack spacing={3}>
      {turns.length === 0 ? (
        <ExampleQuestions onSelect={handleSubmit} disabled={pending} />
      ) : (
        <TurnList
          turns={turns}
          onFormSubmitted={handleFormSubmitted}
          renderSaveControl={renderSaveControl}
        />
      )}
      {error === null ? null : <Alert severity="error">{error}</Alert>}
      <QuestionForm onSubmit={handleSubmit} disabled={pending} />
    </Stack>
  );
}
