import TablePagination from "@mui/material/TablePagination";
import type { JSX } from "react";

export const RESULT_TABLE_ROWS_PER_PAGE = 10;

interface ResultTablePaginationProps {
  readonly rowCount: number;
  readonly page: number;
  readonly onPageChange: (page: number) => void;
}

/**
 * Pagination for a `ResultTable`, fixed at
 * {@link RESULT_TABLE_ROWS_PER_PAGE} rows (AC-F-101). The control offers no
 * other page size, so `onRowsPerPageChange` exists only to satisfy
 * `TablePagination`'s props and never fires with a different value.
 */
export function ResultTablePagination({
  rowCount,
  page,
  onPageChange,
}: ResultTablePaginationProps): JSX.Element {
  return (
    <TablePagination
      component="div"
      count={rowCount}
      page={page}
      rowsPerPage={RESULT_TABLE_ROWS_PER_PAGE}
      rowsPerPageOptions={[RESULT_TABLE_ROWS_PER_PAGE]}
      onPageChange={(_event, newPage) => {
        onPageChange(newPage);
      }}
      onRowsPerPageChange={() => {
        // no-op: RESULT_TABLE_ROWS_PER_PAGE is the only option.
      }}
      labelRowsPerPage="1ページの行数"
      labelDisplayedRows={({ from, to, count }) =>
        `${String(count)}件中 ${String(from)}〜${String(to)}件`
      }
      getItemAriaLabel={(type) => (type === "previous" ? "前のページ" : "次のページ")}
    />
  );
}
