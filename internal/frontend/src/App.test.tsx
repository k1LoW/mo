import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  App,
  FONT_SIZE_STORAGE_KEY,
  formatTitle,
  getInitialFontSize,
  getInitialTocOpenMap,
  isTocOpenForFile,
  TOC_OPEN_STORAGE_KEY,
} from "./App";

// Mocks are hoisted by vi.mock so heavy render dependencies never load in jsdom.
vi.mock("./components/MarkdownViewer", () => ({
  MarkdownViewer: ({ fileId }: { fileId: string }) => <div data-testid="viewer">{fileId}</div>,
}));

vi.mock("./hooks/useSSE", () => ({ useSSE: () => {} }));

vi.mock("./hooks/useFileDrop", () => ({ useFileDrop: () => ({ isDragging: false }) }));

vi.mock("./hooks/useScrollRestoration", () => ({
  SCROLL_SESSION_KEY: "mo-scroll-context",
  useScrollRestoration: () => ({ captureScrollPosition: () => {}, onContentRendered: () => {} }),
}));

describe("getInitialFontSize", () => {
  beforeEach(() => localStorage.clear());
  afterEach(() => localStorage.clear());

  it("returns medium when localStorage is empty", () => {
    expect(getInitialFontSize()).toBe("medium");
  });

  it("returns stored size", () => {
    localStorage.setItem(FONT_SIZE_STORAGE_KEY, "xlarge");
    expect(getInitialFontSize()).toBe("xlarge");
  });

  it("returns medium for invalid value", () => {
    localStorage.setItem(FONT_SIZE_STORAGE_KEY, "huge");
    expect(getInitialFontSize()).toBe("medium");
  });
});

describe("getInitialTocOpenMap", () => {
  beforeEach(() => localStorage.clear());
  afterEach(() => localStorage.clear());

  it("returns empty object when localStorage is empty", () => {
    expect(getInitialTocOpenMap()).toEqual({});
  });

  it("returns stored map", () => {
    localStorage.setItem(TOC_OPEN_STORAGE_KEY, JSON.stringify({ abc123: true, def456: false }));
    expect(getInitialTocOpenMap()).toEqual({ abc123: true, def456: false });
  });

  it("returns empty object for invalid JSON", () => {
    localStorage.setItem(TOC_OPEN_STORAGE_KEY, "not-json");
    expect(getInitialTocOpenMap()).toEqual({});
  });

  it("returns empty object when stored JSON is null", () => {
    localStorage.setItem(TOC_OPEN_STORAGE_KEY, "null");
    expect(getInitialTocOpenMap()).toEqual({});
  });

  it("returns empty object when stored JSON is an array", () => {
    localStorage.setItem(TOC_OPEN_STORAGE_KEY, "[]");
    expect(getInitialTocOpenMap()).toEqual({});
  });
});

describe("isTocOpenForFile", () => {
  it("returns false when fileId is null", () => {
    expect(isTocOpenForFile({ abc: true }, null, "")).toBe(false);
  });

  it("returns false for non-markdown file even if map says true", () => {
    expect(isTocOpenForFile({ abc: true }, "abc", "image.png")).toBe(false);
  });

  it("returns true when map has true for the file", () => {
    expect(isTocOpenForFile({ abc: true }, "abc", "readme.md")).toBe(true);
  });

  it("returns false when map has no entry for the file", () => {
    expect(isTocOpenForFile({}, "abc", "readme.md")).toBe(false);
  });

  it("returns false when map has false for the file", () => {
    expect(isTocOpenForFile({ abc: false }, "abc", "readme.md")).toBe(false);
  });
});

describe("formatTitle", () => {
  it("returns `mo` when fileEntry is undefined", () => {
    expect(formatTitle(undefined)).toBe("mo");
  });

  it("returns `file name` when title is undefined", () => {
    expect(formatTitle({ name: "file.md", title: undefined })).toBe("file.md | mo");
  });

  it("returns `title - file name` when title is defined", () => {
    expect(formatTitle({ name: "file.md", title: "File Title" })).toBe("File Title - file.md | mo");
  });
});

