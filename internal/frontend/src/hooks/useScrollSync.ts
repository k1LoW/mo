import { useCallback, useEffect, useLayoutEffect, useRef } from "react";
import { findElementById } from "../utils/dom";
import { measureSyncPosition, projectSyncPosition, type ScrollGeometry } from "../utils/scrollSync";

/**
 * How long the pane that started scrolling stays in charge. Programmatically
 * setting scrollTop on the other panes fires their own scroll events, and
 * without this window they would immediately try to drive back.
 */
const DRIVER_LOCK_MS = 150;

/** Ignore sub-pixel differences so a settled layout does not keep nudging. */
const SETTLED_THRESHOLD_PX = 1;

export interface SyncPane {
  container: HTMLElement;
  headingIds: readonly string[];
}

interface OffsetCache {
  scrollHeight: number;
  headingIds: readonly string[];
  offsets: number[];
}

export interface UseScrollSyncResult {
  /** Called by each pane; pass null to unregister. */
  registerPane: (index: number, pane: SyncPane | null) => void;
  /** Aligns every other pane to the given one, e.g. when sync is switched on. */
  syncFrom: (index: number) => void;
}

export function useScrollSync(enabled: boolean): UseScrollSyncResult {
  const panes = useRef(new Map<number, SyncPane>());
  const detachers = useRef(new Map<number, () => void>());
  const offsetCache = useRef(new Map<number, OffsetCache>());
  const driver = useRef<{ index: number; until: number } | null>(null);

  const enabledRef = useRef(enabled);
  useLayoutEffect(() => {
    enabledRef.current = enabled;
  });

  const geometryOf = useCallback((index: number, pane: SyncPane): ScrollGeometry => {
    const { container, headingIds } = pane;
    const scrollHeight = container.scrollHeight;
    const cached = offsetCache.current.get(index);

    let offsets: number[];
    if (
      cached != null &&
      cached.scrollHeight === scrollHeight &&
      cached.headingIds === headingIds
    ) {
      offsets = cached.offsets;
    } else {
      // scrollHeight changes whenever the content or layout does, which makes it
      // a cheap invalidation key for offsets that are otherwise costly to
      // recompute on every scroll event.
      const containerTop = container.getBoundingClientRect().top;
      const scrollTop = container.scrollTop;
      offsets = [];
      for (const id of headingIds) {
        const el = findElementById(container, id);
        if (el == null) continue;
        offsets.push(scrollTop + el.getBoundingClientRect().top - containerTop);
      }
      offsetCache.current.set(index, { scrollHeight, headingIds, offsets });
    }

    return {
      scrollTop: container.scrollTop,
      maxScroll: Math.max(0, scrollHeight - container.clientHeight),
      headingOffsets: offsets,
    };
  }, []);

  const applySync = useCallback(
    (index: number) => {
      const source = panes.current.get(index);
      if (source == null) return;

      const position = measureSyncPosition(geometryOf(index, source));

      for (const [otherIndex, other] of panes.current) {
        if (otherIndex === index) continue;
        const top = projectSyncPosition(position, geometryOf(otherIndex, other));
        if (Math.abs(other.container.scrollTop - top) > SETTLED_THRESHOLD_PX) {
          other.container.scrollTop = top;
        }
      }
    },
    [geometryOf],
  );

  const handleScroll = useCallback(
    (index: number) => {
      if (!enabledRef.current || panes.current.size < 2) return;

      const now = performance.now();
      const current = driver.current;
      if (current != null && current.index !== index && now < current.until) return;

      driver.current = { index, until: now + DRIVER_LOCK_MS };
      applySync(index);
    },
    [applySync],
  );

  const registerPane = useCallback(
    (index: number, pane: SyncPane | null) => {
      detachers.current.get(index)?.();
      detachers.current.delete(index);
      panes.current.delete(index);
      offsetCache.current.delete(index);
      if (pane == null) return;

      panes.current.set(index, pane);
      const onScroll = () => handleScroll(index);
      pane.container.addEventListener("scroll", onScroll, { passive: true });
      detachers.current.set(index, () => pane.container.removeEventListener("scroll", onScroll));
    },
    [handleScroll],
  );

  const syncFrom = useCallback(
    (index: number) => {
      if (!enabledRef.current || panes.current.size < 2) return;
      driver.current = { index, until: performance.now() + DRIVER_LOCK_MS };
      applySync(index);
    },
    [applySync],
  );

  useEffect(() => {
    const detachAll = detachers.current;
    return () => {
      for (const detach of detachAll.values()) detach();
      detachAll.clear();
    };
  }, []);

  return { registerPane, syncFrom };
}
