import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useScrollRestoration, SCROLL_SESSION_KEY, scrollSessionKey } from "./useScrollRestoration";

function makeContainer(scrollTop = 0): HTMLDivElement {
  const el = document.createElement("div");
  Object.defineProperty(el, "scrollTop", { value: scrollTop, writable: true });
  el.getBoundingClientRect = () => ({
    top: 0,
    left: 0,
    right: 800,
    bottom: 600,
    width: 800,
    height: 600,
    x: 0,
    y: 0,
    toJSON() {},
  });
  return el;
}

// Headings are looked up inside the pane's scroll container, because sibling
// panes can render the same heading IDs. The fixture mirrors that nesting.
function addHeading(id: string, topOffset: number, parent: HTMLElement) {
  const el = document.createElement("h2");
  el.id = id;
  el.getBoundingClientRect = () => ({
    top: topOffset,
    left: 0,
    right: 800,
    bottom: topOffset + 30,
    width: 800,
    height: 30,
    x: 0,
    y: 0,
    toJSON() {},
  });
  parent.appendChild(el);
  return el;
}

beforeEach(() => {
  sessionStorage.clear();
});

afterEach(() => {
  document.body.innerHTML = "";
  sessionStorage.clear();
});

describe("scrollSessionKey", () => {
  it("keeps the unsuffixed key for the first pane", () => {
    expect(scrollSessionKey(0)).toBe(SCROLL_SESSION_KEY);
  });

  it("suffixes every other pane", () => {
    expect(scrollSessionKey(1)).toBe(`${SCROLL_SESSION_KEY}:1`);
    expect(scrollSessionKey(2)).toBe(`${SCROLL_SESSION_KEY}:2`);
  });
});

