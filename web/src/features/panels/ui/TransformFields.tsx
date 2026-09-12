import FormControlLabel from "@mui/material/FormControlLabel";
import MenuItem from "@mui/material/MenuItem";
import Stack from "@mui/material/Stack";
import Switch from "@mui/material/Switch";
import TextField from "@mui/material/TextField";
import type { ChangeEvent, JSX } from "react";

import { AGGREGATE_LABELS, type Aggregate } from "@/entities/rendering";

import type { FieldOption } from "../model/usePanelFields";

interface TransformFieldsProps {
  /** The fields the catalogue entry's `fields` describes - `groupBy`'s own candidates (P2). */
  readonly fieldOptions: readonly FieldOption[];
  readonly enabled: boolean;
  readonly groupBy: string;
  readonly aggregate: Aggregate;
  readonly aggregateField: string;
  readonly onEnabledChange: (enabled: boolean) => void;
  readonly onGroupByChange: (value: string) => void;
  readonly onAggregateChange: (value: string) => void;
  readonly onAggregateFieldChange: (value: string) => void;
}

const AGGREGATE_OPTIONS: readonly { readonly value: Aggregate; readonly label: string }[] = [
  { value: "count", label: AGGREGATE_LABELS.count },
  { value: "sum", label: AGGREGATE_LABELS.sum },
  { value: "avg", label: AGGREGATE_LABELS.avg },
];

/**
 * Step 4's transform half (`docs/specs/dashboard.md` section 4) - one
 * optional step, off by default. Its own controls appear only once the
 * switch is on, and `field` only once `aggregate` needs one (`count` does
 * not), so a panel that wants none of this shows nothing beyond the switch.
 */
export function TransformFields({
  fieldOptions,
  enabled,
  groupBy,
  aggregate,
  aggregateField,
  onEnabledChange,
  onGroupByChange,
  onAggregateChange,
  onAggregateFieldChange,
}: TransformFieldsProps): JSX.Element {
  return (
    <Stack spacing={2}>
      <FormControlLabel
        control={
          <Switch
            checked={enabled}
            onChange={(event: ChangeEvent<HTMLInputElement>) => {
              onEnabledChange(event.target.checked);
            }}
          />
        }
        label="集計してから描画する"
      />
      {enabled && (
        <>
          <TextField
            select
            label="グループ化する項目"
            value={groupBy}
            onChange={(event: ChangeEvent<HTMLInputElement>) => {
              onGroupByChange(event.target.value);
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
            label="集計方法"
            value={aggregate}
            onChange={(event: ChangeEvent<HTMLInputElement>) => {
              onAggregateChange(event.target.value);
            }}
          >
            {AGGREGATE_OPTIONS.map((option) => (
              <MenuItem key={option.value} value={option.value}>
                {option.label}
              </MenuItem>
            ))}
          </TextField>
          {aggregate !== "count" && (
            <TextField
              select
              label="集計する項目"
              value={aggregateField}
              onChange={(event: ChangeEvent<HTMLInputElement>) => {
                onAggregateFieldChange(event.target.value);
              }}
            >
              {fieldOptions.map((field) => (
                <MenuItem key={field.value} value={field.value}>
                  {field.label}
                </MenuItem>
              ))}
            </TextField>
          )}
        </>
      )}
    </Stack>
  );
}
