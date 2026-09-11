import CloseIcon from "@mui/icons-material/Close";
import AppBar from "@mui/material/AppBar";
import Box from "@mui/material/Box";
import Dialog from "@mui/material/Dialog";
import IconButton from "@mui/material/IconButton";
import Toolbar from "@mui/material/Toolbar";
import Typography from "@mui/material/Typography";
import type { JSX } from "react";

import { ResultTableGrid, type Fields, type Row } from "./ResultTableGrid";
import { ResultTablePagination } from "./ResultTablePagination";

interface ResultTableDialogProps {
  readonly open: boolean;
  readonly onClose: () => void;
  readonly columns: readonly string[];
  readonly rows: readonly Row[];
  readonly rowCount: number;
  readonly page: number;
  readonly onPageChange: (page: number) => void;
  readonly fields?: Fields | undefined;
}

/** The same table rows, expanded into a full-screen modal (AC-F-105). */
export function ResultTableDialog({
  open,
  onClose,
  columns,
  rows,
  rowCount,
  page,
  onPageChange,
  fields,
}: ResultTableDialogProps): JSX.Element {
  return (
    <Dialog fullScreen open={open} onClose={onClose}>
      <AppBar position="relative">
        <Toolbar>
          <IconButton edge="start" color="inherit" aria-label="閉じる" onClick={onClose}>
            <CloseIcon />
          </IconButton>
          <Typography variant="h6" sx={{ ml: 2 }}>
            結果を拡大表示
          </Typography>
        </Toolbar>
      </AppBar>
      <Box sx={{ p: 2 }}>
        <ResultTableGrid columns={columns} rows={rows} fields={fields} />
        <ResultTablePagination rowCount={rowCount} page={page} onPageChange={onPageChange} />
      </Box>
    </Dialog>
  );
}
