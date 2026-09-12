import Alert from "@mui/material/Alert";
import Autocomplete from "@mui/material/Autocomplete";
import TextField from "@mui/material/TextField";
import type { JSX } from "react";

import type { CatalogEntry } from "@/shared/api/catalog";

interface OperationPickerProps {
  readonly entries: readonly CatalogEntry[];
  readonly value: CatalogEntry | null;
  readonly onChange: (entry: CatalogEntry | null) => void;
  readonly loadError: string | null;
}

function entryKey(entry: CatalogEntry): string {
  return `${entry.service}:${entry.operationId}`;
}

/**
 * Step 1 (`docs/specs/dashboard.md` section 6): every operation the
 * catalogue returned, grouped by service - an MUI `Autocomplete`'s own
 * `groupBy`, not a second grouping function of this feature's own.
 */
export function OperationPicker({
  entries,
  value,
  onChange,
  loadError,
}: OperationPickerProps): JSX.Element {
  return (
    <>
      <Autocomplete
        options={entries}
        groupBy={(entry) => entry.service}
        getOptionLabel={(entry) => entry.displayName || entry.operationId}
        isOptionEqualToValue={(option, candidate) => entryKey(option) === entryKey(candidate)}
        value={value}
        onChange={(_event, next) => {
          onChange(next);
        }}
        renderInput={(params) => <TextField {...params} label="操作" />}
      />
      {loadError === null ? null : <Alert severity="error">{loadError}</Alert>}
    </>
  );
}
