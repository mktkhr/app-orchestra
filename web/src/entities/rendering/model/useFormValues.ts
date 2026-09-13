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
    // Not 0. A number field with nothing typed in it is empty, and zero is
    // a number somebody meant - a quantity of none is a different claim
    // from a quantity nobody has given yet. Seeding 0 also made the field
    // impossible to clear: `Number("")` is 0, so deleting the last digit
    // wrote 0 straight back (see ResultFormField).
    return undefined;
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
  /** Removes a key entirely - what an emptied field means (see the implementation). */
  readonly clearValue: (key: string) => void;
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

  // Emptying a field removes the key rather than setting it to undefined
  // or null: `values` is posted whole as a call's arguments
  // (`ResultForm`), and a key that is absent is the one shape that says
  // "nobody gave this". A null would be a value the service has to have an
  // opinion about, and an undefined would survive as a key with nothing in
  // it right up to JSON.stringify, which drops it - silently agreeing with
  // this, one layer later and by accident.
  const clearValue = (key: string): void => {
    setValues((current) => {
      const { [key]: _removed, ...rest } = current;

      return rest;
    });
  };

  return { properties, required, entries, values, setValue, clearValue };
}
