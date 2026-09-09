import { describe, it, expect } from "vitest";
import { normalizeDisplayMath } from "./displayMath";

describe("normalizeDisplayMath", () => {
  it("converts single-line $$...$$ display math to block form", () => {
    const input = "$$F(x, y) = \\int_0^{x-y} (x-y-t) f(t)\\, dt$$";
    expect(normalizeDisplayMath(input)).toBe(
      "$$\nF(x, y) = \\int_0^{x-y} (x-y-t) f(t)\\, dt\n$$",
    );
  });

  it("converts $$...$$ embedded in a paragraph to block form", () => {
    const input = "around text $$y=x$$ around text";
    expect(normalizeDisplayMath(input)).toBe("around text \n$$\ny=x\n$$\n around text");
  });

  it("converts multiple single-line $$...$$ expressions", () => {
    const input = "$$a$$ and $$b$$";
    expect(normalizeDisplayMath(input)).toBe("$$\na\n$$\n and \n$$\nb\n$$");
  });

  it("leaves an unpaired $$ untouched", () => {
    const input = "echo $$ > /tmp/pid";
    expect(normalizeDisplayMath(input)).toBe(input);
  });

  it("passes fenced code blocks through unchanged", () => {
    const input = "```bash\necho $$ > /tmp/pid; kill $$\n```";
    expect(normalizeDisplayMath(input)).toBe(input);
  });

  it("passes tilde-fenced code blocks through unchanged", () => {
    const input = "~~~\n$$x$$\n~~~";
    expect(normalizeDisplayMath(input)).toBe(input);
  });

  it("passes everything after an unclosed fence through unchanged", () => {
    const input = "```\n$$a$$\ntext $$b$$\n";
    expect(normalizeDisplayMath(input)).toBe(input);
  });

  it("converts math again after a fence closes", () => {
    const input = "```\n$$in code$$\n```\n$$F(x)$$\n";
    expect(normalizeDisplayMath(input)).toBe("```\n$$in code$$\n```\n$$\nF(x)\n$$\n");
  });

  it("passes multi-line block math through unchanged", () => {
    const input = "$$\n\\int_{-\\infty}^{\\infty} e^{-x^2} dx = \\sqrt{\\pi}\n$$";
    expect(normalizeDisplayMath(input)).toBe(input);
  });

  it("passes text without display math through unchanged", () => {
    const input = "Just plain text with $math$ already.";
    expect(normalizeDisplayMath(input)).toBe(input);
  });
});
