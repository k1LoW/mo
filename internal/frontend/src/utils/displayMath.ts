/**
 * Expand single-line `$$...$$` display math into the multi-line block form
 * (`$$\n...\n$$`) that remark-math treats as flow (block) math.
 *
 * micromark-extension-math parses `$$x$$` on a single line as inline math,
 * while Pandoc, MathJax, and GitHub render it as display math. Normalizing
 * here keeps such documents rendering as block math in mo.
 *
 * Fenced code blocks are tracked line-by-line and passed through unchanged,
 * the same approach as stripMdxSyntax in ./mdx.
 */
export function normalizeDisplayMath(md: string): string {
  const lines = md.split("\n");
  const result: string[] = [];
  let fenceChar = "";
  let fenceLen = 0;

  for (const line of lines) {
    const fenceMatch = /^(`{3,}|~{3,})/.exec(line);
    if (fenceMatch) {
      const char = fenceMatch[1][0];
      const len = fenceMatch[1].length;
      if (fenceChar) {
        if (char === fenceChar && len >= fenceLen) {
          fenceChar = "";
          fenceLen = 0;
        }
      } else {
        fenceChar = char;
        fenceLen = len;
      }
      result.push(line);
      continue;
    }

    if (fenceChar) {
      result.push(line);
      continue;
    }

    result.push(expandLineDisplayMath(line));
  }

  return result.join("\n");
}

// Expand each `$$...$$` pair on the line to the multi-line block form. The
// regex cannot cross line boundaries, so an unpaired `$$` is left as-is.
function expandLineDisplayMath(line: string): string {
  return line.replace(/\$\$([^\n]*?)\$\$/g, (match, inner: string, offset: number) => {
    const before = line[offset - 1];
    const after = line[offset + match.length];
    const prefix = before != null ? "\n" : "";
    const suffix = after != null ? "\n" : "";
    return `${prefix}$$\n${inner}\n$$${suffix}`;
  });
}
