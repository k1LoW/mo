import { useCallback, useEffect, useMemo, useState } from "react";
import type { FileEntry, Group } from "../hooks/useApi";
import { buildTree, type TreeNode } from "../utils/buildTree";
import { buildFileUrl } from "../utils/groups";
import { fileRowClass, handleFileRowClick } from "../utils/fileRow";
import type { PaneTarget } from "../utils/panes";
import { FileContextMenu } from "./FileContextMenu";
import { FileIcon } from "./FileIcon";

const COLLAPSED_STORAGE_KEY = "mo-sidebar-tree-collapsed";

function getInitialCollapsed(group: string): Set<string> {
  try {
    const stored = localStorage.getItem(COLLAPSED_STORAGE_KEY);
    if (stored) {
      const parsed = JSON.parse(stored);
      if (parsed[group]) return new Set(parsed[group]);
    }
  } catch {
    /* ignore */
  }
  return new Set();
}

interface TreeViewProps {
  files: FileEntry[];
  activeGroup: string;
  activeFileId: string | null;
  openFileIds: ReadonlySet<string>;
  showTitle: boolean;
  menuOpenId: string | null;
  otherGroups: Group[];
  onFileSelect: (id: string, target: PaneTarget) => void;
  onMenuToggle: (id: string) => void;
  onOpenInNewTab: (id: string) => void;
  onOpenInNewPane?: (id: string) => void;
  onCopyPath: (path: string) => void;
  onCopyLink: (id: string) => void;
  onMoveToGroup: (id: string, group: string) => void;
  onRemove: (id: string) => void;
  menuRef: React.RefObject<HTMLDivElement | null>;
}

export function TreeView({
  files,
  activeGroup,
  activeFileId,
  openFileIds,
  showTitle,
  menuOpenId,
  otherGroups,
  onFileSelect,
  onMenuToggle,
  onOpenInNewTab,
  onOpenInNewPane,
  onCopyPath,
  onCopyLink,
  onMoveToGroup,
  onRemove,
  menuRef,
}: TreeViewProps) {
  const tree = useMemo(() => buildTree(files), [files]);
  const [prevGroup, setPrevGroup] = useState(activeGroup);
  const [collapsedPaths, setCollapsedPaths] = useState<Set<string>>(() =>
    getInitialCollapsed(activeGroup),
  );

  if (prevGroup !== activeGroup) {
    setPrevGroup(activeGroup);
    setCollapsedPaths(getInitialCollapsed(activeGroup));
  }

  useEffect(() => {
    try {
      const stored = localStorage.getItem(COLLAPSED_STORAGE_KEY);
      const all = stored ? JSON.parse(stored) : {};
      all[activeGroup] = [...collapsedPaths];
      localStorage.setItem(COLLAPSED_STORAGE_KEY, JSON.stringify(all));
    } catch {
      /* ignore */
    }
  }, [collapsedPaths, activeGroup]);

  const handleToggleCollapse = useCallback((path: string) => {
    setCollapsedPaths((prev) => {
      const next = new Set(prev);
      if (next.has(path)) {
        next.delete(path);
      } else {
        next.add(path);
      }
      return next;
    });
  }, []);

  return (
    <>
      {tree.children.map((node) => (
        <TreeNodeItem
          key={node.fullPath}
          node={node}
          depth={0}
          activeGroup={activeGroup}
          activeFileId={activeFileId}
          openFileIds={openFileIds}
          showTitle={showTitle}
          menuOpenId={menuOpenId}
          otherGroups={otherGroups}
          onFileSelect={onFileSelect}
          onMenuToggle={onMenuToggle}
          onOpenInNewTab={onOpenInNewTab}
          onOpenInNewPane={onOpenInNewPane}
          onCopyPath={onCopyPath}
          onCopyLink={onCopyLink}
          onMoveToGroup={onMoveToGroup}
          onRemove={onRemove}
          menuRef={menuRef}
          collapsedPaths={collapsedPaths}
          onToggleCollapse={handleToggleCollapse}
        />
      ))}
    </>
  );
}

