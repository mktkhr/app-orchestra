import Stack from "@mui/material/Stack";
import type { JSX } from "react";

import { operationKey, type PanelFormState } from "../model/usePanelBuilder";
import { ChartFields } from "./ChartFields";
import { ComponentPicker } from "./ComponentPicker";
import { OperationLabel } from "./OperationLabel";
import { OperationPicker } from "./OperationPicker";
import { PanelArguments } from "./PanelArguments";
import { PanelSaveFields } from "./PanelSaveFields";
import { TransformFields } from "./TransformFields";

interface AddPanelFormProps {
  readonly builder: PanelFormState;
  /**
   * True over an existing panel's edit form: the operation is fixed there
   * (P13), so this draws its name as plain text instead of
   * `OperationPicker`'s `Autocomplete` - stated, not offered, and not
   * merely disabled. A disabled `Autocomplete` still renders an `input`
   * `make guard-layout` measures for size and edge contrast the same as an
   * enabled one, and a disabled MUI input's lighter border is exactly the
   * kind of low-contrast edge that guard exists to catch; stating the name
   * as `Typography` instead removes the control from that selector
   * altogether rather than betting it clears the threshold.
   */
  readonly operationLocked?: boolean;
}

/**
 * Steps 1-5 (`docs/specs/dashboard.md` section 6), one form and not a
 * wizard: every control that applies to the current choice is on screen at
 * once, and a control that does not apply is absent rather than shown
 * disabled - a person who picked `table` sees no chart axes at all, and the
 * transform's own fields exist only once its switch is on.
 *
 * Reused for editing a panel (P12, section 6a), over the same `builder`
 * shape a create and an edit both produce (`PanelFormState`) - a second
 * form would be a second place for the chart's axes and the transform's
 * fields to get out of step with the first.
 */
export function AddPanelForm({ builder, operationLocked = false }: AddPanelFormProps): JSX.Element {
  return (
    <Stack spacing={2} sx={{ mt: 1 }}>
      {operationLocked && builder.entry !== null ? (
        <OperationLabel entry={builder.entry} />
      ) : (
        <OperationPicker
          entries={builder.catalog.entries}
          value={builder.entry}
          onChange={builder.selectEntry}
          loadError={builder.catalog.loadError}
        />
      )}

      {builder.entry === null ? null : (
        <>
          <PanelArguments
            key={operationKey(builder.entry)}
            schema={builder.entry.schema}
            onChange={builder.setArgsValues}
            initialValues={builder.argsValues}
          />

          <ComponentPicker
            options={builder.componentOptions}
            value={builder.component}
            onChange={builder.setComponent}
          />

          {builder.component === "chart" && (
            <ChartFields
              fieldOptions={builder.chartFieldOptions}
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
            saveLabel={operationLocked ? "保存" : "追加"}
          />
        </>
      )}
    </Stack>
  );
}
