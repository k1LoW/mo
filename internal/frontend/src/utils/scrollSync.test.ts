import { describe, expect, it } from "vitest";
import {
  measureSyncPosition,
  projectSyncPosition,
  type ScrollGeometry,
  type SyncPosition,
} from "./scrollSync";

function geometry(
  scrollTop: number,
  maxScroll: number,
  headingOffsets: number[] = [],
): ScrollGeometry {
  return { scrollTop, maxScroll, headingOffsets };
}

/** Two documents with parallel structure but different section lengths. */
const DRIVER = [100, 400, 900, 1500];
const TARGET = [120, 500, 800, 1900];

describe("measureSyncPosition", () => {
  it("reports the preamble before the first heading", () => {
    const pos = measureSyncPosition(geometry(50, 2000, DRIVER));
    expect(pos.headingIndex).toBe(-1);
    expect(pos.fraction).toBeCloseTo(0.5);
  });

  it("reports progress through the section the reader is in", () => {
    // Halfway between heading 1 (400) and heading 2 (900).
    const pos = measureSyncPosition(geometry(650, 2000, DRIVER));
    expect(pos.headingIndex).toBe(1);
    expect(pos.fraction).toBeCloseTo(0.5);
  });

  it("lands exactly on a heading with zero fraction", () => {
    const pos = measureSyncPosition(geometry(900, 2000, DRIVER));
    expect(pos).toMatchObject({ headingIndex: 2, fraction: 0 });
  });

  it("measures the last section against the end of the document", () => {
    // Last heading at 1500, document ends at 2000, so 1750 is halfway.
    const pos = measureSyncPosition(geometry(1750, 2000, DRIVER));
    expect(pos.headingIndex).toBe(3);
    expect(pos.fraction).toBeCloseTo(0.5);
  });

  it("falls back to a plain ratio when the document has no headings", () => {
    const pos = measureSyncPosition(geometry(500, 2000, []));
    expect(pos).toMatchObject({ headingIndex: -1, fraction: 0 });
    expect(pos.ratio).toBeCloseTo(0.25);
  });

  it("reports a zero ratio for a document that does not scroll", () => {
    expect(measureSyncPosition(geometry(0, 0, [])).ratio).toBe(0);
  });

  it("clamps a scroll position past the end of the document", () => {
    const pos = measureSyncPosition(geometry(5000, 2000, DRIVER));
    expect(pos.fraction).toBe(1);
    expect(pos.ratio).toBe(1);
  });
});

describe("projectSyncPosition", () => {
  it("maps a section midpoint onto the target's own section", () => {
    const pos = measureSyncPosition(geometry(650, 2000, DRIVER));
    // Target heading 1 at 500, heading 2 at 800 → halfway is 650.
    expect(projectSyncPosition(pos, geometry(0, 2400, TARGET))).toBeCloseTo(650);
  });

  it("aligns headings exactly even when section lengths differ", () => {
    const pos = measureSyncPosition(geometry(900, 2000, DRIVER));
    expect(projectSyncPosition(pos, geometry(0, 2400, TARGET))).toBe(800);
  });

  it("maps the preamble proportionally", () => {
    const pos = measureSyncPosition(geometry(50, 2000, DRIVER));
    expect(projectSyncPosition(pos, geometry(0, 2400, TARGET))).toBeCloseTo(60);
  });

  it("uses the ratio fallback when the target has fewer headings", () => {
    const pos = measureSyncPosition(geometry(1750, 2000, DRIVER));
    expect(pos.headingIndex).toBe(3);
    // Target only has two headings, so ordinal 3 cannot be matched.
    const target = geometry(0, 1000, [100, 400]);
    expect(projectSyncPosition(pos, target)).toBeCloseTo(pos.ratio * 1000);
  });

  it("uses the ratio fallback when the target has no headings", () => {
    const pos = measureSyncPosition(geometry(650, 2000, DRIVER));
    expect(projectSyncPosition(pos, geometry(0, 1000, []))).toBeCloseTo(pos.ratio * 1000);
  });

  it("never returns a position outside the target's scroll range", () => {
    const pos: SyncPosition = { headingIndex: 0, fraction: 1, ratio: 1 };
    const result = projectSyncPosition(pos, geometry(0, 300, [100, 900]));
    expect(result).toBeLessThanOrEqual(300);
    expect(result).toBeGreaterThanOrEqual(0);
  });

  it("returns 0 for a target that cannot scroll", () => {
    const pos = measureSyncPosition(geometry(650, 2000, DRIVER));
    expect(projectSyncPosition(pos, geometry(0, 0, TARGET))).toBe(0);
  });

  it("round-trips a position back onto the same geometry", () => {
    const source = geometry(650, 2000, DRIVER);
    expect(projectSyncPosition(measureSyncPosition(source), source)).toBeCloseTo(650);
  });
});
