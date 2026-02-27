export type LinkResolution =
  | { type: "external" }
  | { type: "hash" }
  | { type: "markdown"; hrefPath: string }
  | { type: "file"; rawUrl: string }
  | { type: "passthrough" };

export function resolveLink(
  href: string | undefined,
  fileId: number,
): LinkResolution {
  if (
    !href ||
    href.startsWith("http://") ||
    href.startsWith("https://")
  ) {
    return { type: "external" };
  }
  if (href.startsWith("#")) {
    return { type: "hash" };
  }
  const hrefPath = href.split("#")[0];
  if (hrefPath.endsWith(".md")) {
    return { type: "markdown", hrefPath };
  }
  const basename = hrefPath.split("/").pop() || "";
  if (basename.includes(".")) {
    return { type: "file", rawUrl: `/_/api/files/${fileId}/raw/${href}` };
  }
  return { type: "passthrough" };
}

export function resolveImageSrc(
  src: string | undefined,
  fileId: number,
): string | undefined {
  if (src && !src.startsWith("http://") && !src.startsWith("https://")) {
    return `/_/api/files/${fileId}/raw/${src}`;
  }
  return src;
}

export function extractLanguage(
  className: string | undefined,
): string | null {
  const match = /language-(\w+)/.exec(className || "");
  return match ? match[1] : null;
}
