import { describe, it, expect } from "vitest";
import {
  allFileIds,
  parseGroupFromPath,
  groupToPath,
  buildFileUrl,
  parseFileIdFromSearch,
  buildRelativeOpenUrl,
  parseRelativeOpenFromSearch,
} from "./groups";
import type { Group } from "../hooks/useApi";

describe("allFileIds", () => {
  it("returns empty set for no groups", () => {
    expect(allFileIds([])).toEqual(new Set());
  });

  it("collects IDs from a single group", () => {
    const groups: Group[] = [
      {
        name: "default",
        files: [
          { id: "abc12345", name: "a.md", path: "/a.md" },
          { id: "def67890", name: "b.md", path: "/b.md" },
        ],
      },
    ];
    expect(allFileIds(groups)).toEqual(new Set(["abc12345", "def67890"]));
  });

  it("collects IDs from multiple groups", () => {
    const groups: Group[] = [
      {
        name: "default",
        files: [{ id: "abc12345", name: "a.md", path: "/a.md" }],
      },
      {
        name: "docs",
        files: [
          { id: "def67890", name: "b.md", path: "/b.md" },
          { id: "ghi11111", name: "c.md", path: "/c.md" },
        ],
      },
    ];
    expect(allFileIds(groups)).toEqual(new Set(["abc12345", "def67890", "ghi11111"]));
  });

  it("handles groups with no files", () => {
    const groups: Group[] = [
      { name: "empty", files: [] },
      {
        name: "notempty",
        files: [{ id: "eee55555", name: "e.md", path: "/e.md" }],
      },
    ];
    expect(allFileIds(groups)).toEqual(new Set(["eee55555"]));
  });
});

describe("parseGroupFromPath", () => {
  it("returns 'default' for root path", () => {
    expect(parseGroupFromPath("/")).toBe("default");
  });

  it("returns 'default' for empty string", () => {
    expect(parseGroupFromPath("")).toBe("default");
  });

  it("extracts group name from path", () => {
    expect(parseGroupFromPath("/design")).toBe("design");
  });

  it("strips trailing slash", () => {
    expect(parseGroupFromPath("/docs/")).toBe("docs");
  });

  it("handles path without leading slash", () => {
    expect(parseGroupFromPath("notes")).toBe("notes");
  });
});

describe("groupToPath", () => {
  it("returns / for default group", () => {
    expect(groupToPath("default")).toBe("/");
  });

  it("returns /name for named group", () => {
    expect(groupToPath("design")).toBe("/design");
  });
});

describe("buildFileUrl", () => {
  it("builds URL for default group", () => {
    expect(buildFileUrl("default", "abc12345")).toBe("/?file=abc12345");
  });

  it("builds URL for named group", () => {
    expect(buildFileUrl("design", "def67890")).toBe("/design?file=def67890");
  });
});

describe("parseFileIdFromSearch", () => {
  it("returns null for empty search", () => {
    expect(parseFileIdFromSearch("")).toBeNull();
  });

  it("parses file id from search string", () => {
    expect(parseFileIdFromSearch("?file=abc12345")).toBe("abc12345");
  });

  it("returns null when file param is missing", () => {
    expect(parseFileIdFromSearch("?other=1")).toBeNull();
  });

  it("handles search with multiple params", () => {
    expect(parseFileIdFromSearch("?foo=bar&file=def67890&baz=1")).toBe("def67890");
  });

  it("returns null for empty value", () => {
    expect(parseFileIdFromSearch("?file=")).toBeNull();
  });
});

describe("buildRelativeOpenUrl", () => {
  it("encodes source id and relative path for the default group", () => {
    expect(buildRelativeOpenUrl("default", "abc12345", "next.md")).toBe(
      "/?from=abc12345&open=next.md",
    );
  });

  it("encodes a nested path for a named group", () => {
    expect(buildRelativeOpenUrl("design", "abc12345", "docs/guide.mdx")).toBe(
      "/design?from=abc12345&open=docs%2Fguide.mdx",
    );
  });

  it("round-trips a path with spaces back to the original", () => {
    const url = buildRelativeOpenUrl("default", "abc12345", "a b.md");
    expect(parseRelativeOpenFromSearch(url.slice(url.indexOf("?")))?.open).toBe("a b.md");
  });
});

describe("parseRelativeOpenFromSearch", () => {
  it("returns null for empty search", () => {
    expect(parseRelativeOpenFromSearch("")).toBeNull();
  });

  it("parses from and open params", () => {
    expect(parseRelativeOpenFromSearch("?from=abc12345&open=docs%2Fguide.mdx")).toEqual({
      from: "abc12345",
      open: "docs/guide.mdx",
    });
  });

  it("returns null when from is missing", () => {
    expect(parseRelativeOpenFromSearch("?open=next.md")).toBeNull();
  });

  it("returns null when open is missing", () => {
    expect(parseRelativeOpenFromSearch("?from=abc12345")).toBeNull();
  });
});
