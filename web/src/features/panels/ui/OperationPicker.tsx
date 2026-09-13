import Alert from "@mui/material/Alert";
import Autocomplete, { createFilterOptions } from "@mui/material/Autocomplete";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";
import type { JSX } from "react";

import type { CatalogEntry } from "@/shared/api/catalog";

import { fieldOptionsFor } from "../model/panelFieldRules";

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
 * Everything a person might type to find `entry` (K1, `docs/specs/picking.md`
 * section 3): its own display name, its service's, its operation id, its
 * summary, and - the one today's `getOptionLabel`-only filtering misses -
 * the titles of the fields it returns (`fieldOptionsFor`, the same rule
 * `TransformFields`' own dropdowns use, so "数量" finds the operation
 * whose response carries a field titled that, not only one whose own name
 * happens to contain it).
 */
function searchableText(entry: CatalogEntry): string {
  const fieldTitles = fieldOptionsFor(entry).map((field) => field.label);

  return [
    entry.displayName,
    entry.serviceDisplayName,
    entry.operationId,
    entry.summary,
    ...fieldTitles,
  ].join(" ");
}

/**
 * Plain substring, case-insensitive, over `searchableText` - not fuzzy
 * (section 3 argues why: indistinguishable from substring on six options,
 * a ranking problem this does not have on six hundred). `createFilterOptions`
 * over a filter written by hand: its defaults are already exactly this
 * (`ignoreCase: true`, `matchFrom: "any"`, i.e. `String.includes`), so
 * reaching for it is reuse, not reinvention - the piece it does not do by
 * default, matching on more than `getOptionLabel`, is the one point this
 * task changes, via `stringify`.
 */
const filterOptions = createFilterOptions<CatalogEntry>({ stringify: searchableText });

/**
 * Step 1 (`docs/specs/dashboard.md` section 6): every operation the
 * catalogue returned, grouped by service - an MUI `Autocomplete`'s own
 * `groupBy`, not a second grouping function of this feature's own.
 * Grouped by `serviceDisplayName`, never the raw `service` identifier
 * (DECISIONS.md, 2026-09-13) - a contract that declares none falls back
 * to the identifier there, so this never shows blank.
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
        groupBy={(entry) => entry.serviceDisplayName}
        getOptionLabel={(entry) => entry.displayName || entry.operationId}
        filterOptions={filterOptions}
        isOptionEqualToValue={(option, candidate) => entryKey(option) === entryKey(candidate)}
        value={value}
        onChange={(_event, next) => {
          onChange(next);
        }}
        renderOption={(props, entry) => {
          const { key, ...optionProps } = props;

          return (
            <li key={key} {...optionProps}>
              <Stack spacing={0.25} sx={{ minWidth: 0 }}>
                <Typography variant="body2">{entry.displayName || entry.operationId}</Typography>
                {/*
                 * `color="textSecondary"` (camelCase, no dot) - Typography's
                 * own prop accepts `text${Capitalize<keyof TypeText>}`
                 * (`Typography.d.ts`), never the sx-only `"text.secondary"`
                 * dot path every other `color="textSecondary"` in this
                 * codebase actually passes (DECISIONS.md, 2026-09-13): that
                 * string matches no known color and Typography silently
                 * keeps its default (`text.primary`, 0.87 alpha), measured
                 * live here rather than assumed - not a contrast failure
                 * (0.87 alpha only reads darker than 0.6 would), but not
                 * the dimmer second line K2 asks for either.
                 */}
                <Typography variant="caption" color="textSecondary">
                  {entry.summary}
                </Typography>
              </Stack>
            </li>
          );
        }}
        renderInput={(params) => <TextField {...params} label="操作" />}
      />
      {loadError === null ? null : <Alert severity="error">{loadError}</Alert>}
    </>
  );
}
