import Chip from "@mui/material/Chip";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import type { JSX } from "react";

import type { AlternativesTurn } from "../model/turn";

interface AlternativesRowProps {
  readonly turn: AlternativesTurn;
  readonly onSelect: (operationId: string, displayName: string) => void;
}

/**
 * A `result`'s further shortlist candidates, shown as their own assistant
 * turn reading `turn.text` (「違いましたか？」) with one MUI `Chip` per
 * candidate (docs/specs/shortlisting.md, section 4, H5). `TurnList` renders
 * this only for a `role: "alternatives"` turn, and only when `turn.alternatives`
 * is non-empty - the store never appends an empty one - so unlike the
 * previous `kind: "result"`-attached version, this component draws
 * unconditionally.
 *
 * `turn.chosen` is the operation the person picked, set by the store the
 * instant a chip is clicked (docs/specs/shortlisting.md, section 4): while
 * it is undefined, every chip carries an `onClick` - which is what MUI
 * reads to draw it clickable, focusable and `role="button"` in the first
 * place - and posts on click. Once `chosen` is set, no chip carries an
 * `onClick` any more: the chosen chip renders `color="primary"` (still
 * `filled`, as every chip here is) so it reads as selected rather than
 * merely inert, and every other chip renders `disabled` besides. Neither
 * is reachable by a click any more, which is what makes a second click on
 * an already-answered turn impossible: there is no enabled chip left. The
 * user's own words asked for exactly this - the chosen chip shown clearly,
 * the other disabled - after two chips clicked one after another once
 * stacked two operations under a single question.
 *
 * `Chip`'s own default size, not `small`: `make guard-browser` measures
 * touch target size on the built pages, and `small` falls under it. `title`
 * carries the service, the same way a native `title` attribute always has -
 * `Provenance` has no comparable secondary-text pattern for a chip to
 * follow. `disabled` (not an opacity/pointer-events hack) is what
 * `make guard-layout`'s contrast check expects a locked chip to use.
 */
export function AlternativesRow({ turn, onSelect }: AlternativesRowProps): JSX.Element {
  return (
    <Stack spacing={1.5}>
      <Typography variant="body1">{turn.text}</Typography>
      <Stack direction="row" spacing={1} sx={{ flexWrap: "wrap", rowGap: 1 }}>
        {turn.alternatives.map((alternative) => {
          const isChosen = alternative.operationId === turn.chosen;

          return (
            <Chip
              key={alternative.operationId}
              label={alternative.displayName}
              title={alternative.service}
              color={isChosen ? "primary" : "default"}
              variant="filled"
              disabled={turn.chosen !== undefined && !isChosen}
              {...(turn.chosen === undefined
                ? {
                    onClick: () => {
                      onSelect(alternative.operationId, alternative.displayName);
                    },
                  }
                : {})}
            />
          );
        })}
      </Stack>
    </Stack>
  );
}
