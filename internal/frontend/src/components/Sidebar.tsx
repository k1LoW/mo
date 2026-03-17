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
import type { FileEntry, Group } from "../hooks/useApi";
import { removeFile, moveFile } from "../hooks/useApi";
import { buildFileUrl } from "../utils/groups";
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

interface FileItemProps {
  file: FileEntry;
  isActive: boolean;
  showTitle: boolean;
  menuOpenId: string | null;
  otherGroups: Group[];
  onFileSelect: (id: string) => void;
  onMenuToggle: (id: string) => void;
  onOpenInNewTab: (id: string) => void;
  onMoveToGroup: (id: string, group: string) => void;
  onRemove: (id: string) => void;
  menuRef: React.RefObject<HTMLDivElement | null>;
}

function FileItem({
  file,
  isActive,
  showTitle,
  menuOpenId,
  otherGroups,
  onFileSelect,
  onMenuToggle,
  onOpenInNewTab,
  onMoveToGroup,
  onRemove,
  menuRef,
}: FileItemProps) {
  return (
    <div className="relative group/file">
      <button
        className={`flex items-center gap-2 w-full px-3 py-2 border-none cursor-pointer text-left text-sm transition-colors duration-150 ${
          isActive
            ? "bg-gh-bg-active text-gh-text font-semibold"
            : "bg-transparent text-gh-text-secondary hover:bg-gh-bg-hover"
        }`}
        onClick={() => onFileSelect(file.id)}
        title={file.uploaded ? file.name : file.path}
      >
        <FileIcon uploaded={file.uploaded} />
        <span className="overflow-hidden text-ellipsis whitespace-nowrap pr-6">
          {(showTitle && file.title) || file.name}
        </span>
      </button>
      <FileContextMenu
        file={file}
        isOpen={menuOpenId === file.id}
        otherGroups={otherGroups}
        onToggle={onMenuToggle}
        onOpenInNewTab={onOpenInNewTab}
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
  activeFileId: string | null;
  onFileSelect: (id: string) => void;
  onFilesReorder: (groupName: string, fileIds: string[]) => void;
  viewMode: ViewMode;
  showTitle: boolean;
  searchQuery: string | null;
  onSearchQueryChange: (query: string | null) => void;
}

export function Sidebar({
  groups,
  activeGroup,
  activeFileId,
  onFileSelect,
  onFilesReorder,
  viewMode,
  showTitle,
  searchQuery,
  onSearchQueryChange,
}: SidebarProps) {
  const allFiles = useMemo(() => {
    const currentGroup = groups.find((g) => g.name === activeGroup);
    return currentGroup?.files ?? [];
  }, [groups, activeGroup]);
  const searchInputRef = useRef<HTMLInputElement>(null);

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

  const otherGroups = useMemo(() => {
    return [...groups]
      .filter((g) => g.name !== activeGroup)
      .sort((a, b) => {
        if (a.name === "default") return 1;
        if (b.name === "default") return -1;
        return a.name.localeCompare(b.name);
      });
  }, [groups, activeGroup]);

  const handleMoveToGroup = useCallback(async (id: string, group: string) => {
    setMenuOpenId(null);
    try {
      await moveFile(id, group);
    } catch (err) {
      window.alert(err instanceof Error ? err.message : "Failed to move file");
    }
  }, []);

  const handleRemove = useCallback((id: string) => {
    setMenuOpenId(null);
    removeFile(id);
  }, []);

  const handleMenuToggle = useCallback((id: string) => {
    setMenuOpenId((prev) => (prev === id ? null : id));
  }, []);

  return (
    <aside
      className="relative bg-gh-bg-sidebar border-r border-gh-border flex flex-col overflow-y-auto shrink-0"
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
        {viewMode === "tree" ? (
          <TreeView
            files={files}
            activeGroup={activeGroup}
            activeFileId={activeFileId}
            showTitle={showTitle}
            menuOpenId={menuOpenId}
            otherGroups={otherGroups}
            onFileSelect={onFileSelect}
            onMenuToggle={handleMenuToggle}
            onOpenInNewTab={handleOpenInNewTab}
            onMoveToGroup={handleMoveToGroup}
            onRemove={handleRemove}
            menuRef={menuRef}
          />
        ) : isSearching ? (
          files.map((f) => (
            <FileItem
              key={f.id}
              file={f}
              isActive={f.id === activeFileId}
              showTitle={showTitle}
              menuOpenId={menuOpenId}
              otherGroups={otherGroups}
              onFileSelect={onFileSelect}
              onMenuToggle={handleMenuToggle}
              onOpenInNewTab={handleOpenInNewTab}
              onMoveToGroup={handleMoveToGroup}
              onRemove={handleRemove}
              menuRef={menuRef}
            />
          ))
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
                  isActive={f.id === activeFileId}
                  showTitle={showTitle}
                  menuOpenId={menuOpenId}
                  otherGroups={otherGroups}
                  onFileSelect={onFileSelect}
                  onMenuToggle={handleMenuToggle}
                  onOpenInNewTab={handleOpenInNewTab}
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
