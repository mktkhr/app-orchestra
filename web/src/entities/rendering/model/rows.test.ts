import { describe, expect, it } from "vite-plus/test";

import { cellText, columnTitle, type Fields, formatCellValue, rowsFromData } from "./rows";

describe("rowsFromData", () => {
  it("takes the sole array-valued property (inventory's items envelope)", () => {
    const data = { items: [{ id: "itm-001" }, { id: "itm-002" }] };

    expect(rowsFromData(data)).toEqual([{ id: "itm-001" }, { id: "itm-002" }]);
  });

  it("takes the sole array-valued property under a different name (attendance's records envelope)", () => {
    const data = { records: [{ id: "att-001" }] };

    expect(rowsFromData(data)).toEqual([{ id: "att-001" }]);
  });

  it("returns no rows when there is no array-valued property", () => {
    expect(rowsFromData({ total: 0 })).toEqual([]);
  });

  it("returns no rows when more than one property is an array", () => {
    const data = { items: [{ id: "a" }], tags: ["x"] };

    expect(rowsFromData(data)).toEqual([]);
  });

  it("drops array entries that are not themselves objects", () => {
    const data = { items: [{ id: "itm-001" }, "not-a-row", 42] };

    expect(rowsFromData(data)).toEqual([{ id: "itm-001" }]);
  });
});

describe("formatCellValue", () => {
  it("stringifies primitives", () => {
    expect(formatCellValue("allocated")).toBe("allocated");
    expect(formatCellValue(120)).toBe("120");
    expect(formatCellValue(true)).toBe("true");
  });

  it("renders null and undefined as an empty string", () => {
    const missing: { readonly value?: string } = {};

    expect(formatCellValue(null)).toBe("");
    expect(formatCellValue(missing.value)).toBe("");
  });

  it("renders objects and arrays as JSON", () => {
    expect(formatCellValue({ a: 1 })).toBe('{"a":1}');
    expect(formatCellValue([1, 2])).toBe("[1,2]");
  });
});

const statusFields: Fields = {
  status: {
    type: "string",
    enum: ["allocated", "staged", "quarantined", "consigned"],
    enumLabels: {
      allocated: "引当済",
      staged: "出荷準備完了",
      quarantined: "検品保留",
      consigned: "預託在庫",
    },
  },
};

describe("cellText", () => {
  it("shows the Japanese label when fields declares one for the value", () => {
    expect(cellText(statusFields, "status", "quarantined")).toBe("検品保留");
  });

  it("falls back to the raw value when there is no enumLabels entry for it", () => {
    expect(cellText(statusFields, "status", "unknown-value")).toBe("unknown-value");
  });

  it("falls back to the raw value when the column has no field schema at all", () => {
    expect(cellText(statusFields, "name", "梱包用ダンボール")).toBe("梱包用ダンボール");
  });

  it("falls back to the raw value when fields is undefined", () => {
    expect(cellText(undefined, "status", "quarantined")).toBe("quarantined");
  });

  it("formats a non-string value as usual, ignoring enumLabels", () => {
    expect(cellText(statusFields, "quantity", 200)).toBe("200");
  });
});

describe("columnTitle", () => {
  it("uses fields[column].title when the schema declares one", () => {
    const fields: Fields = { status: { type: "string", title: "在庫状況" } };

    expect(columnTitle(fields, "status")).toBe("在庫状況");
  });

  it("falls back to the column key when the schema has no title", () => {
    expect(columnTitle(statusFields, "status")).toBe("status");
  });

  it("falls back to the column key when fields is undefined", () => {
    expect(columnTitle(undefined, "status")).toBe("status");
  });
});
