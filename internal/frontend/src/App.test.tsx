import { describe, it, expect, beforeEach, afterEach } from "vitest";
import {
  FONT_SIZE_STORAGE_KEY,
  formatTitle,
  getInitialFontSize,
  getInitialTocOpenMap,
  isTocOpenForFile,
  TOC_OPEN_STORAGE_KEY,
} from "./App";

describe("getInitialFontSize", () => {
  beforeEach(() => localStorage.clear());
  afterEach(() => localStorage.clear());

  it("returns medium when localStorage is empty", () => {
    expect(getInitialFontSize()).toBe("medium");
  });

  it("returns stored size", () => {
    localStorage.setItem(FONT_SIZE_STORAGE_KEY, "xlarge");
    expect(getInitialFontSize()).toBe("xlarge");
  });

  it("returns medium for invalid value", () => {
    localStorage.setItem(FONT_SIZE_STORAGE_KEY, "huge");
    expect(getInitialFontSize()).toBe("medium");
  });
});

describe("getInitialTocOpenMap", () => {
  beforeEach(() => localStorage.clear());
  afterEach(() => localStorage.clear());

  it("returns empty object when localStorage is empty", () => {
    expect(getInitialTocOpenMap()).toEqual({});
  });

  it("returns stored map", () => {
    localStorage.setItem(TOC_OPEN_STORAGE_KEY, JSON.stringify({ abc123: true, def456: false }));
    expect(getInitialTocOpenMap()).toEqual({ abc123: true, def456: false });
  });

  it("returns empty object for invalid JSON", () => {
    localStorage.setItem(TOC_OPEN_STORAGE_KEY, "not-json");
    expect(getInitialTocOpenMap()).toEqual({});
  });

  it("returns empty object when stored JSON is null", () => {
    localStorage.setItem(TOC_OPEN_STORAGE_KEY, "null");
    expect(getInitialTocOpenMap()).toEqual({});
  });

  it("returns empty object when stored JSON is an array", () => {
    localStorage.setItem(TOC_OPEN_STORAGE_KEY, "[]");
    expect(getInitialTocOpenMap()).toEqual({});
  });
});

describe("isTocOpenForFile", () => {
  it("returns false when fileId is null", () => {
    expect(isTocOpenForFile({ abc: true }, null, "")).toBe(false);
  });

  it("returns false for non-markdown file even if map says true", () => {
    expect(isTocOpenForFile({ abc: true }, "abc", "image.png")).toBe(false);
  });

  it("returns true when map has true for the file", () => {
    expect(isTocOpenForFile({ abc: true }, "abc", "readme.md")).toBe(true);
  });

  it("returns false when map has no entry for the file", () => {
    expect(isTocOpenForFile({}, "abc", "readme.md")).toBe(false);
  });

  it("returns false when map has false for the file", () => {
    expect(isTocOpenForFile({ abc: false }, "abc", "readme.md")).toBe(false);
  });
});

describe("formatTitle", () => {
  it("returns `mo` when fileEntry is undefined", () => {
    expect(formatTitle(undefined)).toBe("mo");
  });

  it("returns `file name` when title is undefined", () => {
    expect(formatTitle({ name: "file.md", title: undefined })).toBe("file.md | mo");
  });

  it("returns `title - file name` when title is defined", () => {
    expect(formatTitle({ name: "file.md", title: "File Title" })).toBe("File Title - file.md | mo");
  });
});
