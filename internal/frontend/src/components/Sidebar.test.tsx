import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Sidebar } from "./Sidebar";
import type { Group } from "../hooks/useApi";

const groups: Group[] = [
  {
    name: "default",
    files: [
      { id: "aaa11111", name: "README.md", path: "/README.md" },
      { id: "bbb22222", name: "GUIDE.md", path: "/GUIDE.md" },
    ],
  },
  {
    name: "docs",
    files: [{ id: "ccc33333", name: "api.md", path: "/docs/api.md" }],
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
        onFilesReorder={() => {}}
        viewMode="flat"
        searchQuery={null}
        onSearchQueryChange={() => {}}
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
        onFilesReorder={() => {}}
        viewMode="flat"
        searchQuery={null}
        onSearchQueryChange={() => {}}
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
        activeFileId={"aaa11111"}
        onFileSelect={() => {}}
        onFilesReorder={() => {}}
        viewMode="flat"
        searchQuery={null}
        onSearchQueryChange={() => {}}
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
        onFilesReorder={() => {}}
        viewMode="flat"
        searchQuery={null}
        onSearchQueryChange={() => {}}
      />,
    );

    await user.click(screen.getByText("GUIDE.md"));
    expect(onFileSelect).toHaveBeenCalledWith("bbb22222");
  });

  it("shows file path as title attribute", () => {
    render(
      <Sidebar
        groups={groups}
        activeGroup="default"
        activeFileId={null}
        onFileSelect={() => {}}
        onFilesReorder={() => {}}
        viewMode="flat"
        searchQuery={null}
        onSearchQueryChange={() => {}}
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
        onFilesReorder={() => {}}
        viewMode="flat"
        searchQuery={null}
        onSearchQueryChange={() => {}}
      />,
    );
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("shows search input when searchQuery is non-null", () => {
    render(
      <Sidebar
        groups={groups}
        activeGroup="default"
        activeFileId={null}
        onFileSelect={() => {}}
        onFilesReorder={() => {}}
        viewMode="flat"
        searchQuery=""
        onSearchQueryChange={() => {}}
      />,
    );
    expect(screen.getByPlaceholderText("Search files...")).toBeInTheDocument();
  });

  it("does not show search input when searchQuery is null", () => {
    render(
      <Sidebar
        groups={groups}
        activeGroup="default"
        activeFileId={null}
        onFileSelect={() => {}}
        onFilesReorder={() => {}}
        viewMode="flat"
        searchQuery={null}
        onSearchQueryChange={() => {}}
      />,
    );
    expect(screen.queryByPlaceholderText("Search files...")).not.toBeInTheDocument();
  });

  it("filters files by search query", () => {
    render(
      <Sidebar
        groups={groups}
        activeGroup="default"
        activeFileId={null}
        onFileSelect={() => {}}
        onFilesReorder={() => {}}
        viewMode="flat"
        searchQuery="read"
        onSearchQueryChange={() => {}}
      />,
    );
    expect(screen.getByText("README.md")).toBeInTheDocument();
    expect(screen.queryByText("GUIDE.md")).not.toBeInTheDocument();
  });

  it("calls onSearchQueryChange with null on Escape key", async () => {
    const user = userEvent.setup();
    const onSearchQueryChange = vi.fn();
    render(
      <Sidebar
        groups={groups}
        activeGroup="default"
        activeFileId={null}
        onFileSelect={() => {}}
        onFilesReorder={() => {}}
        viewMode="flat"
        searchQuery=""
        onSearchQueryChange={onSearchQueryChange}
      />,
    );
    const input = screen.getByPlaceholderText("Search files...");
    await user.click(input);
    await user.keyboard("{Escape}");
    expect(onSearchQueryChange).toHaveBeenCalledWith(null);
  });
});
