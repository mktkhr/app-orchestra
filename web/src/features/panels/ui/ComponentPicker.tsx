import MenuItem from "@mui/material/MenuItem";
import TextField from "@mui/material/TextField";
import type { ChangeEvent, JSX } from "react";

import type { Component } from "@/shared/api/client";

interface ComponentPickerProps {
  readonly options: readonly Component[];
  readonly value: Component;
  readonly onChange: (value: string) => void;
}

const COMPONENT_LABELS: Record<Component, string> = {
  table: "表",
  detail: "詳細",
  form: "フォーム",
  choice: "選択",
  chart: "グラフ",
};

/**
 * Step 3 (`docs/specs/dashboard.md` section 6): which widget draws the
 * panel. The rule's own answer (the catalogue entry's `component`) is the
 * only option unless the entry's `fields` also makes a chart possible (P2)
 * - `usePanelBuilder`'s `componentOptions` decides which options exist,
 * this only draws them.
 */
export function ComponentPicker({ options, value, onChange }: ComponentPickerProps): JSX.Element {
  return (
    <TextField
      select
      label="表示方法"
      value={value}
      onChange={(event: ChangeEvent<HTMLInputElement>) => {
        onChange(event.target.value);
      }}
    >
      {options.map((option) => (
        <MenuItem key={option} value={option}>
          {COMPONENT_LABELS[option]}
        </MenuItem>
      ))}
    </TextField>
  );
}
