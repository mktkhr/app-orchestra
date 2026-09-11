import FullscreenIcon from "@mui/icons-material/Fullscreen";
import Button from "@mui/material/Button";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import { useState, type JSX } from "react";

import type { PlanResult } from "@/shared/api/client";

import { rowsFromData, type Fields } from "../model/rows";
import { ResultTableDialog } from "./ResultTableDialog";
import { ResultTableGrid } from "./ResultTableGrid";
import { RESULT_TABLE_ROWS_PER_PAGE, ResultTablePagination } from "./ResultTablePagination";

interface ResultTableProps {
  readonly data: NonNullable<PlanResult["data"]>;
  /**
   * `PlanResult.fields`: per-column schema, most importantly each enum
   * column's Japanese labels. Undefined for a result that carried none —
   * every column then falls back to its raw key and cell to its raw value
   * (see `columnTitle`/`cellText`, ../model/rows).
   */
  readonly fields?: Fields | undefined;
}

/**
 * A `kind: "result"` / `component: "table"` payload, paginated (AC-F-101)
 * with a control that expands the same rows into a full-screen dialog
 * (AC-F-105).
 *
 * Columns come from the keys of the first row. A header prefers
 * `fields[column].title` when the schema declares one, and a cell prefers
 * `fields[column].enumLabels[value]` when the column is an enum with a
 * label for that value — otherwise both fall back to the raw key/value.
 */
export function ResultTable({ data, fields }: ResultTableProps): JSX.Element {
  const rows = rowsFromData(data);
  const [page, setPage] = useState(0);
  const [expanded, setExpanded] = useState(false);
  const [firstRow] = rows;

  if (firstRow === undefined) {
    return (
      <Typography variant="body2" color="text.secondary">
        結果は0件です。
      </Typography>
    );
  }

  const columns = Object.keys(firstRow);
  const pageRows = rows.slice(
    page * RESULT_TABLE_ROWS_PER_PAGE,
    page * RESULT_TABLE_ROWS_PER_PAGE + RESULT_TABLE_ROWS_PER_PAGE,
  );

  return (
    <Stack spacing={1}>
      <Stack direction="row" sx={{ justifyContent: "flex-end" }}>
        <Button
          size="small"
          startIcon={<FullscreenIcon />}
          onClick={() => {
            setExpanded(true);
          }}
        >
          拡大表示
        </Button>
      </Stack>
      <ResultTableGrid columns={columns} rows={pageRows} fields={fields} />
      <ResultTablePagination rowCount={rows.length} page={page} onPageChange={setPage} />
      <ResultTableDialog
        open={expanded}
        onClose={() => {
          setExpanded(false);
        }}
        columns={columns}
        rows={pageRows}
        rowCount={rows.length}
        page={page}
        onPageChange={setPage}
        fields={fields}
      />
    </Stack>
  );
}
