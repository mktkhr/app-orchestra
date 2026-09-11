import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import { useRef, useState, type JSX } from "react";

import { postPlan, type PlanResult } from "@/shared/api/client";

import type { EnumOption } from "../model/rows";
import { useSubmission } from "@/shared/lib/useSubmission";

type Options = NonNullable<PlanResult["options"]>;

/**
 * `PlanResult.options`, read defensively rather than iterated directly.
 *
 * openapi-fetch's `MethodResponse` mapping turns a nested array-of-objects
 * schema property into a type whose own prototype methods (`map`, `filter`,
 * ...) type-check as `{}` - calling `.map` on it directly is a compile
 * error, not just an unsafe read (see `ResultForm`'s comment on
 * `PlanResult["target"]`/`["schema"]` for the same mapping's effect on
 * plain objects). `Array.isArray` takes an `any` parameter and narrows to a
 * real `any[]`, which is what restores working array methods; the shape
 * check afterwards is the same defensive-parsing style `ResultForm`
 * (`asRecord`/`asStringArray`) and `rows.ts` (`isRecord`/`isRow`) already
 * use for a value whose declared type cannot simply be trusted as-is.
 */
function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function toEnumOptions(options: Options): readonly EnumOption[] {
  if (!Array.isArray(options)) {
    return [];
  }

  return options.flatMap((option: unknown) => {
    if (!isRecord(option)) {
      return [];
    }

    const { value, label } = option;

    return typeof value === "string" && typeof label === "string" ? [{ value, label }] : [];
  });
}

interface ResultChoiceProps {
  /** The planner's disambiguation question, shown above the options. */
  readonly question: string;
  /** The parameter the chosen value fills in. */
  readonly param: string;
  /**
   * Kept as the exact type `PlanResult` produces (`NonNullable<...>`)
   * rather than re-derived into a plain `{value, label}[]`: see
   * `toEnumOptions` above for why this is read defensively instead of
   * mapped over directly.
   */
  readonly options: Options;
  /**
   * What the person actually typed. `PlanResult.question` (above) is the
   * planner's own prompt, not this - an `ask` answer carries no copy of the
   * original query, so `TurnList` walks the turn list back to the nearest
   * preceding question turn and passes its text down here. See that
   * component for why.
   */
  readonly originalQuery: string;
  /**
   * Called once `POST /api/plan` (resubmitted with `answers`) returns
   * successfully, with the `PlanResult` it answered - `result`, `form`,
   * `ask` again, or `none`. `ResultChoice` does not decide what happens
   * next; the caller (`TurnList`, by way of `Conversation`) turns it into a
   * new turn the same way it already does for `ResultForm`'s
   * `onSubmitted`.
   */
  readonly onAnswered: (result: PlanResult) => void;
}

/**
 * For `kind: "ask"` (AC-F-103): the question plus one control per option,
 * each showing its Japanese label. Picking one re-posts `originalQuery`
 * alongside `answers: [{param, value}]` and reports whatever comes back.
 *
 * Once an answer is chosen - in flight or already answered - the whole
 * option row is locked against further clicks (`locked`, below) rather than
 * using MUI's `disabled` prop: a disabled `contained` `Button` has no
 * border of its own and fails `make guard-layout`'s contrast check (WCAG
 * 1.4.11) at rest, the same reason `ResultForm`'s submit button is never
 * disabled by validation - only in flight, and even then not through
 * `disabled`. `pointerEvents`/`opacity` on the row give the same visual and
 * behavioural result without touching that prop. The chosen option itself
 * stays visible afterwards, drawn `contained` while its siblings stay
 * `outlined`, because the answer it produced becomes a new turn right below
 * - hiding or renaming the options would leave no record of what was
 * actually asked.
 */
export function ResultChoice({
  question,
  param,
  options,
  originalQuery,
  onAnswered,
}: ResultChoiceProps): JSX.Element {
  const { submitting, error, run } = useSubmission();
  const [answeredValue, setAnsweredValue] = useState<string | null>(null);
  const locked = submitting || answeredValue !== null;
  const enumOptions = toEnumOptions(options);
  // A second click can land before React has re-rendered with `submitting`
  // true - `locked` alone is a render away, not an event away. `inFlight`
  // is set the instant a click is accepted and cleared once the request
  // settles, so the guard below is exact regardless of render timing; the
  // state-derived `locked` above still drives what the row looks like.
  const inFlight = useRef(false);

  const select = (value: string): Promise<void> =>
    run(async () => {
      const result = await postPlan({ query: originalQuery, answers: [{ param, value }] });

      setAnsweredValue(value);
      onAnswered(result);
    });

  const handleClick = (value: string): void => {
    if (answeredValue !== null || inFlight.current) {
      return;
    }

    inFlight.current = true;
    void select(value).finally(() => {
      inFlight.current = false;
    });
  };

  return (
    <Stack spacing={1.5}>
      <Typography variant="body1">{question}</Typography>
      <Stack
        direction="row"
        spacing={1}
        sx={{
          flexWrap: "wrap",
          opacity: locked ? 0.7 : 1,
          pointerEvents: locked ? "none" : "auto",
        }}
      >
        {enumOptions.map((option) => (
          <Button
            key={option.value}
            variant={option.value === answeredValue ? "contained" : "outlined"}
            onClick={() => {
              handleClick(option.value);
            }}
          >
            {option.label}
          </Button>
        ))}
      </Stack>
      {error === null ? null : <Alert severity="error">{error}</Alert>}
    </Stack>
  );
}
