// Redocly custom rule: every enum must carry a Japanese label for each value.
//
// Design: an LLM resolves a natural-language query into enum values (see
// PRODUCT.md / docs/specs/orchestration.md). Business vocabulary such as
// `allocated` (引当済) or `substitute` (振替休日) cannot be guessed from the
// English token alone, so a schema's `enum` must carry an `x-enum-labels`
// map from each enum value to its Japanese label, e.g.:
//
//   status:
//     type: string
//     enum: [allocated, staged, quarantined]
//     x-enum-labels:
//       allocated: 引当済
//       staged: 出荷準備完了
//       quarantined: 検品保留
//
// This is a static check instead of a written convention: `x-enum-labels`
// must exist, and its key set must exactly match the `enum` value set (no
// missing labels, no stray labels for values that were removed or renamed).
//
// Part of the Repository Harness (harness/quality/README.md). Registered in
// redocly.yaml as `orchestra/enum-labels-required`.

const id = "orchestra";

const EnumLabelsRequired = () => {
  return {
    Schema(schema, { report, location }) {
      if (!schema.enum || !Array.isArray(schema.enum)) return;

      const enumValues = schema.enum.map(String);
      const labels = schema["x-enum-labels"];

      if (labels === undefined) {
        report({
          message:
            "enum requires x-enum-labels: a Japanese label for every enum value (see harness/quality/redocly/enum-labels.js).",
          location: location.child(["enum"]),
        });
        return;
      }

      if (typeof labels !== "object" || labels === null || Array.isArray(labels)) {
        report({
          message: "x-enum-labels must be a mapping from each enum value to its Japanese label.",
          location: location.child(["x-enum-labels"]),
        });
        return;
      }

      const labelKeys = Object.keys(labels);
      const enumSet = new Set(enumValues);
      const labelSet = new Set(labelKeys);

      for (const value of enumValues) {
        if (!labelSet.has(value)) {
          report({
            message: `x-enum-labels is missing a label for enum value "${value}".`,
            location: location.child(["x-enum-labels"]),
          });
        }
      }

      for (const key of labelKeys) {
        if (!enumSet.has(key)) {
          report({
            message: `x-enum-labels has a label for "${key}", which is not one of this schema's enum values.`,
            location: location.child(["x-enum-labels", key]),
          });
        }
      }
    },
  };
};

export default function orchestraPlugin() {
  return {
    id,
    rules: {
      oas3: {
        "enum-labels-required": EnumLabelsRequired,
      },
    },
  };
}
