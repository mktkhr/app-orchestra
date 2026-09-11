import Alert from "@mui/material/Alert";
import Stack from "@mui/material/Stack";
import { useState, type JSX } from "react";

import { postPlan } from "@/shared/api/client";
import { nextTurnId } from "@/shared/lib/turnId";

import type { Turn } from "../model/turn";
import { ExampleQuestions } from "./ExampleQuestions";
import { QuestionForm } from "./QuestionForm";
import { TurnList } from "./TurnList";

/**
 * The chat conversation: the turn list, the question input, and - before the
 * first question - the example questions (AC-F-104). See
 * docs/specs/orchestration.md section 7.
 */
export function Conversation(): JSX.Element {
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

  return (
    <Stack spacing={3}>
      {turns.length === 0 ? (
        <ExampleQuestions onSelect={handleSubmit} disabled={pending} />
      ) : (
        <TurnList turns={turns} />
      )}
      {error === null ? null : <Alert severity="error">{error}</Alert>}
      <QuestionForm onSubmit={handleSubmit} disabled={pending} />
    </Stack>
  );
}
