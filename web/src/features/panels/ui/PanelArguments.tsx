import { useEffect, type Dispatch, type JSX, type SetStateAction } from "react";

import { ResultFormFields, useFormValues } from "@/entities/rendering";

interface PanelArgumentsProps {
  readonly schema: Record<string, unknown>;
  readonly onChange: Dispatch<SetStateAction<Record<string, unknown>>>;
}

/**
 * Step 2 (`docs/specs/dashboard.md` section 6, P7): the same controls
 * `ResultForm` draws for a `kind: "form"` answer, over a catalogue entry's
 * own `schema` - `useFormValues`/`ResultFormFields`, both reused unchanged
 * from `entities/rendering`, rather than a second form.
 *
 * The parent (`AddPanelForm`) remounts this component - `key={service:operationId}` -
 * whenever the chosen operation changes, so a fresh `useFormValues` reseeds
 * from the new schema instead of carrying over the previous operation's
 * values. `onChange` is the parent's own `useState` setter, passed through
 * unwrapped, so this effect's dependency stays referentially stable and
 * fires only when `values` itself changes.
 */
export function PanelArguments({ schema, onChange }: PanelArgumentsProps): JSX.Element {
  const { properties, required, entries, values, setValue } = useFormValues(schema);

  useEffect(() => {
    onChange(values);
  }, [values, onChange]);

  return (
    <ResultFormFields
      entries={entries}
      properties={properties}
      required={required}
      values={values}
      onChange={setValue}
    />
  );
}
