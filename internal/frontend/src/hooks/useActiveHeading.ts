import { useCallback, useEffect, useRef, useSyncExternalStore } from "react";
import { findElementById } from "../utils/dom";

export function useActiveHeading(
  headingIds: string[],
  scrollContainer: HTMLElement | null,
): string | null {
  const activeIdRef = useRef<string | null>(null);
  const subscribersRef = useRef(new Set<() => void>());

  const subscribe = useCallback((cb: () => void) => {
    subscribersRef.current.add(cb);
    return () => subscribersRef.current.delete(cb);
  }, []);

  const getSnapshot = useCallback(() => activeIdRef.current, []);

  useEffect(() => {
    if (!scrollContainer || headingIds.length === 0) {
      if (activeIdRef.current !== null) {
        activeIdRef.current = null;
        subscribersRef.current.forEach((cb) => cb());
      }
      return;
    }

    // Scoped to the pane's own container: sibling panes may render the same
    // document, and therefore the same heading IDs.
    const elements = headingIds
      .map((id) => findElementById(scrollContainer, id))
      .filter((el): el is HTMLElement => el !== null);

    if (elements.length === 0) return;

    const observer = new IntersectionObserver(
      (entries) => {
        const intersecting = new Set<string>();
        for (const entry of entries) {
          if (entry.isIntersecting) {
            intersecting.add(entry.target.id);
          }
        }

        for (const id of headingIds) {
          if (intersecting.has(id)) {
            if (activeIdRef.current !== id) {
              activeIdRef.current = id;
              subscribersRef.current.forEach((cb) => cb());
            }
            return;
          }
        }
      },
      {
        root: scrollContainer,
        rootMargin: "0px 0px -80% 0px",
      },
    );

    for (const el of elements) {
      observer.observe(el);
    }

    return () => observer.disconnect();
  }, [headingIds, scrollContainer]);

  return useSyncExternalStore(subscribe, getSnapshot);
}
