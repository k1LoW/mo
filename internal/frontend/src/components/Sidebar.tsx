import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  DndContext,
  closestCenter,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core";
import {
  SortableContext,
  verticalListSortingStrategy,
  useSortable,
  arrayMove,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import type { FileEntry, Group, SearchResult } from "../hooks/useApi";
import { removeFile, moveFile } from "../hooks/useApi";
import { buildFileUrl } from "../utils/groups";
import { isPlainLeftClick } from "../utils/linkClick";
import { escapeRegExp } from "../utils/regex";
import { fileRowClass, handleFileRowClick } from "../utils/fileRow";
import type { PaneTarget } from "../utils/panes";
import type { ViewMode } from "./ViewModeToggle";
import { TreeView } from "./TreeView";
import { FileContextMenu } from "./FileContextMenu";
import { FileIcon } from "./FileIcon";

const MIN_WIDTH = 180;
const MAX_WIDTH = 480;
const DEFAULT_WIDTH = 260;
const STORAGE_KEY = "mo-sidebar-width";

function getInitialWidth(): number {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored) {
    const n = parseInt(stored, 10);
    if (n >= MIN_WIDTH && n <= MAX_WIDTH) return n;
  }
  return DEFAULT_WIDTH;
}

function renderHighlightedText(text: string, query: string) {
  if (!query) {
    return text;
  }

  const parts = text.split(new RegExp(`(${escapeRegExp(query)})`, "gi"));
  return parts.map((part, index) =>
    part.toLowerCase() === query.toLowerCase() ? (
      <mark key={`${part}:${index}`} className="bg-transparent p-0 font-semibold text-gh-text">
        {part}
      </mark>
    ) : (
      <span key={`${part}:${index}`}>{part}</span>
    ),
  );
}

interface FileItemProps {
  file: FileEntry;
  activeGroup: string;
  isOpen: boolean;
  isFocused: boolean;
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

function FileItem({
  file,
  activeGroup,
  isOpen,
  isFocused,
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
}: FileItemProps) {
  return (
    <div className="relative group/file">
      <a
        href={buildFileUrl(activeGroup, file.id)}
        className={fileRowClass(isOpen, isFocused)}
        onClick={(e) => handleFileRowClick(e, file.id, onFileSelect)}
        title={file.uploaded ? file.name : file.path}
        aria-current={isFocused ? "page" : undefined}
      >
        <FileIcon uploaded={file.uploaded} />
        <span className="overflow-hidden text-ellipsis whitespace-nowrap pr-6">
          {(showTitle && file.title) || file.name}
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

function SortableFileItem(props: FileItemProps) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: props.file.id,
  });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : undefined,
  };

  return (
    <div ref={setNodeRef} style={style} {...attributes} {...listeners}>
      <FileItem {...props} />
    </div>
  );
}

interface SidebarProps {
  groups: Group[];
  activeGroup: string;
  /** File in the focused pane. */
  activeFileId: string | null;
  /** Files open in any pane. Defaults to just the focused one. */
  openFileIds?: ReadonlySet<string>;
  /** False once the layout is full, which hides the new-pane affordances. */
  canOpenNewPane?: boolean;
  onFileSelect: (id: string, target: PaneTarget) => void;
  onFilesReorder: (groupName: string, fileIds: string[]) => void;
  viewMode: ViewMode;
  showTitle: boolean;
  searchQuery: string | null;
  onSearchQueryChange: (query: string | null) => void;
  searchResults?: SearchResult[];
  searchLoading?: boolean;
  onSearchResultSelect?: (fileId: string, heading?: string) => void;
}

