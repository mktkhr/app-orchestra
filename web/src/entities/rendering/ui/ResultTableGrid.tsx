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

/** The rows of a table result, as an MUI `Table`. */
export function ResultTableGrid({ columns, rows, fields }: ResultTableGridProps): JSX.Element {
  return (
    <TableContainer component={Paper} variant="outlined">
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
