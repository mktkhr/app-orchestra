import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vite-plus/test";

import { TextBubble } from "./TextBubble";

describe("TextBubble", () => {
  it("shows the overline and its children, with no control of its own", () => {
    render(<TextBubble overline="質問">新しい名前は何ですか？</TextBubble>);

    expect(screen.getByText("質問")).toBeTruthy();
    expect(screen.getByText("新しい名前は何ですか？")).toBeTruthy();
    expect(screen.queryByRole("button")).toBeNull();
  });
});
