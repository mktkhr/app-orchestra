import { describe, expect, it } from "vite-plus/test";

import { checkImport, locate, resolveSpecifier } from "./fsd.ts";

const layers = [
  { name: "shared", sliced: false },
  { name: "entities", sliced: true },
  { name: "features", sliced: true },
  { name: "widgets", sliced: true },
  { name: "pages", sliced: true },
  { name: "app", sliced: false },
] as const;

describe("locate", () => {
  it("finds the layer and slice of a module", () => {
    expect(locate("features/greet/ui/Form.tsx", layers)).toEqual({
      layer: "features",
      rank: 2,
      slice: "greet",
    });
    expect(locate("shared/api/http.ts", layers)).toEqual({ layer: "shared", rank: 0, slice: null });
  });

  it("returns null outside the declared layers", () => {
    expect(locate("lib/thing.ts", layers)).toBeNull();
  });
});

describe("resolveSpecifier", () => {
  it("maps the alias and relative paths onto the source root", () => {
    expect(resolveSpecifier("features/greet/ui/Form.tsx", "@/shared/ui")).toBe("shared/ui");
    expect(resolveSpecifier("features/greet/ui/Form.tsx", "../model/use")).toBe(
      "features/greet/model/use",
    );
  });

  it("ignores packages", () => {
    expect(resolveSpecifier("app/main.tsx", "react")).toBeNull();
  });
});

describe("checkImport", () => {
  it("allows downward imports through the public API", () => {
    expect(checkImport("pages/home/ui/Home.tsx", "@/features/greet", layers)).toBeNull();
    expect(checkImport("features/greet/ui/Form.tsx", "@/shared/ui", layers)).toBeNull();
    expect(checkImport("features/greet/ui/Form.tsx", "../model/use", layers)).toBeNull();
  });

  it("rejects upward imports", () => {
    expect(checkImport("shared/api/http.ts", "@/features/greet", layers)?.reason).toMatch(
      /downwards/u,
    );
    expect(checkImport("entities/greeting/model/g.ts", "@/app", layers)?.reason).toMatch(
      /downwards/u,
    );
  });

  it("rejects sibling slices", () => {
    expect(checkImport("features/greet/ui/Form.tsx", "@/features/other", layers)?.reason).toMatch(
      /sibling/u,
    );
  });

  it("rejects deep imports into another slice", () => {
    expect(
      checkImport("pages/home/ui/Home.tsx", "@/features/greet/ui/Form", layers)?.reason,
    ).toMatch(/public API/u);
  });

  it("rejects relative imports that escape the slice", () => {
    expect(checkImport("features/greet/ui/Form.tsx", "../../other/ui/X", layers)?.reason).toMatch(
      /sibling/u,
    );
  });

  it("rejects modules outside the declared layers", () => {
    expect(checkImport("features/greet/ui/Form.tsx", "@/lib/x", layers)?.reason).toMatch(
      /declared layer/u,
    );
  });
});
