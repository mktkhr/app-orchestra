import Stack from "@mui/material/Stack";
import type { JSX } from "react";

import { operationKey, type PanelBuilder } from "../model/usePanelBuilder";
import { ChartFields } from "./ChartFields";
import { ComponentPicker } from "./ComponentPicker";
import { OperationPicker } from "./OperationPicker";
import { PanelArguments } from "./PanelArguments";
import { PanelSaveFields } from "./PanelSaveFields";
import { TransformFields } from "./TransformFields";

interface AddPanelFormProps {
  readonly builder: PanelBuilder;
}

/**
 * Steps 1-5 (`docs/specs/dashboard.md` section 6), one form and not a
 * wizard: every control that applies to the current choice is on screen at
 * once, and a control that does not apply is absent rather than shown
 * disabled - a person who picked `table` sees no chart axes at all, and the
 * transform's own fields exist only once its switch is on.
 */
export function AddPanelForm({ builder }: AddPanelFormProps): JSX.Element {
  return (
    <Stack spacing={2} sx={{ mt: 1 }}>
      <OperationPicker
        entries={builder.catalog.entries}
        value={builder.entry}
        onChange={builder.selectEntry}
        loadError={builder.catalog.loadError}
      />

      {builder.entry === null ? null : (
        <>
          <PanelArguments
            key={operationKey(builder.entry)}
            schema={builder.entry.schema}
            onChange={builder.setArgsValues}
          />

          <ComponentPicker
            options={builder.componentOptions}
            value={builder.component}
            onChange={builder.setComponent}
          />

          {builder.component === "chart" && (
            <ChartFields
              fieldOptions={builder.fieldOptions}
              category={builder.category}
              value={builder.value}
              kind={builder.kind}
              onCategoryChange={builder.setCategory}
              onValueChange={builder.setValue}
              onKindChange={builder.setKind}
            />
          )}

          {builder.fieldOptions.length > 0 && (
            <TransformFields
              fieldOptions={builder.fieldOptions}
              enabled={builder.transformEnabled}
              groupBy={builder.groupBy}
              aggregate={builder.aggregate}
              aggregateField={builder.aggregateField}
              onEnabledChange={builder.setTransformEnabled}
              onGroupByChange={builder.setGroupBy}
              onAggregateChange={builder.setAggregate}
              onAggregateFieldChange={builder.setAggregateField}
            />
          )}

          <PanelSaveFields
            title={builder.title}
            onTitleChange={builder.setTitle}
            error={builder.error}
            submitting={builder.submitting}
            onSave={builder.handleSave}
          />
        </>
      )}
    </Stack>
  );
}
