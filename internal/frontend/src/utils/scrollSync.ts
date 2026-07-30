/**
 * Scroll synchronisation between panes.
 *
 * The panes being compared are usually translations of one source, so their
 * heading *text* differs ("概要" vs "要約") while their heading *structure* is
 * parallel. That rules out matching by heading ID — the nth heading of one
 * document corresponds to the nth heading of the others, so position is
 * expressed as "heading ordinal + how far through that section", and only falls
 * back to a whole-document scroll ratio when the ordinals cannot line up.
 */

export interface ScrollGeometry {
  scrollTop: number;
  /** scrollHeight - clientHeight, i.e. the largest valid scrollTop. */
  maxScroll: number;
  /** Content offset of each heading, in document order. */
  headingOffsets: readonly number[];
}

export interface SyncPosition {
  /** Index of the heading the reader is inside; -1 when above the first one. */
  headingIndex: number;
  /** Progress through the current section, 0..1. */
  fraction: number;
  /** Whole-document progress, used when heading ordinals cannot be matched. */
  ratio: number;
}

function clamp01(value: number): number {
  if (!Number.isFinite(value)) return 0;
  return Math.min(1, Math.max(0, value));
}

function clamp(value: number, min: number, max: number): number {
  if (!Number.isFinite(value)) return min;
  return Math.min(max, Math.max(min, value));
}

/** Index of the last heading at or above the current scroll position. */
function currentHeadingIndex(offsets: readonly number[], scrollTop: number): number {
  let index = -1;
  for (let i = 0; i < offsets.length; i++) {
    if (offsets[i] > scrollTop) break;
    index = i;
  }
  return index;
}

export function measureSyncPosition(geometry: ScrollGeometry): SyncPosition {
  const { scrollTop, maxScroll, headingOffsets } = geometry;
  const ratio = maxScroll > 0 ? clamp01(scrollTop / maxScroll) : 0;

  if (headingOffsets.length === 0) {
    return { headingIndex: -1, fraction: 0, ratio };
  }

  const headingIndex = currentHeadingIndex(headingOffsets, scrollTop);

  if (headingIndex === -1) {
    // Above the first heading: progress through the document's preamble.
    const span = headingOffsets[0];
    return { headingIndex, fraction: span > 0 ? clamp01(scrollTop / span) : 0, ratio };
  }

  const start = headingOffsets[headingIndex];
  const isLast = headingIndex + 1 >= headingOffsets.length;
  const end = isLast ? maxScroll : headingOffsets[headingIndex + 1];
  const span = end - start;

  return { headingIndex, fraction: span > 0 ? clamp01((scrollTop - start) / span) : 0, ratio };
}

/** The scrollTop that puts `target` at the same relative position. */
export function projectSyncPosition(position: SyncPosition, target: ScrollGeometry): number {
  const { maxScroll, headingOffsets } = target;

  // No matching ordinal (fewer headings, or none at all) — fall back to ratio.
  if (headingOffsets.length === 0 || position.headingIndex >= headingOffsets.length) {
    return clamp(position.ratio * maxScroll, 0, maxScroll);
  }

  if (position.headingIndex === -1) {
    return clamp(position.fraction * headingOffsets[0], 0, maxScroll);
  }

  const start = headingOffsets[position.headingIndex];
  const isLast = position.headingIndex + 1 >= headingOffsets.length;
  const end = isLast ? maxScroll : headingOffsets[position.headingIndex + 1];

  return clamp(start + position.fraction * (end - start), 0, maxScroll);
}