describe("useScrollRestoration", () => {
  describe("Path A: heading-based restore", () => {
    it("restores scroll position using heading offset", () => {
      const container = makeContainer(400);
      const heading = addHeading("section-3", 120, container);

      const { result } = renderHook(
        ({ sc, headingId, fileId }) => useScrollRestoration(sc, headingId, fileId),
        {
          initialProps: {
            sc: container as HTMLElement | null,
            headingId: "section-3" as string | null,
            fileId: "abc" as string | null,
          },
        },
      );

      // Capture current position
      act(() => {
        result.current.captureScrollPosition();
      });

      // Simulate heading moved after re-render (e.g., content changed)
      heading.getBoundingClientRect = () => ({
        top: 150,
        left: 0,
        right: 800,
        bottom: 180,
        width: 800,
        height: 30,
        x: 0,
        y: 0,
        toJSON() {},
      });
      container.scrollTop = 0; // Reset as if re-rendered

      // Trigger restore
      act(() => {
        result.current.onContentRendered();
      });

      // scrollTop should be adjusted: 0 + (150 - 120) = 30
      expect(container.scrollTop).toBe(30);
    });
  });

  describe("Path A: rawScrollTop fallback", () => {
    it("falls back to rawScrollTop when heading is removed", () => {
      const container = makeContainer(500);
      const heading = addHeading("temp-heading", 200, container);

      const { result } = renderHook(
        ({ sc, headingId, fileId }) => useScrollRestoration(sc, headingId, fileId),
        {
          initialProps: {
            sc: container as HTMLElement | null,
            headingId: "temp-heading" as string | null,
            fileId: "abc" as string | null,
          },
        },
      );

      act(() => {
        result.current.captureScrollPosition();
      });

      // Remove the heading (simulates heading deleted from markdown)
      heading.remove();
      container.scrollTop = 0;

      act(() => {
        result.current.onContentRendered();
      });

      expect(container.scrollTop).toBe(500);
    });

    it("uses rawScrollTop when no heading was active", () => {
      const container = makeContainer(300);

      const { result } = renderHook(
        ({ sc, headingId, fileId }) => useScrollRestoration(sc, headingId, fileId),
        {
          initialProps: {
            sc: container as HTMLElement | null,
            headingId: null as string | null,
            fileId: "abc" as string | null,
          },
        },
      );

      act(() => {
        result.current.captureScrollPosition();
      });
      container.scrollTop = 0;

      act(() => {
        result.current.onContentRendered();
      });

      expect(container.scrollTop).toBe(300);
    });
  });

  describe("Path B: sessionStorage restore on reload", () => {
    it("restores from sessionStorage and removes the entry", () => {
      const container = makeContainer(0);

      // Pre-populate sessionStorage as if saved before reload
      const ctx = {
        headingId: null,
        relativeOffset: 0,
        rawScrollTop: 750,
        fileId: "file1",
        url: "/",
      };
      sessionStorage.setItem(SCROLL_SESSION_KEY, JSON.stringify(ctx));

      const { result } = renderHook(
        ({ sc, headingId, fileId }) => useScrollRestoration(sc, headingId, fileId),
        {
          initialProps: {
            sc: container as HTMLElement | null,
            headingId: null as string | null,
            fileId: "file1" as string | null,
          },
        },
      );

      act(() => {
        result.current.onContentRendered();
      });

      expect(container.scrollTop).toBe(750);
      expect(sessionStorage.getItem(SCROLL_SESSION_KEY)).toBeNull();
    });

    it("skips restore when fileId does not match", () => {
      const container = makeContainer(0);

      sessionStorage.setItem(
        SCROLL_SESSION_KEY,
        JSON.stringify({
          headingId: null,
          relativeOffset: 0,
          rawScrollTop: 750,
          fileId: "other-file",
          url: "/",
        }),
      );

      const { result } = renderHook(
        ({ sc, headingId, fileId }) => useScrollRestoration(sc, headingId, fileId),
        {
          initialProps: {
            sc: container as HTMLElement | null,
            headingId: null as string | null,
            fileId: "file1" as string | null,
          },
        },
      );

      act(() => {
        result.current.onContentRendered();
      });

      expect(container.scrollTop).toBe(0);
      expect(sessionStorage.getItem(SCROLL_SESSION_KEY)).toBeNull();
    });

    it("only attempts sessionStorage restore once", () => {
      const container = makeContainer(0);

      sessionStorage.setItem(
        SCROLL_SESSION_KEY,
        JSON.stringify({
          headingId: null,
          relativeOffset: 0,
          rawScrollTop: 400,
          fileId: "file1",
          url: "/",
        }),
      );

      const { result } = renderHook(
        ({ sc, headingId, fileId }) => useScrollRestoration(sc, headingId, fileId),
        {
          initialProps: {
            sc: container as HTMLElement | null,
            headingId: null as string | null,
            fileId: "file1" as string | null,
          },
        },
      );

      act(() => {
        result.current.onContentRendered();
      });
      expect(container.scrollTop).toBe(400);

      // Reset and add new sessionStorage entry
      container.scrollTop = 0;
      sessionStorage.setItem(
        SCROLL_SESSION_KEY,
        JSON.stringify({
          headingId: null,
          relativeOffset: 0,
          rawScrollTop: 999,
          fileId: "file1",
          url: "/",
        }),
      );

      // Second call should not restore (one-shot guard)
      act(() => {
        result.current.onContentRendered();
      });
      expect(container.scrollTop).toBe(0);
    });
  });

  describe("captureScrollPosition", () => {
    it("writes to sessionStorage", () => {
      const container = makeContainer(250);

      const { result } = renderHook(
        ({ sc, headingId, fileId }) => useScrollRestoration(sc, headingId, fileId),
        {
          initialProps: {
            sc: container as HTMLElement | null,
            headingId: null as string | null,
            fileId: "abc" as string | null,
          },
        },
      );

      act(() => {
        result.current.captureScrollPosition();
      });

      const stored = sessionStorage.getItem(SCROLL_SESSION_KEY);
      expect(stored).not.toBeNull();
      const ctx = JSON.parse(stored!);
      expect(ctx.rawScrollTop).toBe(250);
      expect(ctx.fileId).toBe("abc");
    });

    it("writes each pane to its own key so panes do not overwrite each other", () => {
      const paneOne = makeContainer(120);
      const paneTwo = makeContainer(640);

      const one = renderHook(() => useScrollRestoration(paneOne, null, "abc", 0));
      const two = renderHook(() => useScrollRestoration(paneTwo, null, "abc", 1));

      act(() => {
        one.result.current.captureScrollPosition();
        two.result.current.captureScrollPosition();
      });

      expect(JSON.parse(sessionStorage.getItem(scrollSessionKey(0))!).rawScrollTop).toBe(120);
      expect(JSON.parse(sessionStorage.getItem(scrollSessionKey(1))!).rawScrollTop).toBe(640);
    });

    it("skips capture when no scroll container", () => {
      const { result } = renderHook(
        ({ sc, headingId, fileId }) => useScrollRestoration(sc, headingId, fileId),
        {
          initialProps: {
            sc: null as HTMLElement | null,
            headingId: null as string | null,
            fileId: "abc" as string | null,
          },
        },
      );

      act(() => {
        result.current.captureScrollPosition();
      });

      expect(sessionStorage.getItem(SCROLL_SESSION_KEY)).toBeNull();
    });
  });
});
