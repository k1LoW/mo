import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Sidebar } from "./Sidebar";
import type { Group } from "../hooks/useApi";

const groups: Group[] = [
  {
    name: "default",
    files: [
      { id: 1, name: "README.md", path: "/README.md" },
      { id: 2, name: "GUIDE.md", path: "/GUIDE.md" },
    ],
  },
  {
    name: "docs",
    files: [{ id: 3, name: "api.md", path: "/docs/api.md" }],
  },
];

beforeEach(() => {
  localStorage.clear();
});

describe("Sidebar", () => {
  it("renders files for the active group", () => {
    render(
      <Sidebar
        groups={groups}
        activeGroup="default"
        activeFileId={null}
        onFileSelect={() => {}}
      />,
    );
    expect(screen.getByText("README.md")).toBeInTheDocument();
    expect(screen.getByText("GUIDE.md")).toBeInTheDocument();
    expect(screen.queryByText("api.md")).not.toBeInTheDocument();
  });

  it("renders files for a non-default group", () => {
    render(
      <Sidebar
        groups={groups}
        activeGroup="docs"
        activeFileId={null}
        onFileSelect={() => {}}
      />,
    );
    expect(screen.getByText("api.md")).toBeInTheDocument();
    expect(screen.queryByText("README.md")).not.toBeInTheDocument();
  });

  it("highlights the active file", () => {
    render(
      <Sidebar
        groups={groups}
        activeGroup="default"
        activeFileId={1}
        onFileSelect={() => {}}
      />,
    );
    const activeButton = screen.getByText("README.md").closest("button")!;
    expect(activeButton.className).toContain("bg-gh-bg-active");

    const inactiveButton = screen.getByText("GUIDE.md").closest("button")!;
    expect(inactiveButton.className).toContain("bg-transparent");
  });

  it("calls onFileSelect when a file is clicked", async () => {
    const user = userEvent.setup();
    const onFileSelect = vi.fn();
    render(
      <Sidebar
        groups={groups}
        activeGroup="default"
        activeFileId={null}
        onFileSelect={onFileSelect}
      />,
    );

    await user.click(screen.getByText("GUIDE.md"));
    expect(onFileSelect).toHaveBeenCalledWith(2);
  });

  it("shows file path as title attribute", () => {
    render(
      <Sidebar
        groups={groups}
        activeGroup="default"
        activeFileId={null}
        onFileSelect={() => {}}
      />,
    );
    expect(screen.getByTitle("/README.md")).toBeInTheDocument();
    expect(screen.getByTitle("/GUIDE.md")).toBeInTheDocument();
  });

  it("renders empty when group has no files", () => {
    const emptyGroups: Group[] = [{ name: "empty", files: [] }];
    render(
      <Sidebar
        groups={emptyGroups}
        activeGroup="empty"
        activeFileId={null}
        onFileSelect={() => {}}
      />,
    );
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});
