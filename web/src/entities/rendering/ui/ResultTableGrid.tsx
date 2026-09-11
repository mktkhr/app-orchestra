import Paper from "@mui/material/Paper";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableContainer from "@mui/material/TableContainer";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import type { JSX } from "react";

import { formatCellValue, type Row } from "../model/rows";

export type { Row };

interface ResultTableGridProps {
  readonly columns: readonly string[];
  readonly rows: readonly Row[];
}

/** The rows of a table result, as an MUI `Table`. */
export function ResultTableGrid({ columns, rows }: ResultTableGridProps): JSX.Element {
  return (
    <TableContainer component={Paper} variant="outlined">
      <Table size="small">
        <TableHead>
          <TableRow>
            {columns.map((column) => (
              <TableCell key={column}>{column}</TableCell>
            ))}
          </TableRow>
        </TableHead>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={JSON.stringify(row)}>
              {columns.map((column) => (
                <TableCell key={column}>{formatCellValue(row[column])}</TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  );
}
