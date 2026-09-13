import Typography from "@mui/material/Typography";
import type { JSX } from "react";

import type { CatalogEntry } from "@/shared/api/catalog";

interface OperationLabelProps {
  readonly entry: CatalogEntry;
}

/**
 * States an operation's name, rather than offering to change it -
 * `AddPanelForm`'s edit mode, over `OperationPicker`'s own `Autocomplete`
 * (P13: a panel's operation is fixed once it is made). Plain text, not a
 * disabled control: a disabled `Autocomplete` still renders an `input`
 * `make guard-layout` measures for size and edge contrast the same as an
 * enabled one, and a disabled MUI input's lighter border is exactly the
 * low-contrast edge that guard exists to catch. Text carries no such risk,
 * and reads more plainly besides - there is nothing here to interact with,
 * so nothing here should look interactive.
 */
export function OperationLabel({ entry }: OperationLabelProps): JSX.Element {
  return (
    <Typography variant="body2" color="textSecondary">
      操作: {entry.serviceDisplayName} / {entry.displayName || entry.operationId}
      （変更できません）
    </Typography>
  );
}
