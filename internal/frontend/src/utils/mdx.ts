// Track unclosed brackets/braces/parens to detect multi-line statements
function countUnclosed(line: string): number {
  let depth = 0;
  for (const ch of line) {
    if (ch === "{" || ch === "(" || ch === "[") depth++;
    else if (ch === "}" || ch === ")" || ch === "]") depth--;
  }
  return depth;
}

export function stripMdxSyntax(content: string): string {
  // Short-circuit: skip processing if no MDX-like patterns exist
  if (!/^(import|export)[\s{*]/m.test(content) && !/<[A-Z]/.test(content)) {
    return content;
  }

  const lines = content.split("\n");
  const result: string[] = [];
  let fenceChar = "";
  let fenceLen = 0;
  let strippingDepth = 0;

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

    // Continue stripping multi-line import/export
    if (strippingDepth > 0) {
      strippingDepth += countUnclosed(line);
      continue;
    }

    // Remove import/export lines (must start at beginning of line)
    if (/^import[\s{*]/.test(line) || /^export[\s{*]/.test(line)) {
      strippingDepth = countUnclosed(line);
      continue;
    }

    // Escape JSX tags (uppercase first letter) to prevent rehype-raw from processing them
    const escaped = line.replace(
      /<(\/?)([A-Z][A-Za-z0-9.]*)/g,
      "&lt;$1$2",
    );
    result.push(escaped);
  }

  return result.join("\n");
}