interface TreeNodeItemProps {
  node: TreeNode;
  depth: number;
  activeGroup: string;
  activeFileId: string | null;
  openFileIds: ReadonlySet<string>;
  showTitle: boolean;
  menuOpenId: string | null;
  otherGroups: Group[];
  onFileSelect: (id: string, target: PaneTarget) => void;
  onMenuToggle: (id: string) => void;
  onOpenInNewTab: (id: string) => void;
  onOpenInNewPane?: (id: string) => void;
  onCopyPath: (path: string) => void;
  onCopyLink: (id: string) => void;
  onMoveToGroup: (id: string, group: string) => void;
  onRemove: (id: string) => void;
  menuRef: React.RefObject<HTMLDivElement | null>;
  collapsedPaths: Set<string>;
  onToggleCollapse: (path: string) => void;
}

function TreeNodeItem({
  node,
  depth,
  activeGroup,
  activeFileId,
  openFileIds,
  showTitle,
  menuOpenId,
  otherGroups,
  onFileSelect,
  onMenuToggle,
  onOpenInNewTab,
  onOpenInNewPane,
  onCopyPath,
  onCopyLink,
  onMoveToGroup,
  onRemove,
  menuRef,
  collapsedPaths,
  onToggleCollapse,
}: TreeNodeItemProps) {
  if (node.file != null) {
    return (
      <FileNodeItem
        file={node.file}
        name={node.name}
        depth={depth}
        activeGroup={activeGroup}
        activeFileId={activeFileId}
        openFileIds={openFileIds}
        showTitle={showTitle}
        menuOpenId={menuOpenId}
        otherGroups={otherGroups}
        onFileSelect={onFileSelect}
        onMenuToggle={onMenuToggle}
        onOpenInNewTab={onOpenInNewTab}
        onOpenInNewPane={onOpenInNewPane}
        onCopyPath={onCopyPath}
        onCopyLink={onCopyLink}
        onMoveToGroup={onMoveToGroup}
        onRemove={onRemove}
        menuRef={menuRef}
      />
    );
  }

  const isCollapsed = collapsedPaths.has(node.fullPath);

  return (
    <div>
      <button
        className="flex items-center gap-1.5 w-full px-3 py-1.5 border-none cursor-pointer text-left text-sm bg-transparent text-gh-text-secondary hover:bg-gh-bg-hover transition-colors duration-150"
        style={{ paddingLeft: `${depth * 16 + 12}px` }}
        onClick={() => onToggleCollapse(node.fullPath)}
      >
        {/* Chevron */}
        <svg
          className={`size-3 shrink-0 transition-transform duration-150 ${isCollapsed ? "" : "rotate-90"}`}
          viewBox="0 0 16 16"
          fill="currentColor"
        >
          <path d="M6.427 4.427l3.396 3.396a.25.25 0 0 1 0 .354l-3.396 3.396A.25.25 0 0 1 6 11.396V4.604a.25.25 0 0 1 .427-.177Z" />
        </svg>
        {/* Folder icon */}
        <svg className="size-4 shrink-0" viewBox="0 0 16 16" fill="currentColor">
          {isCollapsed ? (
            <path d="M1.75 1A1.75 1.75 0 0 0 0 2.75v10.5C0 14.216.784 15 1.75 15h12.5A1.75 1.75 0 0 0 16 13.25v-8.5A1.75 1.75 0 0 0 14.25 3H7.5a.25.25 0 0 1-.2-.1l-.9-1.2c-.33-.44-.85-.7-1.4-.7Z" />
          ) : (
            <path d="M.513 1.513A1.75 1.75 0 0 1 1.75 1h3.2c.55 0 1.07.26 1.4.7l.9 1.2a.25.25 0 0 0 .2.1h6.8A1.75 1.75 0 0 1 16 4.75v8.5A1.75 1.75 0 0 1 14.25 15H1.75A1.75 1.75 0 0 1 0 13.25V2.75c0-.464.184-.91.513-1.237ZM1.75 2.5a.25.25 0 0 0-.25.25v10.5c0 .138.112.25.25.25h12.5a.25.25 0 0 0 .25-.25v-8.5a.25.25 0 0 0-.25-.25H7.5c-.55 0-1.07-.26-1.4-.7l-.9-1.2a.25.25 0 0 0-.2-.1Z" />
          )}
        </svg>
        <span className="overflow-hidden text-ellipsis whitespace-nowrap">{node.name}</span>
      </button>
      {!isCollapsed &&
        node.children.map((child) => (
          <TreeNodeItem
            key={child.fullPath}
            node={child}
            depth={depth + 1}
            activeGroup={activeGroup}
            activeFileId={activeFileId}
            openFileIds={openFileIds}
            showTitle={showTitle}
            menuOpenId={menuOpenId}
            otherGroups={otherGroups}
            onFileSelect={onFileSelect}
            onMenuToggle={onMenuToggle}
            onOpenInNewTab={onOpenInNewTab}
            onOpenInNewPane={onOpenInNewPane}
            onCopyPath={onCopyPath}
            onCopyLink={onCopyLink}
            onMoveToGroup={onMoveToGroup}
            onRemove={onRemove}
            menuRef={menuRef}
            collapsedPaths={collapsedPaths}
            onToggleCollapse={onToggleCollapse}
          />
        ))}
    </div>
  );
}

