import FormControlLabel from "@mui/material/FormControlLabel";
import MenuItem from "@mui/material/MenuItem";
import Switch from "@mui/material/Switch";
import TextField from "@mui/material/TextField";
import type { ChangeEvent, JSX } from "react";

import { enumOptions, fieldSchema, type Fields } from "../model/rows";

interface ResultFormFieldProps {
  readonly fieldKey: string;
  readonly label: string;
  readonly fields: Fields;
  readonly required: boolean;
  readonly value: unknown;
  readonly onChange: (key: string, value: unknown) => void;
}

/**
 * One `ResultForm` control, chosen by the property's schema: `enum` ->
 * `TextField select`, `boolean` -> `Switch`, `integer`/`number` -> a numeric
 * `TextField`, anything else -> a plain `TextField`. Split out of
 * `ResultForm.tsx` to keep that file's own import count under
 * `max-dependencies` and its body to the form's own concerns (state,
 * submission), not every control's markup.
 */
export function ResultFormField({
  fieldKey,
  label,
  fields,
  required,
  value,
  onChange,
}: ResultFormFieldProps): JSX.Element {
  const type = fieldSchema(fields, fieldKey)?.["type"];
  const options = enumOptions(fields, fieldKey);

  if (type === "boolean") {
    return (
      <FormControlLabel
        control={
          <Switch
            checked={Boolean(value)}
            onChange={(event: ChangeEvent<HTMLInputElement>) => {
              onChange(fieldKey, event.target.checked);
            }}
          />
        }
        label={required ? `${label}（必須）` : label}
      />
    );
  }

  if (options.length > 0) {
    return (
      <TextField
        select
        label={label}
        required={required}
        value={typeof value === "string" ? value : ""}
        onChange={(event: ChangeEvent<HTMLInputElement>) => {
          onChange(fieldKey, event.target.value);
        }}
      >
        {options.map((option) => (
          <MenuItem key={option.value} value={option.value}>
            {option.label}
          </MenuItem>
        ))}
      </TextField>
    );
  }

  if (type === "integer" || type === "number") {
    return (
      <TextField
        type="number"
        label={label}
        required={required}
        value={typeof value === "number" ? value : ""}
        onChange={(event: ChangeEvent<HTMLInputElement>) => {
          const parsed = Number(event.target.value);

          onChange(fieldKey, Number.isNaN(parsed) ? 0 : parsed);
        }}
      />
    );
  }

  return (
    <TextField
      label={label}
      required={required}
      value={typeof value === "string" ? value : ""}
      onChange={(event: ChangeEvent<HTMLInputElement>) => {
        onChange(fieldKey, event.target.value);
      }}
    />
  );
}
