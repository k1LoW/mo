import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { getInitialTocOpen, TOC_OPEN_STORAGE_KEY } from "./App";

describe("getInitialTocOpen", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    localStorage.clear();
  });

  it("returns false when localStorage is empty", () => {
    expect(getInitialTocOpen()).toBe(false);
  });

  it("returns true when localStorage has 'true'", () => {
    localStorage.setItem(TOC_OPEN_STORAGE_KEY, "true");
    expect(getInitialTocOpen()).toBe(true);
  });

  it("returns false when localStorage has 'false'", () => {
    localStorage.setItem(TOC_OPEN_STORAGE_KEY, "false");
    expect(getInitialTocOpen()).toBe(false);
  });

  it("returns false for invalid values", () => {
    localStorage.setItem(TOC_OPEN_STORAGE_KEY, "invalid");
    expect(getInitialTocOpen()).toBe(false);
  });

  it("returns false for '1' (strict string matching)", () => {
    localStorage.setItem(TOC_OPEN_STORAGE_KEY, "1");
    expect(getInitialTocOpen()).toBe(false);
  });

  it("returns false for '0' (strict string matching)", () => {
    localStorage.setItem(TOC_OPEN_STORAGE_KEY, "0");
    expect(getInitialTocOpen()).toBe(false);
  });
});
