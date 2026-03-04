import { describe, it, expect } from "vitest";
import { stripMdxSyntax } from "./mdx";

describe("stripMdxSyntax", () => {
  it("removes import lines", () => {
    const input = `import React from "react";\nimport { Button } from "./Button";\n\n# Hello`;
    expect(stripMdxSyntax(input)).toBe("\n# Hello");
  });

  it("removes export lines", () => {
    const input = `export default Layout;\nexport const meta = {};\n\n# Hello`;
    expect(stripMdxSyntax(input)).toBe("\n# Hello");
  });

  it("preserves import/export inside code blocks", () => {
    const input = "```js\nimport React from \"react\";\nexport default App;\n```";
    expect(stripMdxSyntax(input)).toBe(input);
  });

  it("removes import/export without space before brace", () => {
    const input = 'import{foo} from "bar";\nexport{meta};\n\n# Hello';
    expect(stripMdxSyntax(input)).toBe("\n# Hello");
  });

  it("removes import with star", () => {
    const input = 'import* as React from "react";\n\n# Hello';
    expect(stripMdxSyntax(input)).toBe("\n# Hello");
  });

  it("does not match words containing import/export", () => {
    const input = "This is an important note.\nWe exported the data.";
    expect(stripMdxSyntax(input)).toBe(input);
  });

  it("escapes JSX tags (uppercase first letter)", () => {
    const input = "<Button>Click me</Button>";
    expect(stripMdxSyntax(input)).toBe("&lt;Button>Click me&lt;/Button>");
  });

  it("does not escape lowercase HTML tags", () => {
    const input = "<div>content</div>";
    expect(stripMdxSyntax(input)).toBe(input);
  });

  it("escapes self-closing JSX tags", () => {
    const input = "<Component />";
    expect(stripMdxSyntax(input)).toBe("&lt;Component />");
  });

  it("escapes JSX tags with dots (namespaced)", () => {
    const input = "<Layout.Header>content</Layout.Header>";
    expect(stripMdxSyntax(input)).toBe("&lt;Layout.Header>content&lt;/Layout.Header>");
  });

  it("returns content unchanged when no MDX syntax", () => {
    const input = "# Hello\n\nThis is plain markdown.\n\n- item 1\n- item 2";
    expect(stripMdxSyntax(input)).toBe(input);
  });

  it("handles mixed content correctly", () => {
    const input = [
      'import { Card } from "./Card";',
      "",
      "# Title",
      "",
      "<Card>",
      "  Some **bold** text",
      "</Card>",
      "",
      "<div>normal html</div>",
    ].join("\n");
    const expected = [
      "",
      "# Title",
      "",
      "&lt;Card>",
      "  Some **bold** text",
      "&lt;/Card>",
      "",
      "<div>normal html</div>",
    ].join("\n");
    expect(stripMdxSyntax(input)).toBe(expected);
  });

  it("preserves code blocks with tilde fences", () => {
    const input = "~~~\nimport foo from \"bar\";\n~~~";
    expect(stripMdxSyntax(input)).toBe(input);
  });

  it("does not close backtick fence with tildes", () => {
    const input = "```\nimport foo from \"bar\";\n~~~\nimport baz from \"qux\";\n```";
    expect(stripMdxSyntax(input)).toBe(input);
  });

  it("requires closing fence length >= opening fence length", () => {
    const input = "````\nimport foo from \"bar\";\n```\nimport baz from \"qux\";\n````";
    expect(stripMdxSyntax(input)).toBe(input);
  });

  it("removes multi-line import statements", () => {
    const input = [
      "import {",
      "  Button,",
      "  Card",
      '} from "./components";',
      "",
      "# Hello",
    ].join("\n");
    expect(stripMdxSyntax(input)).toBe("\n# Hello");
  });

  it("removes multi-line export statements", () => {
    const input = [
      "export const meta = {",
      '  title: "My Page",',
      '  description: "A description"',
      "};",
      "",
      "# Hello",
    ].join("\n");
    expect(stripMdxSyntax(input)).toBe("\n# Hello");
  });

  it("removes multi-line import with nested brackets", () => {
    const input = [
      "import {",
      "  type Props,",
      "  Component",
      '} from "./mod";',
      "# Content",
    ].join("\n");
    expect(stripMdxSyntax(input)).toBe("# Content");
  });
});
