import Paper from "@mui/material/Paper";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableContainer from "@mui/material/TableContainer";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import type { JSX } from "react";

import { cellText, columnTitle, type Fields, type Row } from "../model/rows";

export type { Fields, Row };

interface ResultTableGridProps {
  readonly columns: readonly string[];
  readonly rows: readonly Row[];
  /**
   * Per-column schema, from `PlanResult.fields`. Undefined when the result
   * carried none (see `usecase.fieldsFor`, services/platform), in which
   * case every column falls back to its raw key and every cell to its raw
   * value.
   */
  readonly fields?: Fields | undefined;
}

/**
 * The rows of a table result, as an MUI `Table`.
 *
 * `flexGrow: 1` / `minHeight: 0` / `overflow: "auto"` are what let this be
 * the one scroller a panel's table draws (`ResultTable`'s own doc comment,
 * `docs/specs/dashboard.md` P15): inside `ResultTable`'s `height: "100%"`
 * `Stack`, this fills whatever is left after the "拡大表示" button and the
 * pagination below it, and scrolls its own rows - both vertically, when
 * there are more than fit, and sideways (`TableContainer`'s own default),
 * when a column set is wider than the box, rather than the page
 * (`make guard-layout`). Outside that `Stack` (the chat, `ResultTableDialog`)
 * `flexGrow`/`minHeight` are flex-only properties that no-op without a flex
 * parent, so this draws exactly as it did before there.
 *
 * `tabIndex={0}` is what keeps that scrolling reachable by keyboard, not
 * only by pointer or touch: a bounded, overflowing region with no
 * focusable element inside the part that scrolls is
 * `make guard-a11y`'s own `scrollable-region-focusable` violation
 * (axe-core, WCAG 2.1.1) - harmless before this file could ever actually
 * overflow, and live the moment P15 gave it a height to overflow inside
 * (DECISIONS.md, 2026-09-13).
 */
export function ResultTableGrid({ columns, rows, fields }: ResultTableGridProps): JSX.Element {
  return (
    <TableContainer
      component={Paper}
      variant="outlined"
      tabIndex={0}
      sx={{ flexGrow: 1, minHeight: 0, overflow: "auto" }}
    >
      <Table size="small">
        <TableHead>
          <TableRow>
            {columns.map((column) => (
              <TableCell key={column}>{columnTitle(fields, column)}</TableCell>
            ))}
          </TableRow>
        </TableHead>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={JSON.stringify(row)}>
              {columns.map((column) => (
                <TableCell key={column}>{cellText(fields, column, row[column])}</TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  );
}
