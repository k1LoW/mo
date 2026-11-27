import { useCallback, useEffect, useLayoutEffect, useRef } from "react";
import { headingIdFromHash, scrollToHeading } from "../utils/jumpToHeading";
import { anchorScroll } from "../utils/scrollAnchor";
import { parseFileIdFromSearch, parseGroupFromPath } from "../utils/groups";

interface PendingHeading {
  group: string;
  fileId: string | null;
  id: string;
}

function currentFileId(): string | null {
  return parseFileIdFromSearch(window.location.search);
}

function pendingFromUrl(fileId: string | null): PendingHeading | null {
  const id = headingIdFromHash(window.location.hash);
  if (!id) return null;
  return { group: parseGroupFromPath(window.location.pathname), fileId, id };
}

function matchesCurrentUrl(pending: PendingHeading): boolean {
  return (
    pending.group === parseGroupFromPath(window.location.pathname) &&
    pending.fileId === currentFileId() &&
    pending.id === headingIdFromHash(window.location.hash)
  );
}

// Back/forward between entries that differ only in the hash (left by table of
// contents clicks and in-document anchor links) changes the URL without
// scrolling, because the document scrolls inside a container rather than the
// window. Scroll to the hash's heading, or to the top when the hash is gone.
// A traversal to another file lands on its hash's heading once that file has
// rendered.
export function useHashNavigation(
  scrollContainer: HTMLElement | null,
  activeGroup: string,
  activeFileId: string | null,
) {
  const pendingRef = useRef<PendingHeading | null>(null);
  // The file whose content was last rendered, not the active file state: browsers
  // run a microtask checkpoint between popstate listeners, so App's popstate
  // handler may already have re-rendered with the new file before these listeners
  // run. The group is left out because a file shows the same content in every
  // group, and a group switch that keeps the file does not render again.
  const renderedFileRef = useRef<string | null>(null);
  const latestRef = useRef({ scrollContainer, activeGroup, activeFileId });
  useLayoutEffect(() => {
    latestRef.current = { scrollContainer, activeGroup, activeFileId };
  });

  useEffect(() => {
    const handlePopState = () => {
      pendingRef.current = null;
      const pending = pendingFromUrl(currentFileId());
      if (!pending) return;
      // The last rendered file may already have been replaced by another file's
      // loading state, so only a mounted heading proves hashchange can reach it.
      if (renderedFileRef.current === pending.fileId && document.getElementById(pending.id)) {
        return;
      }
      pendingRef.current = pending;
    };
    const handleHashChange = () => {
      // A traversal to another file also fires hashchange, after popstate. That
      // file has not rendered yet, so the pending heading is applied on render.
      if (renderedFileRef.current !== currentFileId()) return;
      const id = headingIdFromHash(window.location.hash);
      if (id) {
        scrollToHeading(id);
        return;
      }
      const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      latestRef.current.scrollContainer?.scrollTo({
        top: 0,
        behavior: reduced ? "auto" : "smooth",
      });
    };
    window.addEventListener("popstate", handlePopState);
    window.addEventListener("hashchange", handleHashChange);
    return () => {
      window.removeEventListener("popstate", handlePopState);
      window.removeEventListener("hashchange", handleHashChange);
    };
  }, []);

  const onContentRendered = useCallback(() => {
    const { scrollContainer: sc, activeFileId: fileId } = latestRef.current;
    renderedFileRef.current = fileId;
    const pending = pendingRef.current;
    if (!pending) return;
    // Sidebar and link navigation use pushState, which fires no popstate, so a
    // heading left pending by an interrupted traversal is checked against the URL.
    if (!matchesCurrentUrl(pending)) {
      pendingRef.current = null;
      return;
    }
    if (pending.fileId !== fileId) return;
    pendingRef.current = null;
    const headingEl = document.getElementById(pending.id);
    if (sc && headingEl) {
      // Jump rather than scrollIntoView so a smooth scroll does not animate from
      // the top of a document the reader never saw at that position.
      anchorScroll(sc, headingEl);
    }
  }, []);

  // Called after pushState or replaceState, which do not fire popstate.
  const followHash = useCallback((fileId: string) => {
    pendingRef.current = null;
    const pending = pendingFromUrl(fileId);
    if (!pending) return;
    if (renderedFileRef.current === fileId && scrollToHeading(pending.id)) return;
    pendingRef.current = pending;
  }, []);

  return { onContentRendered, followHash };
}
