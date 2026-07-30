import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Sidebar } from "./Sidebar";
import type { Group, SearchResult } from "../hooks/useApi";

const groups: Group[] = [
  {
    name: "default",
    files: [
      { id: "aaa11111", name: "README.md", path: "/README.md", title: "Getting Started" },
      { id: "bbb22222", name: "GUIDE.md", path: "/GUIDE.md" },
    ],
  },
  {
    name: "docs",
    files: [{ id: "ccc33333", name: "api.md", path: "/docs/api.md" }],
  },
];

const searchResults: SearchResult[] = [
  {
    fileId: "aaa11111",
    fileName: "README.md",
    title: "Getting Started",
    path: "/README.md",
    uploaded: false,
    matches: [
      {
        line: 3,
        text: "cache line",
        before: ["# Intro"],
        after: ["after line"],
        heading: "Intro",
        anchor: { kind: "heading", value: "Intro" },
      },
    ],
  },
];

function hasTextContent(text: string) {
  return (_content: string, element: Element | null) => element?.textContent === text;
}

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
        showTitle={false}
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
        showTitle={false}
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
        showTitle={false}
        searchQuery={null}
        onSearchQueryChange={() => {}}
      />,
    );
    const activeLink = screen.getByText("README.md").closest("a")!;
    expect(activeLink.className).toContain("bg-gh-bg-active");
    expect(activeLink.getAttribute("aria-current")).toBe("page");

    const inactiveLink = screen.getByText("GUIDE.md").closest("a")!;
    expect(inactiveLink.className).toContain("bg-transparent");
    expect(inactiveLink.getAttribute("aria-current")).toBeNull();
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
        showTitle={false}
        searchQuery={null}
        onSearchQueryChange={() => {}}
      />,
    );

    await user.click(screen.getByText("GUIDE.md"));
    expect(onFileSelect).toHaveBeenCalledWith("bbb22222", "focused");
  });

  it("opens a file in a new pane on Alt+click", async () => {
    const user = userEvent.setup();
    const onFileSelect = vi.fn();
    render(
      <Sidebar
        groups={groups}
        activeGroup="default"
        activeFileId="aaa11111"
        onFileSelect={onFileSelect}
        onFilesReorder={() => {}}
        viewMode="flat"
        showTitle={false}
        searchQuery={null}
        onSearchQueryChange={() => {}}
      />,
    );

    await user.keyboard("[AltLeft>]");
    await user.click(screen.getByText("GUIDE.md"));
    await user.keyboard("[/AltLeft]");

    expect(onFileSelect).toHaveBeenCalledWith("bbb22222", "new-pane");
  });

  it("highlights every open file but marks only the focused one", () => {
    render(
      <Sidebar
        groups={groups}
        activeGroup="default"
        activeFileId="bbb22222"
        openFileIds={new Set(["aaa11111", "bbb22222"])}
        onFileSelect={() => {}}
        onFilesReorder={() => {}}
        viewMode="flat"
        showTitle={false}
        searchQuery={null}
        onSearchQueryChange={() => {}}
      />,
    );

    const readme = screen.getByText("README.md").closest("a")!;
    const guide = screen.getByText("GUIDE.md").closest("a")!;

    // Both are visible in a pane, so both get the open treatment.
    expect(readme.className).toContain("bg-gh-bg-active");
    expect(guide.className).toContain("bg-gh-bg-active");

    // Only the focused pane's file carries the accent bar and aria-current.
    expect(readme.className).toContain("border-l-transparent");
    expect(guide.className).toContain("border-l-gh-accent");
    expect(readme.getAttribute("aria-current")).toBeNull();
    expect(guide.getAttribute("aria-current")).toBe("page");
  });

  it("renders file items as anchors with href to file URL", () => {
    render(
      <Sidebar
        groups={groups}
        activeGroup="default"
        activeFileId={null}
        onFileSelect={() => {}}
        onFilesReorder={() => {}}
        viewMode="flat"
        showTitle={false}
        searchQuery={null}
        onSearchQueryChange={() => {}}
      />,
    );
    expect(screen.getByText("README.md").closest("a")?.getAttribute("href")).toBe(
      "/?file=aaa11111",
    );
    expect(screen.getByText("GUIDE.md").closest("a")?.getAttribute("href")).toBe("/?file=bbb22222");
  });

  it("does not call onFileSelect when modifier keys are pressed", async () => {
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
        showTitle={false}
        searchQuery={null}
        onSearchQueryChange={() => {}}
      />,
    );

    await user.keyboard("[ControlLeft>]");
    await user.click(screen.getByText("GUIDE.md"));
    await user.keyboard("[/ControlLeft]");
    expect(onFileSelect).not.toHaveBeenCalled();
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
        showTitle={false}
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
        showTitle={false}
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
        showTitle={false}
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
        showTitle={false}
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
        showTitle={false}
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
        showTitle={false}
        searchQuery=""
        onSearchQueryChange={onSearchQueryChange}
      />,
    );
    const input = screen.getByPlaceholderText("Search files...");
    await user.click(input);
    await user.keyboard("{Escape}");
    expect(onSearchQueryChange).toHaveBeenCalledWith(null);
  });

  it("shows heading titles when showTitle is true", () => {
    render(
      <Sidebar
        groups={groups}
        activeGroup="default"
        activeFileId={null}
        onFileSelect={() => {}}
        onFilesReorder={() => {}}
        viewMode="flat"
        showTitle={true}
        searchQuery={null}
        onSearchQueryChange={() => {}}
      />,
    );
    expect(screen.getByText("Getting Started")).toBeInTheDocument();
    expect(screen.queryByText("README.md")).not.toBeInTheDocument();
    expect(screen.getByText("GUIDE.md")).toBeInTheDocument();
  });

  it("shows file names when showTitle is false even if title exists", () => {
    render(
      <Sidebar
        groups={groups}
        activeGroup="default"
        activeFileId={null}
        onFileSelect={() => {}}
        onFilesReorder={() => {}}
        viewMode="flat"
        showTitle={false}
        searchQuery={null}
        onSearchQueryChange={() => {}}
      />,
    );
    expect(screen.getByText("README.md")).toBeInTheDocument();
    expect(screen.queryByText("Getting Started")).not.toBeInTheDocument();
  });

  it("search matches against title", () => {
    render(
      <Sidebar
        groups={groups}
        activeGroup="default"
        activeFileId={null}
        onFileSelect={() => {}}
        onFilesReorder={() => {}}
        viewMode="flat"
        showTitle={false}
        searchQuery="getting"
        onSearchQueryChange={() => {}}
      />,
    );
    expect(screen.getByText("README.md")).toBeInTheDocument();
    expect(screen.queryByText("GUIDE.md")).not.toBeInTheDocument();
  });

  it("renders content search results", () => {
    render(
      <Sidebar
        groups={groups}
        activeGroup="default"
        activeFileId={null}
        onFileSelect={() => {}}
        onFilesReorder={() => {}}
        viewMode="flat"
        showTitle={false}
        searchQuery="cache"
        onSearchQueryChange={() => {}}
        searchResults={searchResults}
      />,
    );
    expect(screen.getByText("Content matches")).toBeInTheDocument();
    expect(screen.getByText("Line 3")).toBeInTheDocument();
    expect(screen.getByText(hasTextContent("cache line"))).toBeInTheDocument();
  });

  it("toggles content matches section", async () => {
    const user = userEvent.setup();
    render(
      <Sidebar
        groups={groups}
        activeGroup="default"
        activeFileId={null}
        onFileSelect={() => {}}
        onFilesReorder={() => {}}
        viewMode="flat"
        showTitle={false}
        searchQuery="cache"
        onSearchQueryChange={() => {}}
        searchResults={searchResults}
      />,
    );

    await user.click(screen.getByRole("button", { name: /content matches/i }));
    expect(screen.queryByText(hasTextContent("cache line"))).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /content matches/i }));
    expect(screen.getByText(hasTextContent("cache line"))).toBeInTheDocument();
  });
});
