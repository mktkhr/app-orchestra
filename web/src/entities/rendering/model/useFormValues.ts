import { useState } from "react";

import { fieldEntries, type FieldEntry } from "./fieldEntries";
import { fieldSchema, type Fields } from "./rows";

export type FormValues = Record<string, unknown>;

function isRecordValue(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function asRecord(value: unknown): Record<string, unknown> | undefined {
  return isRecordValue(value) ? value : undefined;
}

function asStringArray(value: unknown): readonly string[] {
  return Array.isArray(value)
    ? value.filter((item): item is string => typeof item === "string")
    : [];
}

function seedValue(fieldType: string | undefined, initialValue: unknown): unknown {
  if (initialValue !== undefined) {
    return initialValue;
  }

  if (fieldType === "boolean") {
    return false;
  }

  if (fieldType === "integer" || fieldType === "number") {
    return 0;
  }

  return "";
}

export interface FormValuesState {
  /** `schema.properties`, as a `Fields` map - what `ResultFormFields` needs to choose each control. */
  readonly properties: Fields;
  /** `schema.required` - which of `properties`' keys must be filled. */
  readonly required: readonly string[];
  /** `properties`' keys, each paired with its display label (`fieldEntries`). */
  readonly entries: readonly FieldEntry[];
  /** The form's current values, one per `entries` key, seeded from `initial` (or a type-appropriate default). */
  readonly values: FormValues;
  readonly setValue: (key: string, value: unknown) => void;
}

/**
 * A JSON-Schema `schema`'s properties, turned into editable form state:
 * which controls to draw (`entries`/`properties`/`required`) and their
 * current values (`values`/`setValue`), seeded once from `initial`.
 *
 * Pulled out of `ResultForm` (`docs/plans/dashboard.md` Task 6, P7) so a
 * second caller can draw the identical controls without a second form:
 * `features/panels`' builder reuses this together with `ResultFormFields`
 * for a catalogue entry's own `schema`, the same shape `ResultForm` already
 * builds a `kind: form` answer's arguments from. `ResultForm` itself keeps
 * using this exactly as before - only the seeding/state logic moved, not
 * its behaviour.
 */
export function useFormValues(
  schema: Record<string, unknown>,
  initial?: Record<string, unknown>,
): FormValuesState {
  const properties: Fields = asRecord(schema["properties"]) ?? {};
  const required = asStringArray(schema["required"]);
  const entries = fieldEntries(properties, Object.keys(properties));

  const [values, setValues] = useState<FormValues>(() => {
    const seed: FormValues = {};

    for (const { key } of entries) {
      const type = fieldSchema(properties, key)?.["type"];

      seed[key] = seedValue(typeof type === "string" ? type : undefined, initial?.[key]);
    }

    return seed;
  });

  const setValue = (key: string, value: unknown): void => {
    setValues((current) => ({ ...current, [key]: value }));
  };

  return { properties, required, entries, values, setValue };
}
