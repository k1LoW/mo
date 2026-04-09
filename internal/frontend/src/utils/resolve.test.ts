import { describe, it, expect } from "vitest";
import { resolveLink, resolveImageSrc, extractLanguage } from "./resolve";

describe("resolveLink", () => {
  it("returns external for undefined href", () => {
    expect(resolveLink(undefined, "default", "a")).toEqual({ type: "external" });
  });

  it("returns external for http:// URLs", () => {
    expect(resolveLink("http://example.com", "default", "a")).toEqual({ type: "external" });
  });

  it("returns external for https:// URLs", () => {
    expect(resolveLink("https://example.com/page", "default", "a")).toEqual({ type: "external" });
  });

  it("returns hash for anchor-only links", () => {
    expect(resolveLink("#section", "default", "a")).toEqual({ type: "hash" });
  });

  it("returns markdown for .md links", () => {
    expect(resolveLink("other.md", "default", "e")).toEqual({
      type: "markdown",
      hrefPath: "other.md",
    });
  });

  it("strips anchor from markdown links", () => {
    expect(resolveLink("readme.md#title", "default", "e")).toEqual({
      type: "markdown",
      hrefPath: "readme.md",
    });
  });

  it("returns markdown for nested path .md links", () => {
    expect(resolveLink("docs/guide.md", "default", "c")).toEqual({
      type: "markdown",
      hrefPath: "docs/guide.md",
    });
  });

  it("returns markdown for .mdx links", () => {
    expect(resolveLink("component.mdx", "default", "e")).toEqual({
      type: "markdown",
      hrefPath: "component.mdx",
    });
  });

  it("returns markdown for nested path .mdx links", () => {
    expect(resolveLink("docs/intro.mdx", "default", "c")).toEqual({
      type: "markdown",
      hrefPath: "docs/intro.mdx",
    });
  });

  it("strips anchor from .mdx links", () => {
    expect(resolveLink("page.mdx#section", "default", "e")).toEqual({
      type: "markdown",
      hrefPath: "page.mdx",
    });
  });

  it("returns file for links with non-md extensions", () => {
    expect(resolveLink("image.png", "default", "g")).toEqual({
      type: "file",
      rawUrl: "/_/api/groups/default/files/g/raw/image.png",
    });
  });

  it("returns file and preserves anchor in rawUrl", () => {
    expect(resolveLink("data.csv#sheet1", "default", "b")).toEqual({
      type: "file",
      rawUrl: "/_/api/groups/default/files/b/raw/data.csv#sheet1",
    });
  });

  it("returns file for nested paths with extensions", () => {
    expect(resolveLink("assets/logo.svg", "default", "d")).toEqual({
      type: "file",
      rawUrl: "/_/api/groups/default/files/d/raw/assets/logo.svg",
    });
  });

  it("returns passthrough for extensionless paths", () => {
    expect(resolveLink("somedir", "default", "a")).toEqual({ type: "passthrough" });
  });

  it("returns passthrough for directory-like paths", () => {
    expect(resolveLink("path/to/dir", "default", "a")).toEqual({ type: "passthrough" });
  });

  it("encodes group name in URL", () => {
    expect(resolveLink("image.png", "api/docs", "g")).toEqual({
      type: "file",
      rawUrl: "/_/api/groups/api%2Fdocs/files/g/raw/image.png",
    });
  });
});

describe("resolveImageSrc", () => {
  it("rewrites relative src to raw API URL", () => {
    expect(resolveImageSrc("image.png", "default", "c")).toBe(
      "/_/api/groups/default/files/c/raw/image.png",
    );
  });

  it("rewrites nested relative src", () => {
    expect(resolveImageSrc("assets/photo.jpg", "default", "e")).toBe(
      "/_/api/groups/default/files/e/raw/assets/photo.jpg",
    );
  });

  it("passes through http:// URLs", () => {
    expect(resolveImageSrc("http://example.com/img.png", "default", "a")).toBe(
      "http://example.com/img.png",
    );
  });

  it("passes through https:// URLs", () => {
    expect(resolveImageSrc("https://example.com/img.png", "default", "a")).toBe(
      "https://example.com/img.png",
    );
  });

  it("returns undefined for undefined src", () => {
    expect(resolveImageSrc(undefined, "default", "a")).toBeUndefined();
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
