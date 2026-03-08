export interface FileEntry {
  name: string;
  id: string;
  path: string;
  uploaded?: boolean;
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

export async function fetchFileContent(id: string): Promise<FileContent> {
  const res = await fetch(`/_/api/files/${id}/content`);
  if (!res.ok) throw new Error("Failed to fetch file content");
  return res.json();
}

export async function openRelativeFile(
  fileId: string,
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

export async function removeFile(id: string): Promise<void> {
  const res = await fetch(`/_/api/files/${id}`, { method: "DELETE" });
  if (!res.ok) throw new Error("Failed to remove file");
}

export async function reorderFiles(
  groupName: string,
  fileIds: string[],
): Promise<void> {
  const res = await fetch("/_/api/reorder", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ group: groupName, fileIds }),
  });
  if (!res.ok) throw new Error("Failed to reorder files");
}

export async function moveFile(id: string, group: string): Promise<void> {
  const res = await fetch(`/_/api/files/${id}/group`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ group }),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text.trim() || "Failed to move file");
  }
}

export async function uploadFile(name: string, content: string, group: string): Promise<void> {
  const res = await fetch("/_/api/files/upload", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, content, group }),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text.trim() || "Failed to upload file");
  }
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
