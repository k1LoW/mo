import { describe, it, expect } from "vitest";
import { allFileIds, parseGroupFromPath } from "../utils/groups";
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
          { id: 1, name: "a.md", path: "/a.md" },
          { id: 2, name: "b.md", path: "/b.md" },
        ],
      },
    ];
    expect(allFileIds(groups)).toEqual(new Set([1, 2]));
  });

  it("collects IDs from multiple groups", () => {
    const groups: Group[] = [
      {
        name: "default",
        files: [{ id: 1, name: "a.md", path: "/a.md" }],
      },
      {
        name: "docs",
        files: [
          { id: 2, name: "b.md", path: "/b.md" },
          { id: 3, name: "c.md", path: "/c.md" },
        ],
      },
    ];
    expect(allFileIds(groups)).toEqual(new Set([1, 2, 3]));
  });

  it("handles groups with no files", () => {
    const groups: Group[] = [
      { name: "empty", files: [] },
      {
        name: "notempty",
        files: [{ id: 5, name: "e.md", path: "/e.md" }],
      },
    ];
    expect(allFileIds(groups)).toEqual(new Set([5]));
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
