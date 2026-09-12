import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import CardHeader from "@mui/material/CardHeader";
import Checkbox from "@mui/material/Checkbox";
import FormControlLabel from "@mui/material/FormControlLabel";
import FormGroup from "@mui/material/FormGroup";
import type { JSX } from "react";

import { keyOf, type PermissionGroup } from "../model/usePermissionGrid";

interface ServiceCardProps {
  readonly group: PermissionGroup;
  readonly granted: ReadonlyMap<string, unknown>;
  readonly onToggleOperation: (service: string, operationId: string) => void;
  readonly onToggleService: (service: string) => void;
}

/**
 * One service's card in the permission grid (docs/plans/auth.md Task 5): a
 * header checkbox that grants or revokes the whole service in a single
 * gesture (docs/specs/auth.md section 4), and one checkbox per operation it
 * holds. Split out of `PermissionGrid` to keep that file's own import count
 * under `import/max-dependencies`.
 */
export function ServiceCard({
  group,
  granted,
  onToggleOperation,
  onToggleService,
}: ServiceCardProps): JSX.Element {
  const grantedCount = group.operations.filter((operation) =>
    granted.has(keyOf({ service: group.service, operationId: operation.operationId })),
  ).length;
  const allGranted = grantedCount === group.operations.length && group.operations.length > 0;
  const someGranted = grantedCount > 0 && !allGranted;

  return (
    <Card variant="outlined">
      <CardHeader
        title={group.service}
        avatar={
          <Checkbox
            checked={allGranted}
            indeterminate={someGranted}
            onChange={() => {
              onToggleService(group.service);
            }}
            // p: "12px" rather than the default 9px padding - a medium
            // Checkbox's 24px icon plus the default padding is 42px, under
            // the 44px floor make guard-layout measures on the underlying
            // <input>.
            sx={{ p: "12px" }}
            slotProps={{ input: { "aria-label": `${group.service}をすべて許可` } }}
          />
        }
      />
      <CardContent>
        <FormGroup>
          {group.operations.map((operation) => (
            <FormControlLabel
              key={operation.operationId}
              control={
                <Checkbox
                  checked={granted.has(
                    keyOf({ service: group.service, operationId: operation.operationId }),
                  )}
                  onChange={() => {
                    onToggleOperation(group.service, operation.operationId);
                  }}
                  sx={{ p: "12px" }}
                />
              }
              label={operation.summary ?? operation.operationId}
            />
          ))}
        </FormGroup>
      </CardContent>
    </Card>
  );
}
