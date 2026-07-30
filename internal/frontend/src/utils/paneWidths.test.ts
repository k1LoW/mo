import { afterEach, beforeEach, describe, expect, it } from "vitest";
import {
  equalWeights,
  MIN_PANE_WIDTH,
  PANE_WIDTHS_STORAGE_KEY,
  readStoredWeights,
  resizeWeights,
} from "./paneWidths";

describe("equalWeights", () => {
  it("gives every column the same share", () => {
    expect(equalWeights(3)).toEqual([1, 1, 1]);
    expect(equalWeights(0)).toEqual([]);
  });
});

describe("readStoredWeights", () => {
  beforeEach(() => localStorage.clear());
  afterEach(() => localStorage.clear());

  it("returns equal weights when nothing is stored", () => {
    expect(readStoredWeights(2)).toEqual([1, 1]);
  });

  it("returns an empty array for no panes", () => {
    expect(readStoredWeights(0)).toEqual([]);
  });

  it("restores a stored layout for the same column count", () => {
    localStorage.setItem(PANE_WIDTHS_STORAGE_KEY, JSON.stringify([300, 700, 500]));
    expect(readStoredWeights(3)).toEqual([300, 700, 500]);
  });

  it("ignores a stored layout for a different column count", () => {
    localStorage.setItem(PANE_WIDTHS_STORAGE_KEY, JSON.stringify([300, 700, 500]));
    expect(readStoredWeights(2)).toEqual([1, 1]);
  });

  it("ignores malformed or non-positive stored values", () => {
    localStorage.setItem(PANE_WIDTHS_STORAGE_KEY, "not-json");
    expect(readStoredWeights(2)).toEqual([1, 1]);

    localStorage.setItem(PANE_WIDTHS_STORAGE_KEY, JSON.stringify([300, 0]));
    expect(readStoredWeights(2)).toEqual([1, 1]);

    localStorage.setItem(PANE_WIDTHS_STORAGE_KEY, JSON.stringify([300, "wide"]));
    expect(readStoredWeights(2)).toEqual([1, 1]);

    localStorage.setItem(PANE_WIDTHS_STORAGE_KEY, JSON.stringify({ a: 1 }));
    expect(readStoredWeights(2)).toEqual([1, 1]);
  });
});

describe("resizeWeights", () => {
  it("moves space from the right column to the left one", () => {
    expect(resizeWeights([600, 600], 1, 100)).toEqual([700, 500]);
  });

  it("moves space the other way for a negative delta", () => {
    expect(resizeWeights([600, 600], 1, -100)).toEqual([500, 700]);
  });

  it("leaves every other column untouched", () => {
    expect(resizeWeights([400, 400, 400], 2, 50)).toEqual([400, 450, 350]);
  });

  it("stops at the minimum width instead of collapsing a column", () => {
    const [left, right] = resizeWeights([600, 600], 1, 10_000);
    expect(right).toBe(MIN_PANE_WIDTH);
    expect(left).toBe(1200 - MIN_PANE_WIDTH);
  });

  it("stops at the minimum width when dragging left too", () => {
    expect(resizeWeights([600, 600], 1, -10_000)[0]).toBe(MIN_PANE_WIDTH);
  });

  it("does nothing when the pair cannot fit both minimums", () => {
    const cramped = [MIN_PANE_WIDTH, MIN_PANE_WIDTH - 20];
    expect(resizeWeights(cramped, 1, 40)).toEqual(cramped);
  });

  it("ignores an out-of-range boundary", () => {
    expect(resizeWeights([600, 600], 0, 100)).toEqual([600, 600]);
    expect(resizeWeights([600, 600], 2, 100)).toEqual([600, 600]);
  });

  it("returns a new array rather than mutating the input", () => {
    const widths = [600, 600];
    expect(resizeWeights(widths, 1, 100)).not.toBe(widths);
    expect(widths).toEqual([600, 600]);
  });
});
