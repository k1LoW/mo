import { describe, it, expect } from "vitest";
import { isMarkdownFile, detectLanguage } from "./filetype";

describe("isMarkdownFile", () => {
  it("returns true for .md files", () => {
    expect(isMarkdownFile("README.md")).toBe(true);
  });

  it("returns true for .mdx files", () => {
    expect(isMarkdownFile("component.mdx")).toBe(true);
  });

  it("returns true for .markdown files", () => {
    expect(isMarkdownFile("notes.markdown")).toBe(true);
  });

  it("is case-insensitive", () => {
    expect(isMarkdownFile("README.MD")).toBe(true);
    expect(isMarkdownFile("file.Mdx")).toBe(true);
  });

  it("returns false for non-markdown files", () => {
    expect(isMarkdownFile("main.go")).toBe(false);
    expect(isMarkdownFile("index.ts")).toBe(false);
    expect(isMarkdownFile("config.json")).toBe(false);
    expect(isMarkdownFile("notes.txt")).toBe(false);
  });

  it("returns false for files without extension", () => {
    expect(isMarkdownFile("Makefile")).toBe(false);
  });
});

describe("detectLanguage", () => {
  it("maps common extensions to languages", () => {
    expect(detectLanguage("main.go")).toBe("go");
    expect(detectLanguage("index.ts")).toBe("typescript");
    expect(detectLanguage("app.tsx")).toBe("tsx");
    expect(detectLanguage("style.css")).toBe("css");
    expect(detectLanguage("data.json")).toBe("json");
    expect(detectLanguage("config.yaml")).toBe("yaml");
    expect(detectLanguage("script.py")).toBe("python");
    expect(detectLanguage("lib.rs")).toBe("rust");
    expect(detectLanguage("run.sh")).toBe("bash");
  });

  it("handles special filenames", () => {
    expect(detectLanguage("Dockerfile")).toBe("dockerfile");
    expect(detectLanguage("Dockerfile.prod")).toBe("dockerfile");
    expect(detectLanguage("Makefile")).toBe("makefile");
  });

  it("returns text for unknown extensions", () => {
    expect(detectLanguage("file.xyz")).toBe("text");
    expect(detectLanguage("data.dat")).toBe("text");
  });

  it("handles paths with directories", () => {
    expect(detectLanguage("src/main.go")).toBe("go");
  });
});
