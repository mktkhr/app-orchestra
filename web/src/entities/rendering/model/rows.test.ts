import { describe, expect, it } from "vite-plus/test";

import { formatCellValue, rowsFromData } from "./rows";

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
