import { useCallback, useEffect, useRef, useState } from "react";
import {
  equalWeights,
  PANE_WIDTHS_STORAGE_KEY,
  readStoredWeights,
  resizeWeights,
} from "../utils/paneWidths";

interface DragState {
  boundary: number;
  startX: number;
  /** Pixel widths of every column when the drag started. */
  startWidths: number[];
}

export interface UsePaneWidthsResult {
  /** flex-grow value for each column, in order. */
  weights: number[];
  /** Begins a drag on the boundary to the left of pane `boundary`. */
  startResize: (boundary: number, clientX: number) => void;
  resetWidths: () => void;
}

/**
 * Drag-to-resize for the pane columns, following the same pattern as `Sidebar`
 * and `TocPanel`: track the drag in a ref, listen on the document, persist to
 * localStorage. Weights are relative rather than absolute so the layout keeps
 * its proportions when the window is resized.
 */
export function usePaneWidths(
  paneCount: number,
  containerRef: React.RefObject<HTMLElement | null>,
): UsePaneWidthsResult {
  const [weights, setWeights] = useState<number[]>(() => readStoredWeights(paneCount));
  const [prevPaneCount, setPrevPaneCount] = useState(paneCount);
  const dragRef = useRef<DragState | null>(null);

  // Opening or closing a column invalidates the current proportions.
  if (prevPaneCount !== paneCount) {
    setPrevPaneCount(paneCount);
    setWeights(readStoredWeights(paneCount));
  }

  const startResize = useCallback(
    (boundary: number, clientX: number) => {
      const container = containerRef.current;
      if (container == null) return;

      const startWidths = Array.from(
        container.children,
        (child) => child.getBoundingClientRect().width,
      );
      dragRef.current = { boundary, startX: clientX, startWidths };
      document.body.style.cursor = "col-resize";
      document.body.style.userSelect = "none";
    },
    [containerRef],
  );

  useEffect(() => {
    const onMouseMove = (e: MouseEvent) => {
      const drag = dragRef.current;
      if (drag == null) return;
      setWeights(resizeWeights(drag.startWidths, drag.boundary, e.clientX - drag.startX));
    };
    const onMouseUp = () => {
      if (dragRef.current == null) return;
      dragRef.current = null;
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
    };

    document.addEventListener("mousemove", onMouseMove);
    document.addEventListener("mouseup", onMouseUp);
    return () => {
      document.removeEventListener("mousemove", onMouseMove);
      document.removeEventListener("mouseup", onMouseUp);
    };
  }, []);

  useEffect(() => {
    try {
      localStorage.setItem(PANE_WIDTHS_STORAGE_KEY, JSON.stringify(weights));
    } catch {
      /* ignore */
    }
  }, [weights]);

  const resetWidths = useCallback(() => {
    setWeights(equalWeights(paneCount));
  }, [paneCount]);

  return { weights, startResize, resetWidths };
}
