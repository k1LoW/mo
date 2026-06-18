import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MarkdownViewer } from "./MarkdownViewer";

vi.mock("mermaid", () => ({
  default: {
    initialize: vi.fn(),
    render: vi.fn(),
  },
}));

vi.mock("../hooks/useApi", () => ({
  fetchFileContent: vi.fn().mockResolvedValue({ content: "# Hello", baseDir: "/repo" }),
  openRelativeFile: vi.fn(),
}));

// jsdom has no layout, so stub getBoundingClientRect: the scroll container and
// the sticky bar sit at the top, and the heading goes wherever a test wants it.
let scrollContainer: HTMLElement;
let headingRect: { top: number; bottom: number };
const barRect = { top: 0, bottom: 37 };
const originalGetBoundingClientRect = Element.prototype.getBoundingClientRect;

function rect(top: number, bottom: number): DOMRect {
  return {
    top,
    bottom,
    left: 0,
    right: 0,
    width: 0,
    height: bottom - top,
    x: 0,
    y: top,
  } as DOMRect;
}

function renderViewer(props: Partial<Parameters<typeof MarkdownViewer>[0]> = {}) {
  return render(
    <MarkdownViewer
      fileId="aaa11111"
      fileName="README.md"
      activeGroup="default"
      revision={0}
      scrollContainer={scrollContainer}
      onFileOpened={() => {}}
      onHeadingsChange={() => {}}
      isTocOpen={false}
      onTocToggle={() => {}}
      onRemoveFile={() => {}}
      isWide={false}
      fontSize="medium"
      {...props}
    />,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  scrollContainer = document.createElement("div");
  headingRect = { top: 100, bottom: 150 }; // on screen by default
  Element.prototype.getBoundingClientRect = function () {
    if (this === scrollContainer) return rect(0, 800);
    // The sticky bar is the only div carrying a title attribute.
    if (this.tagName === "DIV" && this.hasAttribute("title"))
      return rect(barRect.top, barRect.bottom);
    if (/^H[1-6]$/.test(this.tagName)) return rect(headingRect.top, headingRect.bottom);
    return rect(0, 0);
  };
});

afterEach(() => {
  Element.prototype.getBoundingClientRect = originalGetBoundingClientRect;
});

describe("MarkdownViewer file label", () => {
  it("shows the file name alone while the title is on screen", async () => {
    headingRect = { top: 100, bottom: 150 };
    renderViewer({ title: "Project Readme", filePath: "/home/me/code/mo/docs/README.md" });

    const label = await screen.findByTitle("/home/me/code/mo/docs/README.md");
    expect(label.textContent).toBe("README.md");
    // The label must not be part of the rendered markdown content.
    expect(label.closest(".markdown-body")).toBeNull();
  });

  it("folds the title in once the heading scrolls behind the bar", async () => {
    headingRect = { top: -50, bottom: -10 };
    renderViewer({ title: "Project Readme", filePath: "/home/me/code/mo/docs/README.md" });

    const label = await screen.findByTitle("/home/me/code/mo/docs/README.md");
    // The folded text is applied by the post-render geometry effect.
    await waitFor(() => expect(label.textContent).toBe("Project Readme - README.md"));
  });

  it("keeps just the file name when the file has no title", async () => {
    headingRect = { top: -50, bottom: -10 };
    renderViewer({ filePath: "/home/me/code/mo/docs/README.md" });

    const label = await screen.findByTitle("/home/me/code/mo/docs/README.md");
    expect(label.textContent).toBe("README.md");
  });

  it("uses the file name as the tooltip for uploaded files", async () => {
    headingRect = { top: -50, bottom: -10 };
    renderViewer({ title: "Pasted", uploaded: true });

    const label = await screen.findByTitle("README.md");
    await waitFor(() => expect(label.textContent).toBe("Pasted - README.md"));
  });

  it("right-aligns the label text", async () => {
    renderViewer({ title: "Project Readme", filePath: "/home/me/code/mo/docs/README.md" });

    const label = await screen.findByTitle("/home/me/code/mo/docs/README.md");
    expect(label).toHaveClass("text-right");
  });
});
