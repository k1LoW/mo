import { buildFileUrl, groupToPath } from "./groups";

// Upper bound on simultaneously visible panes. Past this the columns are too
// narrow to read, so the layout stops being useful rather than degrading.
export const MAX_PANES = 6;

/**
 * Which files are shown side by side, and which column plain sidebar clicks
 * replace. `fileIds` is ordered left to right; duplicates are allowed so the
 * same document can be pinned in two columns.
 */
export interface PaneState {
  readonly fileIds: readonly string[];
  readonly focusIndex: number;
}

export const EMPTY_PANES: PaneState = { fileIds: [], focusIndex: 0 };

/** Where a file-list click should open the file. */
export type PaneTarget = "focused" | "new-pane";

function clampIndex(index: number, length: number): number {
  if (length === 0) return 0;
  if (!Number.isInteger(index) || index < 0) return 0;
  return Math.min(index, length - 1);
}

export function focusedFileId(state: PaneState): string | null {
  return state.fileIds[state.focusIndex] ?? null;
}

/**
 * Reads the pane layout out of a URL query string. Falls back to the
 * single-file `?file=` form that predates split panes, so old links and
 * bookmarks keep working.
 */
export function parsePanesFromSearch(search: string): PaneState | null {
  const params = new URLSearchParams(search);

  const raw = params.get("files");
  if (raw != null && raw !== "") {
    const fileIds = raw
      .split(",")
      .map((id) => id.trim())
      .filter((id) => id !== "")
      .slice(0, MAX_PANES);
    if (fileIds.length > 0) {
      const parsed = Number.parseInt(params.get("focus") ?? "", 10);
      return {
        fileIds,
        focusIndex: clampIndex(Number.isNaN(parsed) ? 0 : parsed, fileIds.length),
      };
    }
  }

  const single = params.get("file");
  if (single != null && single !== "") {
    return { fileIds: [single], focusIndex: 0 };
  }

  return null;
}

/**
 * Canonical URL for a pane layout. A single pane keeps emitting the legacy
 * `?file=` form so that the common case produces the same shareable links it
 * always has.
 */
export function buildPanesUrl(groupName: string, state: PaneState): string {
  if (state.fileIds.length === 0) return groupToPath(groupName);
  if (state.fileIds.length === 1) return buildFileUrl(groupName, state.fileIds[0]);

  const files = state.fileIds.map(encodeURIComponent).join(",");
  const url = `${groupToPath(groupName)}?files=${files}`;
  return state.focusIndex > 0 ? `${url}&focus=${state.focusIndex}` : url;
}

/**
 * Drops panes whose file is no longer in the group (closed, moved, or deleted)
 * and keeps the focus on the same document when it survives. Returns the input
 * object untouched when nothing changed, so callers can use it inside a state
 * updater without looping.
 */
export function reconcilePanes(state: PaneState, availableIds: ReadonlySet<string>): PaneState {
  const fileIds = state.fileIds.filter((id) => availableIds.has(id));

  // Nothing was dropped, so indices did not shift. Returning early also keeps
  // the focus off the first copy of a file that is open in two panes.
  if (fileIds.length === state.fileIds.length) {
    const focusIndex = clampIndex(state.focusIndex, fileIds.length);
    return focusIndex === state.focusIndex ? state : { fileIds, focusIndex };
  }

  const previousFocusId = focusedFileId(state);
  const survivingFocus = previousFocusId == null ? -1 : fileIds.indexOf(previousFocusId);

  return {
    fileIds,
    focusIndex: survivingFocus >= 0 ? survivingFocus : clampIndex(state.focusIndex, fileIds.length),
  };
}

/** Shows `fileId` in the given pane and focuses it. */
export function setPaneFile(state: PaneState, index: number, fileId: string): PaneState {
  if (state.fileIds.length === 0) return { fileIds: [fileId], focusIndex: 0 };
  if (index < 0 || index >= state.fileIds.length) return state;
  if (state.fileIds[index] === fileId) return focusPane(state, index);

  return {
    fileIds: state.fileIds.map((id, i) => (i === index ? fileId : id)),
    focusIndex: index,
  };
}

/**
 * Opens `fileId` in a new column on the right. A file that is already on screen
 * is focused instead of duplicated — the same thing browser tabs do — and the
 * layout is left alone once MAX_PANES columns are open.
 */
export function addPane(state: PaneState, fileId: string): PaneState {
  const existing = state.fileIds.indexOf(fileId);
  if (existing >= 0) return focusPane(state, existing);
  if (state.fileIds.length >= MAX_PANES) return state;

  return {
    fileIds: [...state.fileIds, fileId],
    focusIndex: state.fileIds.length,
  };
}

export function closePane(state: PaneState, index: number): PaneState {
  if (index < 0 || index >= state.fileIds.length) return state;

  const previousFocusId = focusedFileId(state);
  const fileIds = state.fileIds.filter((_, i) => i !== index);

  // Closing a pane other than the focused one must not move the focus off the
  // document the reader was on, even though its index shifted left.
  const survivingFocus =
    index === state.focusIndex || previousFocusId == null
      ? -1
      : state.focusIndex < index
        ? state.focusIndex
        : state.focusIndex - 1;

  return {
    fileIds,
    focusIndex: survivingFocus >= 0 ? survivingFocus : clampIndex(state.focusIndex, fileIds.length),
  };
}

export function focusPane(state: PaneState, index: number): PaneState {
  if (index < 0 || index >= state.fileIds.length) return state;
  if (index === state.focusIndex) return state;
  return { fileIds: state.fileIds, focusIndex: index };
}
