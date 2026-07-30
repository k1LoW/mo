import { useCallback, useEffect, useLayoutEffect, useRef } from "react";
import { findElementById } from "../utils/dom";

export const SCROLL_SESSION_KEY = "mo-scroll-context";

/**
 * Each pane restores its own scroll position, so they cannot share one key.
 * Pane 0 keeps the unsuffixed key it has always used, which is also the key
 * App reads to recover the focused file after a reload.
 */
export function scrollSessionKey(paneIndex: number): string {
  return paneIndex === 0 ? SCROLL_SESSION_KEY : `${SCROLL_SESSION_KEY}:${paneIndex}`;
}

interface ScrollContext {
  headingId: string | null;
  relativeOffset: number;
  rawScrollTop: number;
  fileId: string;
  url: string;
}

export function useScrollRestoration(
  scrollContainer: HTMLElement | null,
  activeHeadingId: string | null,
  activeFileId: string | null,
  paneIndex = 0,
) {
  const savedContextRef = useRef<ScrollContext | null>(null);
  const pendingRestoreRef = useRef(false);
  const sessionRestoredRef = useRef(false);

  // Single ref object for stable access in beforeunload and captureScrollPosition
  const latestRef = useRef({ scrollContainer, activeHeadingId, activeFileId, paneIndex });
  useLayoutEffect(() => {
    latestRef.current = { scrollContainer, activeHeadingId, activeFileId, paneIndex };
  });

  const captureScrollPosition = useCallback(() => {
    const {
      scrollContainer: sc,
      activeFileId: fileId,
      activeHeadingId: headingId,
      paneIndex: index,
    } = latestRef.current;
    if (!sc || !fileId) return;

    const rawScrollTop = sc.scrollTop;
    let relativeOffset = 0;

    if (headingId) {
      const headingEl = findElementById(sc, headingId);
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
      sessionStorage.setItem(scrollSessionKey(index), JSON.stringify(ctx));
    } catch {
      // sessionStorage may be unavailable
    }
  }, []);

  const restoreFromContext = useCallback((ctx: ScrollContext) => {
    const sc = latestRef.current.scrollContainer;
    if (!sc) return;

    if (ctx.headingId) {
      const headingEl = findElementById(sc, ctx.headingId);
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
    const { activeFileId: fileId, paneIndex: index } = latestRef.current;

    // Path A: React re-render (ref-based)
    if (pendingRestoreRef.current && savedContextRef.current) {
      const ctx = savedContextRef.current;
      if (ctx.fileId === fileId) {
        restoreFromContext(ctx);
      }
      savedContextRef.current = null;
      pendingRestoreRef.current = false;
      try {
        sessionStorage.removeItem(scrollSessionKey(index));
      } catch {
        // ignore
      }
      return;
    }

    // Path B: Full page reload (sessionStorage-based, one-shot)
    if (sessionRestoredRef.current) return;
    sessionRestoredRef.current = true;
    try {
      const stored = sessionStorage.getItem(scrollSessionKey(index));
      if (stored) {
        const ctx: ScrollContext = JSON.parse(stored);
        sessionStorage.removeItem(scrollSessionKey(index));
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
