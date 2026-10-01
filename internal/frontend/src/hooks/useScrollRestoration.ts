import { useCallback, useEffect, useLayoutEffect, useRef } from "react";

export const SCROLL_SESSION_KEY = "mo-scroll-context";

interface ScrollContext {
  headingId: string | null;
  relativeOffset: number;
  rawScrollTop: number;
  fileId: string;
  url: string;
}

function initialHashId(): string | null {
  const raw = window.location.hash.slice(1);
  if (!raw) return null;
  try {
    return decodeURIComponent(raw);
  } catch {
    return raw;
  }
}

export function useScrollRestoration(
  scrollContainer: HTMLElement | null,
  activeHeadingId: string | null,
  activeFileId: string | null,
) {
  const savedContextRef = useRef<ScrollContext | null>(null);
  const pendingRestoreRef = useRef(false);
  const sessionRestoredRef = useRef(false);
  // A heading id from the initial URL hash (e.g. a shared table-of-contents deep
  // link). Applied once, after the first content render, then cleared.
  const hashScrollRef = useRef<string | null>(initialHashId());

  // Single ref object for stable access in beforeunload and captureScrollPosition
  const latestRef = useRef({ scrollContainer, activeHeadingId, activeFileId });
  useLayoutEffect(() => {
    latestRef.current = { scrollContainer, activeHeadingId, activeFileId };
  });

  const captureScrollPosition = useCallback(() => {
    const {
      scrollContainer: sc,
      activeFileId: fileId,
      activeHeadingId: headingId,
    } = latestRef.current;
    if (!sc || !fileId) return;

    const rawScrollTop = sc.scrollTop;
    let relativeOffset = 0;

    if (headingId) {
      const headingEl = document.getElementById(headingId);
      if (headingEl) {
        relativeOffset = headingEl.getBoundingClientRect().top - sc.getBoundingClientRect().top;
      }
    }

    const ctx: ScrollContext = {
      headingId,
      relativeOffset,
      rawScrollTop,
      fileId,
      url: window.location.pathname,
    };

    savedContextRef.current = ctx;
    pendingRestoreRef.current = true;

    try {
      sessionStorage.setItem(SCROLL_SESSION_KEY, JSON.stringify(ctx));
    } catch {
      // sessionStorage may be unavailable
    }
  }, []);

  const restoreFromContext = useCallback((ctx: ScrollContext) => {
    const sc = latestRef.current.scrollContainer;
    if (!sc) return;

    if (ctx.headingId) {
      const headingEl = document.getElementById(ctx.headingId);
      if (headingEl) {
        const currentOffset =
          headingEl.getBoundingClientRect().top - sc.getBoundingClientRect().top;
        sc.scrollTop += currentOffset - ctx.relativeOffset;
        return;
      }
    }

    sc.scrollTop = ctx.rawScrollTop;
  }, []);

  const onContentRendered = useCallback(() => {
    const fileId = latestRef.current.activeFileId;

    // A URL hash points at a specific heading, so it wins over a restored scroll
    // position on load. Consumed once so later renders fall through to restore.
    const hashId = hashScrollRef.current;
    if (hashId) {
      hashScrollRef.current = null;
      const sc = latestRef.current.scrollContainer;
      const headingEl = document.getElementById(hashId);
      if (sc && headingEl) {
        const offset = headingEl.getBoundingClientRect().top - sc.getBoundingClientRect().top;
        sc.scrollTop += offset;
        // Drop any saved position so a later render does not undo the deep link.
        savedContextRef.current = null;
        pendingRestoreRef.current = false;
        sessionRestoredRef.current = true;
        try {
          sessionStorage.removeItem(SCROLL_SESSION_KEY);
        } catch {
          // ignore
        }
        return;
      }
    }

    // Path A: React re-render (ref-based)
    if (pendingRestoreRef.current && savedContextRef.current) {
      const ctx = savedContextRef.current;
      if (ctx.fileId === fileId) {
        restoreFromContext(ctx);
      }
      savedContextRef.current = null;
      pendingRestoreRef.current = false;
      try {
        sessionStorage.removeItem(SCROLL_SESSION_KEY);
      } catch {
        // ignore
      }
      return;
    }

    // Path B: Full page reload (sessionStorage-based, one-shot)
    if (sessionRestoredRef.current) return;
    sessionRestoredRef.current = true;
    try {
      const stored = sessionStorage.getItem(SCROLL_SESSION_KEY);
      if (stored) {
        const ctx: ScrollContext = JSON.parse(stored);
        sessionStorage.removeItem(SCROLL_SESSION_KEY);
        if (ctx.fileId === fileId && ctx.url === window.location.pathname) {
          restoreFromContext(ctx);
        }
      }
    } catch {
      // ignore
    }
  }, [restoreFromContext]);

  // Capture scroll position before any page unload
  useEffect(() => {
    window.addEventListener("beforeunload", captureScrollPosition);
    return () => window.removeEventListener("beforeunload", captureScrollPosition);
  }, [captureScrollPosition]);

  return { captureScrollPosition, onContentRendered };
}
