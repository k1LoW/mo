import { describe, it, expect } from "vitest";
import { parseFrontmatter } from "./frontmatter";

describe("parseFrontmatter", () => {
  it("extracts frontmatter and content", () => {
    const input = `---
title: Hello
date: 2024-01-01
---
# Hello World`;
    const result = parseFrontmatter(input);
    expect(result).toEqual({
      yaml: "title: Hello\ndate: 2024-01-01",
      content: "# Hello World",
    });
  });

  it("returns null when no frontmatter", () => {
    expect(parseFrontmatter("# Just a heading")).toBeNull();
  });

  it("returns null when --- is not at the start", () => {
    expect(parseFrontmatter("some text\n---\ntitle: x\n---\n")).toBeNull();
  });

  it("handles empty frontmatter", () => {
    const input = `---

---
Content here`;
    const result = parseFrontmatter(input);
    expect(result).toEqual({
      yaml: "",
      content: "Content here",
    });
  });

  it("handles multiline YAML values", () => {
    const input = `---
title: Hello
tags:
  - one
  - two
description: |
  A long
  description
---
# Body`;
    const result = parseFrontmatter(input);
    expect(result).not.toBeNull();
    expect(result!.yaml).toContain("tags:");
    expect(result!.yaml).toContain("  - one");
    expect(result!.content).toBe("# Body");
  });

  it("handles content with --- inside markdown", () => {
    const input = `---
title: Test
---
# Heading

Some text with --- inside it`;
    const result = parseFrontmatter(input);
    expect(result).not.toBeNull();
    expect(result!.yaml).toBe("title: Test");
    expect(result!.content).toContain("--- inside it");
  });

  it("handles CRLF line endings", () => {
    const input = "---\r\ntitle: Hello\r\n---\r\n# Content";
    const result = parseFrontmatter(input);
    expect(result).not.toBeNull();
    expect(result!.yaml).toBe("title: Hello");
    expect(result!.content).toBe("# Content");
  });

  it("handles trailing newline after closing ---", () => {
    const input = `---
title: Test
---

# Content`;
    const result = parseFrontmatter(input);
    expect(result).toEqual({
      yaml: "title: Test",
      content: "\n# Content",
    });
  });
});
