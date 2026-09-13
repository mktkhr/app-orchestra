import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemText from "@mui/material/ListItemText";
import Typography from "@mui/material/Typography";
import type { JSX } from "react";

import { fieldEntries } from "../model/fieldEntries";
import { cellText, type Fields, type Row } from "../model/rows";

interface ResultDetailProps {
  readonly data: Row;
  /**
   * `PlanResult.fields`: per-property schema, most importantly each enum
   * property's Japanese labels. Undefined for a result that carried none -
   * every property then falls back to its raw key and value (see
   * `columnTitle`/`cellText`, ../model/rows), the same fallback `ResultTable`
   * uses for a column with no schema.
   */
  readonly fields?: Fields | undefined;
}

/**
 * A `kind: "result"` / `component: "detail"` payload: one object rendered
 * as a label/value list (AC-F-102's sibling for a single record - a table
 * row shown on its own). Labels and values come from `columnTitle` and
 * `cellText`, the same lookups `ResultTable` uses for a header and a cell,
 * applied here to one row instead of many rather than reimplemented.
 */
export function ResultDetail({ data, fields }: ResultDetailProps): JSX.Element {
  const entries = fieldEntries(fields, Object.keys(data));

  if (entries.length === 0) {
    return (
      <Typography variant="body2" color="textSecondary">
        表示できる項目がありません。
      </Typography>
    );
  }

  return (
    <List dense disablePadding>
      {entries.map(({ key, label }) => (
        <ListItem key={key} disableGutters>
          <ListItemText primary={label} secondary={cellText(fields, key, data[key])} />
        </ListItem>
      ))}
    </List>
  );
}
