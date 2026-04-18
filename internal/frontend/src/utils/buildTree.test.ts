import { describe, it, expect } from "vitest";
import type { FileEntry } from "../hooks/useApi";
import { buildTree } from "./buildTree";

function makeFile(id: string, path: string): FileEntry {
  const name = path.split("/").pop()!;
  return { id, name, path };
}

function makeUploadedFile(id: string, name: string): FileEntry {
  return { id, name, path: "", uploaded: true };
}

describe("buildTree", () => {
  it("builds tree from multiple directories", () => {
    const files = [
      makeFile("1", "/home/user/docs/a.md"),
      makeFile("2", "/home/user/docs/sub/b.md"),
      makeFile("3", "/home/user/docs/other/c.md"),
    ];
    const root = buildTree(files);

    // root should have dirs first (other, sub) then file (a.md)
    expect(root.children.length).toBe(3);
    expect(root.children[0].name).toBe("other");
    expect(root.children[0].file).toBeNull();
    expect(root.children[0].children[0].file?.id).toBe("3");
    expect(root.children[1].name).toBe("sub");
    expect(root.children[1].children[0].file?.id).toBe("2");
    expect(root.children[2].name).toBe("a.md");
    expect(root.children[2].file?.id).toBe("1");
  });

  it("handles single file", () => {
    const files = [makeFile("1", "/home/user/docs/readme.md")];
    const root = buildTree(files);

    expect(root.children.length).toBe(1);
    expect(root.children[0].name).toBe("readme.md");
    expect(root.children[0].file?.id).toBe("1");
  });

  it("handles all files in same directory", () => {
    const files = [
      makeFile("1", "/docs/a.md"),
      makeFile("2", "/docs/b.md"),
      makeFile("3", "/docs/c.md"),
    ];
    const root = buildTree(files);

    expect(root.children.length).toBe(3);
    expect(root.children.map((c) => c.name)).toEqual(["a.md", "b.md", "c.md"]);
    expect(root.children.every((c) => c.file != null)).toBe(true);
  });

  it("collapses single-child directory chains", () => {
    const files = [
      makeFile("1", "/home/user/project/src/components/App.tsx"),
      makeFile("2", "/home/user/project/src/components/Button.tsx"),
      makeFile("3", "/home/user/project/src/utils/helpers.ts"),
    ];
    const root = buildTree(files);

    // Common prefix is /home/user/project/src
    // children should be: components (dir), utils (dir)
    expect(root.children.length).toBe(2);
    expect(root.children[0].name).toBe("components");
    expect(root.children[0].children.length).toBe(2);
    expect(root.children[1].name).toBe("utils");
    expect(root.children[1].children.length).toBe(1);
  });

  it("collapses deeply nested single-child directories", () => {
    const files = [makeFile("1", "/root/a/b/c/file.md"), makeFile("2", "/root/x/file2.md")];
    const root = buildTree(files);

    // Common prefix is /root
    // "a" -> "b" -> "c" should collapse to "a/b/c"
    expect(root.children.length).toBe(2);
    const collapsed = root.children.find((c) => c.name.startsWith("a"));
    expect(collapsed?.name).toBe("a/b/c");
    expect(collapsed?.children[0].file?.id).toBe("1");
  });

  it("returns empty root for no files", () => {
    const root = buildTree([]);
    expect(root.children.length).toBe(0);
  });

  it("sorts directories before files at each level", () => {
    const files = [
      makeFile("1", "/proj/z-file.md"),
      makeFile("2", "/proj/a-dir/nested.md"),
      makeFile("3", "/proj/a-file.md"),
    ];
    const root = buildTree(files);

    expect(root.children[0].name).toBe("a-dir");
    expect(root.children[0].file).toBeNull();
    expect(root.children[1].name).toBe("a-file.md");
    expect(root.children[2].name).toBe("z-file.md");
  });

  it("handles uploaded files (empty path) at root level", () => {
    const files = [makeUploadedFile("1", "uploaded.md"), makeUploadedFile("2", "another.md")];
    const root = buildTree(files);

    expect(root.children.length).toBe(2);
    expect(root.children[0].name).toBe("another.md");
    expect(root.children[0].file?.id).toBe("2");
    expect(root.children[1].name).toBe("uploaded.md");
    expect(root.children[1].file?.id).toBe("1");
  });

  it("sorts sibling files by natural order (i1, i2, ..., i10)", () => {
    const files = [
      makeFile("1", "/docs/i1.md"),
      makeFile("2", "/docs/i10.md"),
      makeFile("3", "/docs/i2.md"),
      makeFile("4", "/docs/i11.md"),
      makeFile("5", "/docs/i13.md"),
      makeFile("6", "/docs/i3.md"),
    ];
    const root = buildTree(files);

    expect(root.children.map((c) => c.name)).toEqual([
      "i1.md",
      "i2.md",
      "i3.md",
      "i10.md",
      "i11.md",
      "i13.md",
    ]);
  });

  it("mixes filesystem and uploaded files", () => {
    const files = [
      makeFile("1", "/docs/a.md"),
      makeFile("2", "/docs/sub/b.md"),
      makeUploadedFile("3", "dropped.md"),
    ];
    const root = buildTree(files);

    // Should have: sub (dir), a.md (file), dropped.md (uploaded at root)
    expect(root.children.length).toBe(3);
    expect(root.children[0].name).toBe("sub");
    expect(root.children[0].file).toBeNull();
    expect(root.children[1].name).toBe("a.md");
    expect(root.children[2].name).toBe("dropped.md");
    expect(root.children[2].file?.id).toBe("3");
  });
});
