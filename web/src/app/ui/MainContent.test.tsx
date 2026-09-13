import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { afterEach, describe, expect, it, vi } from "vite-plus/test";

import { MainContent } from "./MainContent";

vi.mock("@/pages/chat", () => ({
  ChatPage: () => <p>chat screen</p>,
}));

vi.mock("@/pages/users", () => ({
  UsersPage: () => <p>users screen</p>,
}));

vi.mock("@/pages/workspace", () => ({
  WorkspacePage: ({ workspaceId }: { workspaceId: string }) => (
    <p>workspace screen: {workspaceId}</p>
  ),
}));

function renderAt(path: string): void {
  render(
    <MemoryRouter initialEntries={[path]}>
      <MainContent />
    </MemoryRouter>,
  );
}

describe("MainContent", () => {
  afterEach(() => {
    window.location.hash = "";
  });

  it("renders the chat at /", () => {
    renderAt("/");

    expect(screen.getByText("chat screen")).toBeTruthy();
  });

  it("renders a workspace at /workspaces/{id}, keyed by that id", () => {
    renderAt("/workspaces/ws-1");

    expect(screen.getByText("workspace screen: ws-1")).toBeTruthy();
  });

  it("renders the admin's users screen at /users", () => {
    renderAt("/users");

    expect(screen.getByText("users screen")).toBeTruthy();
  });

  it("renders the chat for an address matching nothing", () => {
    renderAt("/nope-such-screen");

    expect(screen.getByText("chat screen")).toBeTruthy();
  });

  it("ignores a hash entirely - the route is the path now", () => {
    window.location.hash = "#workspace-abc";

    renderAt("/");

    expect(screen.getByText("chat screen")).toBeTruthy();
  });
});
