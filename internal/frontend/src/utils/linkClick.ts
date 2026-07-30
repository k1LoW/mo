import type { MouseEvent } from "react";

export function isPlainLeftClick(e: MouseEvent): boolean {
  return e.button === 0 && !e.metaKey && !e.ctrlKey && !e.shiftKey && !e.altKey;
}

/**
 * Alt/Option+left-click, mo's "open in a new pane" gesture.
 *
 * Alt is the only modifier available on a link: Cmd/Ctrl means new browser tab
 * and Shift means new window, both of which the sidebar deliberately keeps.
 * Alt+click on a link merely downloads it, which is useless for an SPA route,
 * so overriding it costs nothing.
 */
export function isAltLeftClick(e: MouseEvent): boolean {
  return e.button === 0 && e.altKey && !e.metaKey && !e.ctrlKey && !e.shiftKey;
}
