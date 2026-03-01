import { describe, it, expect } from "vitest";
import { resolveLink, resolveImageSrc, extractLanguage } from "./resolve";

describe("resolveLink", () => {
  it("returns external for undefined href", () => {
    expect(resolveLink(undefined, 1)).toEqual({ type: "external" });
  });

  it("returns external for http:// URLs", () => {
    expect(resolveLink("http://example.com", 1)).toEqual({ type: "external" });
  });

  it("returns external for https:// URLs", () => {
    expect(resolveLink("https://example.com/page", 1)).toEqual({ type: "external" });
  });

  it("returns hash for anchor-only links", () => {
    expect(resolveLink("#section", 1)).toEqual({ type: "hash" });
  });

  it("returns markdown for .md links", () => {
    expect(resolveLink("other.md", 5)).toEqual({
      type: "markdown",
      hrefPath: "other.md",
    });
  });

  it("strips anchor from markdown links", () => {
    expect(resolveLink("readme.md#title", 5)).toEqual({
      type: "markdown",
      hrefPath: "readme.md",
    });
  });

  it("returns markdown for nested path .md links", () => {
    expect(resolveLink("docs/guide.md", 3)).toEqual({
      type: "markdown",
      hrefPath: "docs/guide.md",
    });
  });

  it("returns file for links with non-md extensions", () => {
    expect(resolveLink("image.png", 7)).toEqual({
      type: "file",
      rawUrl: "/_/api/files/7/raw/image.png",
    });
  });

  it("returns file and preserves anchor in rawUrl", () => {
    expect(resolveLink("data.csv#sheet1", 2)).toEqual({
      type: "file",
      rawUrl: "/_/api/files/2/raw/data.csv#sheet1",
    });
  });

  it("returns file for nested paths with extensions", () => {
    expect(resolveLink("assets/logo.svg", 4)).toEqual({
      type: "file",
      rawUrl: "/_/api/files/4/raw/assets/logo.svg",
    });
  });

  it("returns passthrough for extensionless paths", () => {
    expect(resolveLink("somedir", 1)).toEqual({ type: "passthrough" });
  });

  it("returns passthrough for directory-like paths", () => {
    expect(resolveLink("path/to/dir", 1)).toEqual({ type: "passthrough" });
  });
});

describe("resolveImageSrc", () => {
  it("rewrites relative src to raw API URL", () => {
    expect(resolveImageSrc("image.png", 3)).toBe("/_/api/files/3/raw/image.png");
  });

  it("rewrites nested relative src", () => {
    expect(resolveImageSrc("assets/photo.jpg", 5)).toBe(
      "/_/api/files/5/raw/assets/photo.jpg",
    );
  });

  it("passes through http:// URLs", () => {
    expect(resolveImageSrc("http://example.com/img.png", 1)).toBe(
      "http://example.com/img.png",
    );
  });

  it("passes through https:// URLs", () => {
    expect(resolveImageSrc("https://example.com/img.png", 1)).toBe(
      "https://example.com/img.png",
    );
  });

  it("returns undefined for undefined src", () => {
    expect(resolveImageSrc(undefined, 1)).toBeUndefined();
  });
});

describe("extractLanguage", () => {
  it("extracts language from className", () => {
    expect(extractLanguage("language-typescript")).toBe("typescript");
  });

  it("extracts language with other classes present", () => {
    expect(extractLanguage("foo language-python bar")).toBe("python");
  });

  it("returns null for undefined className", () => {
    expect(extractLanguage(undefined)).toBeNull();
  });

  it("returns null for empty className", () => {
    expect(extractLanguage("")).toBeNull();
  });

  it("returns null when no language- prefix", () => {
    expect(extractLanguage("highlight code")).toBeNull();
  });
});