interface FileNodeItemProps {
  file: FileEntry;
  name: string;
  depth: number;
  activeGroup: string;
  activeFileId: string | null;
  openFileIds: ReadonlySet<string>;
  showTitle: boolean;
  menuOpenId: string | null;
  otherGroups: Group[];
  onFileSelect: (id: string, target: PaneTarget) => void;
  onMenuToggle: (id: string) => void;
  onOpenInNewTab: (id: string) => void;
  onOpenInNewPane?: (id: string) => void;
  onCopyPath: (path: string) => void;
  onCopyLink: (id: string) => void;
  onMoveToGroup: (id: string, group: string) => void;
  onRemove: (id: string) => void;
  menuRef: React.RefObject<HTMLDivElement | null>;
}

function FileNodeItem({
  file,
  name,
  depth,
  activeGroup,
  activeFileId,
  openFileIds,
  showTitle,
  menuOpenId,
  otherGroups,
  onFileSelect,
  onMenuToggle,
  onOpenInNewTab,
  onOpenInNewPane,
  onCopyPath,
  onCopyLink,
  onMoveToGroup,
  onRemove,
  menuRef,
}: FileNodeItemProps) {
  const isFocused = file.id === activeFileId;

  return (
    <div className="relative group/file">
      <a
        href={buildFileUrl(activeGroup, file.id)}
        className={fileRowClass(openFileIds.has(file.id), isFocused)}
        // 2px less than the flat list's indent to leave room for the focus bar.
        style={{ paddingLeft: `${depth * 16 + 10}px` }}
        onClick={(e) => handleFileRowClick(e, file.id, onFileSelect)}
        title={file.uploaded ? file.name : file.path}
        aria-current={isFocused ? "page" : undefined}
      >
        <FileIcon uploaded={file.uploaded} />
        <span className="overflow-hidden text-ellipsis whitespace-nowrap pr-6">
          {(showTitle && file.title) || name}
        </span>
      </a>
      <FileContextMenu
        file={file}
        isOpen={menuOpenId === file.id}
        otherGroups={otherGroups}
        onToggle={onMenuToggle}
        onOpenInNewTab={onOpenInNewTab}
        onOpenInNewPane={onOpenInNewPane}
        onCopyPath={onCopyPath}
        onCopyLink={onCopyLink}
        onMoveToGroup={onMoveToGroup}
        onRemove={onRemove}
        menuRef={menuRef}
      />
    </div>
  );
}
