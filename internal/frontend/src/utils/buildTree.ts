import type { FileEntry } from "../hooks/useApi";

export interface TreeNode {
  name: string;
  fullPath: string;
  children: TreeNode[];
  file: FileEntry | null;
}

export function buildTree(files: FileEntry[]): TreeNode {
  if (files.length === 0) {
    return { name: "", fullPath: "", children: [], file: null };
  }

  // Separate uploaded files from filesystem files
  const fsFiles = files.filter((f) => !f.uploaded);
  const uploadedFiles = files.filter((f) => f.uploaded);

  if (fsFiles.length === 0) {
    // All files are uploaded — flat list at root
    const root: TreeNode = { name: "", fullPath: "", children: [], file: null };
    for (const file of uploadedFiles) {
      root.children.push({
        name: file.name,
        fullPath: `uploaded:${file.id}`,
        children: [],
        file,
      });
    }
    sortTree(root);
    return root;
  }

  // Split each file path into segments once
  const splitPaths = fsFiles.map((f) => f.path.split("/"));
  const dirSegmentsList = splitPaths.map((parts) => parts.slice(0, -1));

  // Find common prefix among directory parts
  const commonPrefix = findCommonPrefix(dirSegmentsList);
  const prefixLen = commonPrefix.length;

  // Build a trie from relative paths
  const root: TreeNode = { name: "", fullPath: "", children: [], file: null };

  for (let fi = 0; fi < fsFiles.length; fi++) {
    const file = fsFiles[fi];
    const parts = splitPaths[fi];
    const dirParts = parts.slice(prefixLen, -1); // relative dir segments
    let current = root;

    for (const segment of dirParts) {
      let child = current.children.find((c) => c.file == null && c.name === segment);
      if (!child) {
        child = {
          name: segment,
          fullPath: current.fullPath ? `${current.fullPath}/${segment}` : segment,
          children: [],
          file: null,
        };
        current.children.push(child);
      }
      current = child;
    }

    current.children.push({
      name: file.name,
      fullPath: current.fullPath ? `${current.fullPath}/${file.name}` : file.name,
      children: [],
      file,
    });
  }

  // Add uploaded files at root level
  for (const file of uploadedFiles) {
    root.children.push({
      name: file.name,
      fullPath: `uploaded:${file.id}`,
      children: [],
      file,
    });
  }

  // Collapse single-child directory nodes
  collapseSingleChild(root);

  // Sort: directories first, then alphabetical
  sortTree(root);

  return root;
}

function findCommonPrefix(segmentsList: string[][]): string[] {
  if (segmentsList.length === 0) return [];
  const first = segmentsList[0];
  let len = first.length;
  for (let i = 1; i < segmentsList.length; i++) {
    len = Math.min(len, segmentsList[i].length);
    for (let j = 0; j < len; j++) {
      if (first[j] !== segmentsList[i][j]) {
        len = j;
        break;
      }
    }
  }
  return first.slice(0, len);
}

function collapseSingleChild(node: TreeNode): void {
  for (let i = 0; i < node.children.length; i++) {
    let child = node.children[i];
    // Collapse chain: directory with exactly one child that is also a directory
    while (child.file == null && child.children.length === 1 && child.children[0].file == null) {
      const grandchild = child.children[0];
      child = {
        name: `${child.name}/${grandchild.name}`,
        fullPath: grandchild.fullPath,
        children: grandchild.children,
        file: null,
      };
    }
    node.children[i] = child;
    collapseSingleChild(child);
  }
}

const naturalCompare = new Intl.Collator(undefined, {
  numeric: true,
  sensitivity: "base",
}).compare;

function sortTree(node: TreeNode): void {
  node.children.sort((a, b) => {
    const aIsDir = a.file == null;
    const bIsDir = b.file == null;
    if (aIsDir !== bIsDir) return aIsDir ? -1 : 1;
    return naturalCompare(a.name, b.name);
  });
  for (const child of node.children) {
    sortTree(child);
  }
}