describe("App URL sync", () => {
  const groupsPayload = [
    {
      name: "default",
      files: [
        { id: "aaa11111", name: "README.md", path: "/README.md" },
        { id: "bbb22222", name: "GUIDE.md", path: "/GUIDE.md" },
      ],
    },
    {
      name: "design",
      files: [{ id: "ccc33333", name: "spec.md", path: "/design/spec.md" }],
    },
  ];

  function mockFetch() {
    return vi.fn().mockImplementation((input: RequestInfo | URL) => {
      const url = typeof input === "string" ? input : input.toString();
      if (url === "/_/api/groups") {
        return Promise.resolve({ ok: true, json: async () => groupsPayload });
      }
      return Promise.resolve({ ok: true, json: async () => ({}) });
    });
  }

  function setUrl(url: string) {
    window.history.replaceState(null, "", url);
  }

  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
    setUrl("/");
    vi.stubGlobal("fetch", mockFetch());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    setUrl("/");
  });

  it("updates URL with ?file= when a file is clicked in the sidebar", async () => {
    const user = userEvent.setup();
    render(<App />);

    // Wait for groups to load and the auto-selected first file to render.
    await screen.findByText("GUIDE.md");

    await user.click(screen.getByText("GUIDE.md"));

    await waitFor(() => {
      expect(window.location.pathname + window.location.search).toBe("/?file=bbb22222");
    });
  });

  it("pushes a new history entry on file selection (back returns to the previous file)", async () => {
    const user = userEvent.setup();
    render(<App />);

    await screen.findByText("README.md");
    // The first file is auto-selected; URL is replaced (not pushed).
    await waitFor(() => {
      expect(window.location.search).toBe("?file=aaa11111");
    });

    const startLength = window.history.length;
    await user.click(screen.getByText("GUIDE.md"));

    await waitFor(() => {
      expect(window.location.search).toBe("?file=bbb22222");
    });
    expect(window.history.length).toBe(startLength + 1);
  });

  it("hydrates the active file from ?file= on initial load", async () => {
    setUrl("/?file=bbb22222");
    render(<App />);

    await waitFor(() => {
      expect(screen.getByTestId("viewer")).toHaveTextContent("bbb22222");
    });
    // URL is preserved (no push, no clear).
    expect(window.location.search).toBe("?file=bbb22222");
  });

  it("falls back to the first file when ?file= references an unknown id", async () => {
    setUrl("/?file=zzz99999");
    render(<App />);

    await waitFor(() => {
      expect(screen.getByTestId("viewer")).toHaveTextContent("aaa11111");
    });
    // URL is rewritten via replaceState to match the actual selected file.
    await waitFor(() => {
      expect(window.location.search).toBe("?file=aaa11111");
    });
  });

  it("hydrates a split layout from ?files= on initial load", async () => {
    setUrl("/?files=bbb22222,aaa11111&focus=1");
    render(<App />);

    await waitFor(() => {
      const viewers = screen.getAllByTestId("viewer");
      expect(viewers.map((v) => v.textContent)).toEqual(["bbb22222", "aaa11111"]);
    });
    expect(window.location.search).toBe("?files=bbb22222,aaa11111&focus=1");
  });

  it("drops panes whose file is not in the group and rewrites the URL", async () => {
    setUrl("/?files=aaa11111,zzz99999");
    render(<App />);

    await waitFor(() => {
      expect(screen.getAllByTestId("viewer")).toHaveLength(1);
    });
    await waitFor(() => {
      expect(window.location.search).toBe("?file=aaa11111");
    });
  });

  it("adds a pane on Alt+click and records it in the URL", async () => {
    const user = userEvent.setup();
    render(<App />);

    await screen.findByText("GUIDE.md");
    await waitFor(() => {
      expect(window.location.search).toBe("?file=aaa11111");
    });

    await user.keyboard("[AltLeft>]");
    await user.click(screen.getByText("GUIDE.md"));
    await user.keyboard("[/AltLeft]");

    await waitFor(() => {
      expect(window.location.search).toBe("?files=aaa11111,bbb22222&focus=1");
    });
    expect(screen.getAllByTestId("viewer").map((v) => v.textContent)).toEqual([
      "aaa11111",
      "bbb22222",
    ]);
  });

  it("replaces the focused pane on a plain click instead of adding one", async () => {
    const user = userEvent.setup();
    setUrl("/?files=aaa11111,bbb22222&focus=1");
    render(<App />);

    await waitFor(() => {
      expect(screen.getAllByTestId("viewer")).toHaveLength(2);
    });

    // The second pane is focused, so this swaps its file and leaves pane 0 alone.
    await user.click(screen.getByText("README.md"));

    await waitFor(() => {
      expect(screen.getAllByTestId("viewer").map((v) => v.textContent)).toEqual([
        "aaa11111",
        "aaa11111",
      ]);
    });
  });

  it("restores a split layout via popstate", async () => {
    render(<App />);

    await screen.findByText("README.md");
    await waitFor(() => {
      expect(window.location.search).toBe("?file=aaa11111");
    });

    act(() => {
      window.history.replaceState(null, "", "/?files=aaa11111,bbb22222");
      window.dispatchEvent(new PopStateEvent("popstate"));
    });

    await waitFor(() => {
      expect(screen.getAllByTestId("viewer").map((v) => v.textContent)).toEqual([
        "aaa11111",
        "bbb22222",
      ]);
    });
    expect(window.location.search).toBe("?files=aaa11111,bbb22222");
  });

  it("offers the scroll sync toggle only in split view", async () => {
    setUrl("/?file=aaa11111");
    const single = render(<App />);
    await waitFor(() => {
      expect(screen.getAllByTestId("viewer")).toHaveLength(1);
    });
    expect(screen.queryByLabelText("Sync scrolling")).toBeNull();
    single.unmount();

    setUrl("/?files=aaa11111,bbb22222");
    render(<App />);
    await waitFor(() => {
      expect(screen.getAllByTestId("viewer")).toHaveLength(2);
    });
    expect(screen.getByLabelText("Sync scrolling")).toBeInTheDocument();
  });

  it("collapses to the focused pane on a narrow window and keeps the rest reachable", async () => {
    const user = userEvent.setup();
    vi.stubGlobal("matchMedia", (query: string) => ({
      matches: query.includes("max-width"),
      media: query,
      addEventListener: () => {},
      removeEventListener: () => {},
    }));

    setUrl("/?files=aaa11111,bbb22222&focus=1");
    render(<App />);

    await waitFor(() => {
      const viewers = screen.getAllByTestId("viewer");
      expect(viewers).toHaveLength(1);
      expect(viewers[0]).toHaveTextContent("bbb22222");
    });

    // The layout itself is untouched, so the navigator still reports both panes.
    expect(screen.getByText("2/2")).toBeInTheDocument();

    await user.click(screen.getByLabelText("Previous pane"));
    await waitFor(() => {
      expect(screen.getAllByTestId("viewer")[0]).toHaveTextContent("aaa11111");
    });
    expect(window.location.search).toBe("?files=aaa11111,bbb22222");
  });

  it("follows back/forward navigation via popstate", async () => {
    const user = userEvent.setup();
    render(<App />);

    await screen.findByText("README.md");
    await waitFor(() => {
      expect(window.location.search).toBe("?file=aaa11111");
    });

    await user.click(screen.getByText("GUIDE.md"));
    await waitFor(() => {
      expect(window.location.search).toBe("?file=bbb22222");
    });

    // Simulate browser Back: rewind URL, then dispatch popstate as the browser would.
    act(() => {
      window.history.replaceState(null, "", "/?file=aaa11111");
      window.dispatchEvent(new PopStateEvent("popstate"));
    });

    await waitFor(() => {
      expect(screen.getByTestId("viewer")).toHaveTextContent("aaa11111");
    });
    // URL is not re-pushed by the popstate-driven state change.
    expect(window.location.search).toBe("?file=aaa11111");
  });
});
