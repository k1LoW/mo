export interface FileEntry {
  name: string;
  id: number;
  path: string;
}

export interface Group {
  name: string;
  files: FileEntry[];
}

export interface FileContent {
  content: string;
  baseDir: string;
}

export interface VersionInfo {
  version: string;
  revision: string;
}

export async function fetchGroups(): Promise<Group[]> {
  const res = await fetch("/_/api/groups");
  if (!res.ok) throw new Error("Failed to fetch groups");
  return res.json();
}

export async function fetchFileContent(id: number): Promise<FileContent> {
  const res = await fetch(`/_/api/files/${id}/content`);
  if (!res.ok) throw new Error("Failed to fetch file content");
  return res.json();
}

export async function openRelativeFile(
  fileId: number,
  relativePath: string,
): Promise<FileEntry> {
  const res = await fetch("/_/api/files/open", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ fileId, path: relativePath }),
  });
  if (!res.ok) throw new Error("Failed to open file");
  return res.json();
}

export async function removeFile(id: number): Promise<void> {
  const res = await fetch(`/_/api/files/${id}`, { method: "DELETE" });
  if (!res.ok) throw new Error("Failed to remove file");
}

export async function restartServer(): Promise<void> {
  const res = await fetch("/_/api/restart", { method: "POST" });
  if (!res.ok) throw new Error("Failed to restart server");
}

export async function fetchVersion(): Promise<VersionInfo> {
  const res = await fetch("/_/api/version");
  if (!res.ok) throw new Error("Failed to fetch version");
  return res.json();
}
