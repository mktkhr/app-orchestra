import MenuItem from "@mui/material/MenuItem";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import type { ChangeEvent, JSX } from "react";

import type { ChartKind } from "@/entities/rendering";

import type { FieldOption } from "../model/usePanelFields";

interface ChartFieldsProps {
  /** The fields `entities/rendering`'s `CatalogEntry.fields` describes - never free text (P2). */
  readonly fieldOptions: readonly FieldOption[];
  readonly category: string;
  readonly value: string;
  readonly kind: ChartKind;
  readonly onCategoryChange: (value: string) => void;
  readonly onValueChange: (value: string) => void;
  readonly onKindChange: (value: string) => void;
}

const KIND_OPTIONS: readonly { readonly value: ChartKind; readonly label: string }[] = [
  { value: "bar", label: "棒グラフ" },
  { value: "line", label: "折れ線グラフ" },
  { value: "pie", label: "円グラフ" },
];

/**
 * Step 4's chart half (`docs/specs/dashboard.md` section 6, P2): category,
 * value and kind, offering only the fields the catalogue entry's `fields`
 * describes, so a saved panel's axes always name a field that exists.
 * Shown only while `component === "chart"` (`AddPanelForm` decides that);
 * a person who picked `table` never sees this.
 */
export function ChartFields({
  fieldOptions,
  category,
  value,
  kind,
  onCategoryChange,
  onValueChange,
  onKindChange,
}: ChartFieldsProps): JSX.Element {
  return (
    <Stack spacing={2}>
      <TextField
        select
        label="分類の軸"
        value={category}
        onChange={(event: ChangeEvent<HTMLInputElement>) => {
          onCategoryChange(event.target.value);
        }}
      >
        {fieldOptions.map((field) => (
          <MenuItem key={field.value} value={field.value}>
            {field.label}
          </MenuItem>
        ))}
      </TextField>
      <TextField
        select
        label="値の軸"
        value={value}
        onChange={(event: ChangeEvent<HTMLInputElement>) => {
          onValueChange(event.target.value);
        }}
      >
        {fieldOptions.map((field) => (
          <MenuItem key={field.value} value={field.value}>
            {field.label}
          </MenuItem>
        ))}
      </TextField>
      <TextField
        select
        label="グラフの種類"
        value={kind}
        onChange={(event: ChangeEvent<HTMLInputElement>) => {
          onKindChange(event.target.value);
        }}
      >
        {KIND_OPTIONS.map((option) => (
          <MenuItem key={option.value} value={option.value}>
            {option.label}
          </MenuItem>
        ))}
      </TextField>
    </Stack>
  );
}
