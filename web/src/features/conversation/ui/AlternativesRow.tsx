import Chip from "@mui/material/Chip";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import type { JSX } from "react";

import type { PlanResult } from "@/shared/api/client";

interface Alternative {
  readonly operationId: string;
  readonly displayName: string;
  readonly service: string;
}

interface AlternativesRowProps {
  readonly alternatives: PlanResult["alternatives"];
  readonly onSelect: (operationId: string, displayName: string) => void;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/**
 * `PlanResult.alternatives`, read defensively rather than iterated
 * directly. openapi-fetch's `MethodResponse` mapping turns a nested
 * array-of-objects schema property into a type whose own prototype methods
 * (`map`, `filter`, ...) type-check as `{}` - calling `.map` on it directly
 * is a compile error, not just an unsafe read (`ResultChoice`'s own
 * `toEnumOptions` carries the same comment for `PlanResult.options`).
 * `Array.isArray` narrows to a real `any[]`, which is what restores
 * working array methods; the shape check afterwards drops anything that
 * does not carry all three fields as strings.
 */
function toAlternatives(alternatives: PlanResult["alternatives"]): readonly Alternative[] {
  if (!Array.isArray(alternatives)) {
    return [];
  }

  return alternatives.flatMap((alternative: unknown) => {
    if (!isRecord(alternative)) {
      return [];
    }

    const { operationId, displayName, service } = alternative;

    return typeof operationId === "string" &&
      typeof displayName === "string" &&
      typeof service === "string"
      ? [{ operationId, displayName, service }]
      : [];
  });
}

/**
 * A `result`'s further shortlist candidates, offered under it as
 * 「違いましたか？」 and one MUI `Chip` per alternative
 * (docs/specs/shortlisting.md, section 4, H5). Absent or empty
 * `alternatives` - narrowing off, or nothing left in the shortlist - draws
 * nothing at all, not an empty row: `TurnList` renders this unconditionally
 * for every `kind: "result"` answer, so this is the one place that decides
 * whether there is anything to show.
 *
 * `Chip`'s own default size, not `small`: `make guard-browser` measures
 * touch target size on the built pages, and `small` falls under it.
 * `title` carries the service, the same way a native `title` attribute
 * always has - `Provenance` has no comparable secondary-text pattern for a
 * chip to follow.
 */
export function AlternativesRow({
  alternatives,
  onSelect,
}: AlternativesRowProps): JSX.Element | null {
  const parsed = toAlternatives(alternatives);

  if (parsed.length === 0) {
    return null;
  }

  return (
    <Stack direction="row" spacing={1} sx={{ alignItems: "center", flexWrap: "wrap", rowGap: 1 }}>
      <Typography variant="body2">違いましたか？</Typography>
      {parsed.map((alternative) => (
        <Chip
          key={alternative.operationId}
          label={alternative.displayName}
          title={alternative.service}
          clickable
          onClick={() => {
            onSelect(alternative.operationId, alternative.displayName);
          }}
        />
      ))}
    </Stack>
  );
}
