import type { Group } from "../hooks/useApi";

export function allFileIds(groups: Group[]): Set<string> {
  const ids = new Set<string>();
  for (const g of groups) {
    for (const f of g.files) {
      ids.add(f.id);
    }
  }
  return ids;
}

export function parseGroupFromPath(pathname: string): string {
  const path = pathname.replace(/^\//, "").replace(/\/$/, "");
  return path || "default";
}

export function groupToPath(groupName: string): string {
  return groupName === "default" ? "/" : `/${groupName}`;
}

export function buildFileUrl(groupName: string, fileId: string): string {
  return `${groupToPath(groupName)}?file=${fileId}`;
}

export function parseFileIdFromSearch(search: string): string | null {
  const params = new URLSearchParams(search);
  const raw = params.get("file");
  if (raw == null || raw === "") return null;
  return raw;
}

export interface RelativeOpenRequest {
  // ID of the source file the relative link was rendered in.
  from: string;
  // Relative path to resolve against the source file, as written in the link.
  open: string;
}

// A relative Markdown link has no file ID until the server resolves it, so a new
// browser tab cannot be pointed straight at buildFileUrl. This URL carries the
// source file and the relative path instead, and the SPA resolves it on load.
export function buildRelativeOpenUrl(
  groupName: string,
  sourceFileId: string,
  relativePath: string,
): string {
  const params = new URLSearchParams({ from: sourceFileId, open: relativePath });
  return `${groupToPath(groupName)}?${params.toString()}`;
}

export function parseRelativeOpenFromSearch(search: string): RelativeOpenRequest | null {
  const params = new URLSearchParams(search);
  const from = params.get("from");
  const open = params.get("open");
  if (!from || !open) return null;
  return { from, open };
}
