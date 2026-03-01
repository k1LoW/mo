import { describe, it, expect } from "vitest";
import { createElement } from "react";
import { extractText } from "./extractText";

describe("extractText", () => {
  it("returns string as-is", () => {
    expect(extractText("hello")).toBe("hello");
  });

  it("returns empty string for empty string", () => {
    expect(extractText("")).toBe("");
  });

  it("converts number to string", () => {
    expect(extractText(42)).toBe("42");
  });

  it("converts zero to string", () => {
    expect(extractText(0)).toBe("0");
  });

  it("joins array elements", () => {
    expect(extractText(["hello", " ", "world"])).toBe("hello world");
  });

  it("handles mixed array with numbers and strings", () => {
    expect(extractText(["chapter ", 3])).toBe("chapter 3");
  });

  it("returns empty string for null", () => {
    expect(extractText(null)).toBe("");
  });

  it("returns empty string for undefined", () => {
    expect(extractText(undefined)).toBe("");
  });

  it("returns empty string for boolean true", () => {
    expect(extractText(true)).toBe("");
  });

  it("returns empty string for boolean false", () => {
    expect(extractText(false)).toBe("");
  });

  it("extracts text from ReactElement", () => {
    const node = createElement("span", null, "inner text");
    expect(extractText(node)).toBe("inner text");
  });

  it("extracts text from nested ReactElements", () => {
    const node = createElement("span", null, createElement("strong", null, "deeply nested"));
    expect(extractText(node)).toBe("deeply nested");
  });

  it("extracts text from ReactElement with mixed children", () => {
    const node = createElement(
      "span",
      null,
      "prefix ",
      createElement("strong", null, "bold"),
      " suffix",
    );
    expect(extractText(node)).toBe("prefix bold suffix");
  });

  it("handles ReactElement with no children", () => {
    const node = createElement("br");
    expect(extractText(node)).toBe("");
  });
});