export function Sidebar({
  groups,
  activeGroup,
  activeFileId,
  openFileIds,
  canOpenNewPane = true,
  onFileSelect,
  onFilesReorder,
  viewMode,
  showTitle,
  searchQuery,
  onSearchQueryChange,
  searchResults = [],
  searchLoading = false,
  onSearchResultSelect,
}: SidebarProps) {
  const allFiles = useMemo(() => {
    const currentGroup = groups.find((g) => g.name === activeGroup);
    return currentGroup?.files ?? [];
  }, [groups, activeGroup]);
  const searchInputRef = useRef<HTMLInputElement>(null);

  // Without split panes the focused file is the only open one, so callers that
  // predate panes (and tests) can keep passing activeFileId alone.
  const openIds = useMemo(
    () => openFileIds ?? new Set(activeFileId != null ? [activeFileId] : []),
    [openFileIds, activeFileId],
  );

  const searchOpen = searchQuery != null;
  const isSearching = searchQuery != null && searchQuery.length > 0;

  const files = useMemo(() => {
    if (!searchQuery) return allFiles;
    const q = searchQuery.toLowerCase();
    return allFiles.filter(
      (f) => f.name.toLowerCase().includes(q) || (f.title && f.title.toLowerCase().includes(q)),
    );
  }, [allFiles, searchQuery]);

  useEffect(() => {
    if (searchOpen) {
      searchInputRef.current?.focus();
    }
  }, [searchOpen]);

  const [width, setWidth] = useState(getInitialWidth);
  const resizeDragging = useRef(false);
  const [menuOpenId, setMenuOpenId] = useState<string | null>(null);
  const [contentMatchesOpen, setContentMatchesOpen] = useState(true);
  const [fileMatchesOpen, setFileMatchesOpen] = useState(true);
  const menuRef = useRef<HTMLDivElement>(null);

  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: { distance: 5 },
    }),
  );

  const handleDragEnd = useCallback(
    (event: DragEndEvent) => {
      const { active, over } = event;
      if (!over || active.id === over.id) return;

      const oldIndex = files.findIndex((f) => f.id === active.id);
      const newIndex = files.findIndex((f) => f.id === over.id);
      if (oldIndex === -1 || newIndex === -1) return;

      const reordered = arrayMove(files, oldIndex, newIndex);
      onFilesReorder(
        activeGroup,
        reordered.map((f) => f.id),
      );
    },
    [files, activeGroup, onFilesReorder],
  );

  const onMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    resizeDragging.current = true;
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
  }, []);

  useEffect(() => {
    const onMouseMove = (e: MouseEvent) => {
      if (!resizeDragging.current) return;
      const clamped = Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, e.clientX));
      setWidth(clamped);
    };
    const onMouseUp = () => {
      if (!resizeDragging.current) return;
      resizeDragging.current = false;
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
    };
    document.addEventListener("mousemove", onMouseMove);
    document.addEventListener("mouseup", onMouseUp);
    return () => {
      document.removeEventListener("mousemove", onMouseMove);
      document.removeEventListener("mouseup", onMouseUp);
    };
  }, []);

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, String(width));
  }, [width]);

  // Close menu on outside click
  useEffect(() => {
    if (menuOpenId == null) return;
    const handleClick = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpenId(null);
      }
    };
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [menuOpenId]);

  const handleOpenInNewTab = useCallback(
    (id: string) => {
      setMenuOpenId(null);
      window.open(buildFileUrl(activeGroup, id), "_blank");
    },
    [activeGroup],
  );

  const handleOpenInNewPane = useCallback(
    (id: string) => {
      setMenuOpenId(null);
      onFileSelect(id, "new-pane");
    },
    [onFileSelect],
  );

  const otherGroups = useMemo(() => {
    return [...groups]
      .filter((g) => g.name !== activeGroup)
      .sort((a, b) => {
        if (a.name === "default") return 1;
        if (b.name === "default") return -1;
        return a.name.localeCompare(b.name);
      });
  }, [groups, activeGroup]);

  const handleMoveToGroup = useCallback(
    async (id: string, group: string) => {
      setMenuOpenId(null);
      try {
        await moveFile(activeGroup, id, group);
      } catch (err) {
        window.alert(err instanceof Error ? err.message : "Failed to move file");
      }
    },
    [activeGroup],
  );

  const handleCopyPath = useCallback((path: string) => {
    setMenuOpenId(null);
    navigator.clipboard.writeText(path).catch(() => {});
  }, []);

  const handleCopyLink = useCallback(
    (id: string) => {
      setMenuOpenId(null);
      const url = new URL(buildFileUrl(activeGroup, id), window.location.origin);
      navigator.clipboard.writeText(url.toString()).catch(() => {});
    },
    [activeGroup],
  );

  const handleRemove = useCallback(
    (id: string) => {
      setMenuOpenId(null);
      removeFile(activeGroup, id);
    },
    [activeGroup],
  );

  const handleMenuToggle = useCallback((id: string) => {
    setMenuOpenId((prev) => (prev === id ? null : id));
  }, []);

  return (
    <aside
      className="relative bg-gh-bg-sidebar border-r border-gh-border flex flex-col overflow-y-auto overscroll-contain shrink-0"
      style={{ width }}
    >
      {searchOpen && (
        <div className="px-2 pt-2 pb-1">
          <input
            ref={searchInputRef}
            type="text"
            value={searchQuery}
            onChange={(e) => onSearchQueryChange(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Escape") onSearchQueryChange(null);
            }}
            placeholder="Search files..."
            className="w-full px-2 py-1.5 text-sm bg-gh-bg border border-gh-border rounded-md text-gh-text placeholder:text-gh-text-secondary outline-none focus:border-gh-accent"
          />
        </div>
      )}
      <nav className="flex flex-col pb-1">
        {isSearching ? (
          <>
            {searchLoading ? (
              <div className="px-3 py-2 text-sm text-gh-text-secondary">Searching contents...</div>
            ) : searchResults.length > 0 ? (
              <>
                <button
                  type="button"
                  className="flex w-full items-center justify-between px-3 pt-2 pb-1 text-left text-xs font-semibold uppercase tracking-wide text-gh-text-secondary"
                  onClick={() => setContentMatchesOpen((v) => !v)}
                  aria-expanded={contentMatchesOpen}
                >
                  <span>Content matches</span>
                  <span className="text-sm leading-none">{contentMatchesOpen ? "−" : "+"}</span>
                </button>
                {contentMatchesOpen &&
                  searchResults.flatMap((result) =>
                    result.matches.map((match, index) => (
                      <a
                        key={`${result.fileId}:${match.line}:${index}`}
                        href={buildFileUrl(activeGroup, result.fileId)}
                        className="w-full px-3 py-2 text-left border-none bg-transparent cursor-pointer no-underline text-gh-text-secondary transition-colors duration-150 hover:bg-gh-bg-hover"
                        onClick={(e) => {
                          if (!isPlainLeftClick(e)) return;
                          e.preventDefault();
                          onSearchResultSelect?.(result.fileId, match.heading);
                        }}
                      >
                        <div className="flex items-center gap-2 text-sm font-medium text-gh-text">
                          <FileIcon uploaded={result.uploaded} />
                          <span className="min-w-0 overflow-hidden text-ellipsis whitespace-nowrap">
                            {(showTitle && result.title) || result.fileName}
                          </span>
                        </div>
                        <div className="text-xs text-gh-text-secondary">{`Line ${match.line}`}</div>
                        <div className="mt-2 rounded-sm bg-gh-bg-hover/80 px-2 py-1.5">
                          {match.before?.map((line, i) => (
                            <div
                              key={`before:${i}`}
                              className="text-xs leading-5 text-gh-text-secondary whitespace-pre-wrap break-words"
                            >
                              {line}
                            </div>
                          ))}
                          <div className="text-sm leading-5 text-gh-text-secondary whitespace-pre-wrap break-words">
                            {renderHighlightedText(match.text, searchQuery)}
                          </div>
                          {match.after?.map((line, i) => (
                            <div
                              key={`after:${i}`}
                              className="text-xs leading-5 text-gh-text-secondary whitespace-pre-wrap break-words"
                            >
                              {line}
                            </div>
                          ))}
                        </div>
                      </a>
                    )),
                  )}
              </>
            ) : null}
            {files.length > 0 && (
              <>
                <button
                  type="button"
                  className="flex w-full items-center justify-between px-3 pt-2 pb-1 text-left text-xs font-semibold uppercase tracking-wide text-gh-text-secondary"
                  onClick={() => setFileMatchesOpen((v) => !v)}
                  aria-expanded={fileMatchesOpen}
                >
                  <span>File name matches</span>
                  <span className="text-sm leading-none">{fileMatchesOpen ? "−" : "+"}</span>
                </button>
                {fileMatchesOpen &&
                  files.map((f) => (
                    <FileItem
                      key={f.id}
                      file={f}
                      activeGroup={activeGroup}
                      isOpen={openIds.has(f.id)}
                      isFocused={f.id === activeFileId}
                      showTitle={showTitle}
                      menuOpenId={menuOpenId}
                      otherGroups={otherGroups}
                      onFileSelect={onFileSelect}
                      onMenuToggle={handleMenuToggle}
                      onOpenInNewTab={handleOpenInNewTab}
                      onOpenInNewPane={canOpenNewPane ? handleOpenInNewPane : undefined}
                      onCopyPath={handleCopyPath}
                      onCopyLink={handleCopyLink}
                      onMoveToGroup={handleMoveToGroup}
                      onRemove={handleRemove}
                      menuRef={menuRef}
                    />
                  ))}
              </>
            )}
            {!searchLoading && searchResults.length === 0 && files.length === 0 && (
              <div className="px-3 py-2 text-sm text-gh-text-secondary">No matches found</div>
            )}
          </>
        ) : viewMode === "tree" ? (
          <TreeView
            files={files}
            activeGroup={activeGroup}
            activeFileId={activeFileId}
            openFileIds={openIds}
            showTitle={showTitle}
            menuOpenId={menuOpenId}
            otherGroups={otherGroups}
            onFileSelect={onFileSelect}
            onMenuToggle={handleMenuToggle}
            onOpenInNewTab={handleOpenInNewTab}
            onOpenInNewPane={canOpenNewPane ? handleOpenInNewPane : undefined}
            onCopyPath={handleCopyPath}
            onCopyLink={handleCopyLink}
            onMoveToGroup={handleMoveToGroup}
            onRemove={handleRemove}
            menuRef={menuRef}
          />
        ) : (
          <DndContext
            sensors={sensors}
            collisionDetection={closestCenter}
            onDragEnd={handleDragEnd}
          >
            <SortableContext items={files.map((f) => f.id)} strategy={verticalListSortingStrategy}>
              {files.map((f) => (
                <SortableFileItem
                  key={f.id}
                  file={f}
                  activeGroup={activeGroup}
                  isOpen={openIds.has(f.id)}
                  isFocused={f.id === activeFileId}
                  showTitle={showTitle}
                  menuOpenId={menuOpenId}
                  otherGroups={otherGroups}
                  onFileSelect={onFileSelect}
                  onMenuToggle={handleMenuToggle}
                  onOpenInNewTab={handleOpenInNewTab}
                  onOpenInNewPane={canOpenNewPane ? handleOpenInNewPane : undefined}
                  onCopyPath={handleCopyPath}
                  onCopyLink={handleCopyLink}
                  onMoveToGroup={handleMoveToGroup}
                  onRemove={handleRemove}
                  menuRef={menuRef}
                />
              ))}
            </SortableContext>
          </DndContext>
        )}
      </nav>
      {/* Resize handle */}
      <div
        className="absolute top-0 right-0 w-1 h-full cursor-col-resize hover:bg-gh-border active:bg-gh-border transition-colors"
        onMouseDown={onMouseDown}
      />
    </aside>
  );
}
