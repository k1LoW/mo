import type { Group } from "../hooks/useApi";

export function allFileIds(groups: Group[]): Set<number> {
  const ids = new Set<number>();
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
