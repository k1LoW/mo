import type { MouseEvent } from "react";
import { isAltLeftClick, isPlainLeftClick } from "./linkClick";
import type { PaneTarget } from "./panes";

/**
 * Styling shared by the sidebar's flat list and tree view.
 *
 * A file open in any pane gets the highlighted treatment; the one in the focused
 * pane additionally gets an accent bar, so during a three-way comparison it is
 * clear which column a plain click will replace. Both states reserve the 2px
 * border so labels never shift.
 */
export function fileRowClass(isOpen: boolean, isFocused: boolean): string {
  const base =
    "flex items-center gap-2 w-full pl-2.5 pr-3 py-2 border-l-2 cursor-pointer text-left text-sm no-underline transition-colors duration-150";
  const openState = isOpen
    ? "bg-gh-bg-active text-gh-text font-semibold"
    : "bg-transparent text-gh-text-secondary hover:bg-gh-bg-hover";
  const focusState = isFocused ? "border-l-gh-accent" : "border-l-transparent";
  return `${base} ${openState} ${focusState}`;
}

/**
 * Click behaviour shared by every file row. Alt+click opens a new column;
 * every other modifier keeps the browser's own meaning (new tab, new window).
 */
export function handleFileRowClick(
  e: MouseEvent,
  fileId: string,
  onFileSelect: (id: string, target: PaneTarget) => void,
): void {
  if (isAltLeftClick(e)) {
    e.preventDefault();
    onFileSelect(fileId, "new-pane");
    return;
  }
  if (!isPlainLeftClick(e)) return;
  e.preventDefault();
  onFileSelect(fileId, "focused");
}
