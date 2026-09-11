import FullscreenIcon from "@mui/icons-material/Fullscreen";
import Button from "@mui/material/Button";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import { useState, type JSX } from "react";

import type { PlanResult } from "@/shared/api/client";

import { rowsFromData } from "../model/rows";
import { ResultTableDialog } from "./ResultTableDialog";
import { ResultTableGrid } from "./ResultTableGrid";
import { RESULT_TABLE_ROWS_PER_PAGE, ResultTablePagination } from "./ResultTablePagination";

interface ResultTableProps {
  readonly data: NonNullable<PlanResult["data"]>;
}

/**
 * A `kind: "result"` / `component: "table"` payload, paginated (AC-F-101)
 * with a control that expands the same rows into a full-screen dialog
 * (AC-F-105).
 *
 * Columns come from the keys of the first row. `/api/plan` carries no column
 * schema alongside `data` (confirmed against the running platform — see
 * Task 13's report), so headers are the keys themselves; there is no `title`
 * to prefer.
 */
export function ResultTable({ data }: ResultTableProps): JSX.Element {
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
      <ResultTableGrid columns={columns} rows={pageRows} />
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
      />
    </Stack>
  );
}
