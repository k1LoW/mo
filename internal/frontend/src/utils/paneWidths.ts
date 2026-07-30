export const PANE_WIDTHS_STORAGE_KEY = "mo-pane-widths";

/** Narrowest a column may be dragged. Below this the prose stops being readable. */
export const MIN_PANE_WIDTH = 260;

/**
 * Relative column weights, used as `flex-grow` against `flex-basis: 0`. Storing
 * ratios rather than pixels means the layout survives a window resize.
 */
export function equalWeights(paneCount: number): number[] {
  return Array.from({ length: paneCount }, () => 1);
}

export function readStoredWeights(paneCount: number): number[] {
  if (paneCount <= 0) return [];
  try {
    const stored = localStorage.getItem(PANE_WIDTHS_STORAGE_KEY);
    if (stored) {
      const parsed = JSON.parse(stored);
      // A stored layout only applies to the same number of columns; anything
      // else would silently distort widths after opening or closing a pane.
      if (
        Array.isArray(parsed) &&
        parsed.length === paneCount &&
        parsed.every((w) => typeof w === "number" && Number.isFinite(w) && w > 0)
      ) {
        return parsed;
      }
    }
  } catch {
    /* ignore */
  }
  return equalWeights(paneCount);
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

/**
 * Applies a drag on the boundary to the left of pane `boundary`, moving space
 * between that pane and its left neighbour while leaving every other column
 * alone. Widths are in pixels as measured at drag start; the result doubles as
 * the new weight array.
 */
export function resizeWeights(
  widths: readonly number[],
  boundary: number,
  delta: number,
): number[] {
  const left = boundary - 1;
  const right = boundary;
  if (left < 0 || right >= widths.length) return [...widths];

  const total = widths[left] + widths[right];
  // Not enough room for both minimums — leave the layout as it is rather than
  // squeezing one column to nothing.
  if (total < MIN_PANE_WIDTH * 2) return [...widths];

  const nextLeft = clamp(widths[left] + delta, MIN_PANE_WIDTH, total - MIN_PANE_WIDTH);
  const next = [...widths];
  next[left] = nextLeft;
  next[right] = total - nextLeft;
  return next;
}
