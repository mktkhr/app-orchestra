import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import Stack from "@mui/material/Stack";
import type { JSX, SyntheticEvent } from "react";

import { postInvoke, type PlanResult } from "@/shared/api/client";

import { useFormValues } from "../model/useFormValues";
import { useSubmission } from "@/shared/lib/useSubmission";
import { ResultFormFields } from "./ResultFormFields";

type Target = NonNullable<PlanResult["target"]>;
type Schema = NonNullable<PlanResult["schema"]>;

interface ResultFormProps {
  readonly schema: Schema;
  readonly initial?: NonNullable<PlanResult["initial"]> | undefined;
  readonly target: Target;
  /**
   * Called once `POST /api/invoke` returns successfully, with a
   * `PlanResult` shaped exactly like a `kind: "result"` answer from
   * `/api/plan`. `ResultForm` lives in `entities/rendering` and cannot
   * import `Turn` from `features/conversation` - features are siblings, and
   * entities sit below them - so it hands back a value shaped by a type
   * `entities` already depends on (`shared/api`'s `PlanResult`) instead of
   * reaching for one it does not own. The caller (`TurnList`, by way of a
   * callback `Conversation` passes down) wraps it into a new turn with
   * `nextTurnId()` the same way it wraps any other answer.
   */
  readonly onSubmitted: (result: PlanResult) => void;
}

/**
 * A `kind: "form"` payload's inputs, derived from its JSON Schema
 * (AC-F-102): a `string` property becomes a `TextField`, an enum property
 * becomes a `TextField select` with one `MenuItem` per option (value from
 * the enum, label from `enumLabels`), `integer`/`number` becomes a numeric
 * `TextField`, `boolean` becomes a `Switch` - see `ResultFormField` for the
 * per-property choice. Every value starts at `initial`, and a property
 * named in `schema.required` marks its control required.
 *
 * The controls themselves and their state come from `useFormValues`/
 * `ResultFormFields` (`docs/plans/dashboard.md` Task 6, P7) - shared with
 * `features/panels`' builder, which draws the same controls over a
 * catalogue entry's `schema` without invoking anything.
 *
 * Submitting posts the edited values to `/api/invoke` and reports the
 * result through `onSubmitted` - this component does not itself decide
 * where the result goes. The submit button is never disabled by empty
 * input, only while a submission is in flight: a disabled MUI `contained`
 * button has no border of its own and fails `make guard-layout`'s contrast
 * check (WCAG 1.4.11) at rest.
 */
export function ResultForm({ schema, initial, target, onSubmitted }: ResultFormProps): JSX.Element {
  const { properties, required, entries, values, setValue, clearValue } = useFormValues(
    schema,
    initial,
  );
  const { submitting, error, run } = useSubmission();

  const submit = (): Promise<void> =>
    run(async () => {
      const result = await postInvoke({
        service: target.service,
        operationId: target.operationId,
        args: values,
      });

      onSubmitted({
        kind: "result",
        component: result.component,
        data: result.data,
        // Spread rather than assign: under exactOptionalPropertyTypes an
        // optional property does not accept an explicit undefined, and an
        // invoke result carries no fields when nothing can describe them.
        ...(result.fields === undefined ? {} : { fields: result.fields }),
        source: {
          service: target.service,
          serviceDisplayName: target.serviceDisplayName,
          operationId: target.operationId,
          args: values,
        },
      });
    });

  const handleSubmit = (event: SyntheticEvent<HTMLFormElement>): void => {
    event.preventDefault();

    if (submitting) {
      return;
    }

    void submit();
  };

  return (
    <Box component="form" onSubmit={handleSubmit}>
      <Stack spacing={2}>
        <ResultFormFields
          entries={entries}
          properties={properties}
          required={required}
          values={values}
          onChange={setValue}
          onClear={clearValue}
        />
        {error === null ? null : <Alert severity="error">{error}</Alert>}
        <Box>
          <Button type="submit" variant="contained" disabled={submitting}>
            送信
          </Button>
        </Box>
      </Stack>
    </Box>
  );
}
