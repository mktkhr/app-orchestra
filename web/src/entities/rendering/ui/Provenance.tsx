import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import Accordion from "@mui/material/Accordion";
import AccordionDetails from "@mui/material/AccordionDetails";
import AccordionSummary from "@mui/material/AccordionSummary";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import { useState, type JSX, type SyntheticEvent } from "react";

import type { PlanResult } from "@/shared/api/client";

import { formatCellValue } from "../model/rows";

type Source = NonNullable<PlanResult["source"]>;

interface ProvenanceProps {
  readonly source: Source;
}

/**
 * A result's origin: which service and operation produced it (AC-F-106).
 * When the planner supplied arguments, they are hidden behind an expander
 * rather than shown outright — a filtered list and its unfiltered sibling
 * should not look different at a glance.
 *
 * The expander is a controlled `Accordion` that renders `AccordionDetails`
 * only while open, rather than leaving MUI to hide it with CSS: the
 * arguments should not exist in the page until asked for.
 */
export function Provenance({ source }: ProvenanceProps): JSX.Element {
  const [open, setOpen] = useState(false);
  const args = Object.entries(source.args ?? {});

  const handleChange = (_event: SyntheticEvent, expanded: boolean): void => {
    setOpen(expanded);
  };

  return (
    <Stack spacing={0.5} sx={{ mb: 1 }}>
      <Typography variant="caption" color="text.secondary">
        {`${source.service} / ${source.operationId}`}
      </Typography>
      {args.length === 0 ? null : (
        <Accordion
          disableGutters
          elevation={0}
          square
          expanded={open}
          onChange={handleChange}
          sx={{ bgcolor: "transparent", "&:before": { display: "none" } }}
        >
          <AccordionSummary
            expandIcon={<ExpandMoreIcon fontSize="small" />}
            sx={{ minHeight: 0, px: 0 }}
          >
            <Typography variant="caption" color="text.secondary">
              引数を表示
            </Typography>
          </AccordionSummary>
          {open ? (
            <AccordionDetails sx={{ px: 0, pt: 0 }}>
              <Stack spacing={0.25}>
                {args.map(([key, value]) => (
                  <Typography key={key} variant="body2">
                    {`${key}=${formatCellValue(value)}`}
                  </Typography>
                ))}
              </Stack>
            </AccordionDetails>
          ) : null}
        </Accordion>
      )}
    </Stack>
  );
}
