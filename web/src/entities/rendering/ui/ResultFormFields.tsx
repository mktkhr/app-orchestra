import Stack from "@mui/material/Stack";
import type { JSX } from "react";

import type { FieldEntry } from "../model/fieldEntries";
import type { Fields } from "../model/rows";
import { ResultFormField } from "./ResultFormField";

interface ResultFormFieldsProps {
  readonly entries: readonly FieldEntry[];
  readonly properties: Fields;
  readonly required: readonly string[];
  readonly values: Record<string, unknown>;
  readonly onChange: (key: string, value: unknown) => void;
  readonly onClear: (key: string) => void;
}

/**
 * The list of controls a schema's properties become - one `ResultFormField`
 * per entry, exactly as `ResultForm` draws its own. Split out of that
 * component (`docs/plans/dashboard.md` Task 6, P7) so `features/panels`'
 * builder can draw the identical controls over a catalogue entry's `schema`
 * without a second form and without a submit button it does not want -
 * paired with `useFormValues` for the state these controls read and write.
 */
export function ResultFormFields({
  entries,
  properties,
  required,
  values,
  onChange,
  onClear,
}: ResultFormFieldsProps): JSX.Element {
  return (
    <Stack spacing={2}>
      {entries.map(({ key, label }) => (
        <ResultFormField
          key={key}
          fieldKey={key}
          label={label}
          fields={properties}
          required={required.includes(key)}
          value={values[key]}
          onChange={onChange}
          onClear={onClear}
        />
      ))}
    </Stack>
  );
}
