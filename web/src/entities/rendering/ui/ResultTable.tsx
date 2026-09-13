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
 *
 * The outer `Stack` carries `height: "100%"` and `ResultTableGrid`'s own
 * `TableContainer` carries `flexGrow: 1` / `overflow: "auto"`
 * (`docs/specs/dashboard.md` P15, DECISIONS.md 2026-09-13): inside a
 * panel, whose card gives this a definite height to fill, that makes the
 * rows scroll in their own box while the "拡大表示" button above and the
 * pagination below stay in place - one scroller, not the card's and the
 * table's both, and the pagination stays reachable rather than scrolling
 * away with the rows. Outside a panel (the chat, where nothing bounds this
 * component's height) `height: "100%"` resolves against an indefinite
 * ancestor and is a no-op, so this draws exactly as it did before there.
 *
 * The button row and `ResultTablePagination` both carry `flexShrink: 0`:
 * a flex item shrinks by default, and `TableContainer` is meant to be the
 * only one of the three that gives up height when the three together do
 * not fit the `Stack`'s own. Without it, `MuiTablePagination-root` -
 * whose MUI-own styles already carry `overflow: "auto"` unconditionally -
 * gets squeezed below its content's natural height and genuinely
 * overflows itself, which is a second scrollbar exactly as unreachable by
 * keyboard as the one P15 already removed (`make guard-a11y`'s own
 * `scrollable-region-focusable`, DECISIONS.md 2026-09-13).
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
    <Stack spacing={1} sx={{ height: "100%", minHeight: 0 }}>
      <Stack direction="row" sx={{ justifyContent: "flex-end", flexShrink: 0 }}>
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
      <ResultTablePagination
        rowCount={rows.length}
        page={page}
        onPageChange={setPage}
        sx={{ flexShrink: 0 }}
      />
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
