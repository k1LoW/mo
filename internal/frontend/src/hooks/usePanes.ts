import { useCallback, useLayoutEffect, useRef, useState } from "react";
import {
  addPane,
  buildPanesUrl,
  closePane,
  focusPane,
  setPaneFile,
  type PaneState,
} from "../utils/panes";

export interface UsePanesResult {
  panes: PaneState;
  /**
   * Raw setter for automatic changes (group reload, SSE, reconciliation). It
   * does not touch history; App reconciles the URL with `replaceState`.
   */
  setPanes: React.Dispatch<React.SetStateAction<PaneState>>;
  /** Shows the file in the focused column, replacing whatever was there. */
  openInFocusedPane: (fileId: string) => void;
  /** Adds a column on the right showing the file. */
  openInNewPane: (fileId: string) => void;
  closePaneAt: (index: number) => void;
  focusPaneAt: (index: number) => void;
}

export function usePanes(activeGroup: string, initialPanes: PaneState): UsePanesResult {
  const [panes, setPanes] = useState<PaneState>(initialPanes);

  // pushState is a side effect, so it must not run inside a state updater —
  // React may invoke updaters twice and we would push two history entries.
  // This mirror lets the actions below compute the next state outside of one.
  const panesRef = useRef(panes);
  useLayoutEffect(() => {
    panesRef.current = panes;
  });

  // Pane changes the user asked for own the URL: pushing a history entry makes
  // browser Back return to the previous layout.
  const navigate = useCallback(
    (transform: (prev: PaneState) => PaneState) => {
      const prev = panesRef.current;
      const next = transform(prev);
      if (next === prev) return;
      panesRef.current = next;
      window.history.pushState(null, "", buildPanesUrl(activeGroup, next));
      setPanes(next);
    },
    [activeGroup],
  );

  const openInFocusedPane = useCallback(
    (fileId: string) => {
      navigate((prev) => setPaneFile(prev, prev.focusIndex, fileId));
    },
    [navigate],
  );

  const openInNewPane = useCallback(
    (fileId: string) => {
      navigate((prev) => addPane(prev, fileId));
    },
    [navigate],
  );

  const closePaneAt = useCallback(
    (index: number) => {
      navigate((prev) => closePane(prev, index));
    },
    [navigate],
  );

  // Clicking between columns happens constantly, so focus must not push history
  // entries. App's URL reconciliation picks the change up via replaceState.
  const focusPaneAt = useCallback((index: number) => {
    setPanes((prev) => focusPane(prev, index));
  }, []);

  return { panes, setPanes, openInFocusedPane, openInNewPane, closePaneAt, focusPaneAt };
}
