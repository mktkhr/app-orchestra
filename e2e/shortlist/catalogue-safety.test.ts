import { expect, test } from "vite-plus/test";

import { safeOperationIds } from "./catalogue-safety.ts";

/**
 * catalogue-safety.ts's own test: the fixture is static data (no server,
 * no model), so this reads it directly rather than faking it - the same
 * way e2e/narrowing/fixture/*.test.ts already do.
 */

const ids = safeOperationIds();

test.each([
  { operationId: "listInventoryBarcodes", want: true },
  { operationId: "getInventoryBarcode", want: true },
  { operationId: "searchInventoryPickLists", want: true },
])("$operationId (a GET operation) is safe", ({ operationId, want }) => {
  expect(ids.has(operationId)).toBe(want);
});

test.each([
  { operationId: "createInventoryBarcode", want: false },
  { operationId: "updateInventoryBarcode", want: false },
  { operationId: "deleteInventoryBarcode", want: false },
])("$operationId (a write operation) is not safe", ({ operationId, want }) => {
  expect(ids.has(operationId)).toBe(want);
});
