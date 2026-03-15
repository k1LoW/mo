import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Sidebar } from "./components/Sidebar";
import { MarkdownViewer } from "./components/MarkdownViewer";
import { ThemeToggle } from "./components/ThemeToggle";
import { WidthToggle } from "./components/WidthToggle";
import { GroupDropdown } from "./components/GroupDropdown";
import { ViewModeToggle, type ViewMode } from "./components/ViewModeToggle";
import { SearchToggle } from "./components/SearchToggle";
import { TimestampToggle, type TimestampMode } from "./components/TimestampToggle";
import { TreeCollapseToggle } from "./components/TreeCollapseToggle";
import type { TreeViewHandle } from "./components/TreeView";
import { RestartButton } from "./components/RestartButton";
import { DropOverlay } from "./components/DropOverlay";
import { TocPanel } from "./components/TocPanel";
import type { TocHeading } from "./components/TocPanel";
import { useSSE } from "./hooks/useSSE";
import { useFileDrop } from "./hooks/useFileDrop";
import { useActiveHeading } from "./hooks/useActiveHeading";
import { useScrollRestoration, SCROLL_SESSION_KEY } from "./hooks/useScrollRestoration";
import type { Group, VersionInfo } from "./hooks/useApi";
import { fetchGroups, fetchVersion, removeFile, reorderFiles } from "./hooks/useApi";
import { allFileIds, parseGroupFromPath, parseFileIdFromSearch, parseFilenameFromSearch, groupToPath } from "./utils/groups";
import { isMarkdownFile } from "./utils/filetype";

const VIEWMODE_STORAGE_KEY = "mo-sidebar-viewmode";
const WIDTH_STORAGE_KEY = "mo-layout-width";
const TIMESTAMPS_STORAGE_KEY = "mo-timestamp-mode";

export function App() {
  const [groups, setGroups] = useState<Group[]>([]);
  const [activeGroup, setActiveGroup] = useState<string>(
    () => parseGroupFromPath(window.location.pathname) || "default",
  );
  const [activeFileId, setActiveFileId] = useState<string | null>(null);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [tocOpen, setTocOpen] = useState(false);
  const [headings, setHeadings] = useState<TocHeading[]>([]);
  const [contentRevision, setContentRevision] = useState(0);
  const [searchQuery, setSearchQuery] = useState<string | null>(null);
  const [timestampMode, setTimestampMode] = useState<TimestampMode>(() => {
    try {
      const v = localStorage.getItem(TIMESTAMPS_STORAGE_KEY);
      if (v === "relative" || v === "absolute") return v;
    } catch {
      /* ignore */
    }
    return "off";
  });
  const [viewModes, setViewModes] = useState<Record<string, ViewMode>>(() => {
    try {
      const stored = localStorage.getItem(VIEWMODE_STORAGE_KEY);
      if (stored) return JSON.parse(stored);
    } catch {
      /* ignore */
    }
    return {};
  });
  const [isWide, setIsWide] = useState(() => {
    try {
      return localStorage.getItem(WIDTH_STORAGE_KEY) === "wide";
    } catch {
      return false;
    }
  });
  const [version, setVersion] = useState<VersionInfo | null>(null);
  const knownFileIds = useRef<Set<string>>(new Set());
  const treeViewRef = useRef<TreeViewHandle>(null);
  const [treeAllCollapsed, setTreeAllCollapsed] = useState(false);
  const [initialFilename, setInitialFilename] = useState<string | null>(() => parseFilenameFromSearch(window.location.search));
  const [initialFileId, setInitialFileId] = useState<string | null>(() => {
    const fromUrl = parseFileIdFromSearch(window.location.search);
    if (fromUrl) return fromUrl;
    // Restore active file from scroll context saved before reload
    try {
      const stored = sessionStorage.getItem(SCROLL_SESSION_KEY);
      if (stored) {
        const ctx = JSON.parse(stored);
        if (ctx.url === window.location.pathname && ctx.fileId) return ctx.fileId;
      }
    } catch {
      /* ignore */
    }
    return null;
  });
  const [scrollContainer, setScrollContainer] = useState<HTMLDivElement | null>(null);

  // Track previous values for render-time state adjustment
  const [prevGroups, setPrevGroups] = useState<Group[]>([]);
  const [prevActiveGroup, setPrevActiveGroup] = useState(activeGroup);

  // Adjust derived state during render when groups or activeGroup changes
  if (groups !== prevGroups || activeGroup !== prevActiveGroup) {
    setPrevGroups(groups);
    setPrevActiveGroup(activeGroup);

    // Active file selection and sidebar auto open/close
    const group = groups.find((g) => g.name === activeGroup);
    setSidebarOpen(group != null && group.files.length >= 2);

    if (groups.length === 0) {
      setActiveFileId(null);
    } else if (!group) {
      const sortedGroups = [...groups].sort((a, b) => {
        if (a.name === "default") return 1;
        if (b.name === "default") return -1;
        return a.name.localeCompare(b.name);
      });
      setActiveGroup(sortedGroups[0].name);
    } else if (group.files.length === 0) {
      setActiveFileId(null);
    } else if (initialFileId != null || initialFilename != null) {
      // Resolve initialFilename to an ID by matching against the group's files
      let resolvedId: string | null = initialFileId;
      if (resolvedId == null && initialFilename != null) {
        const match = group.files.find((f) => f.name === initialFilename);
        resolvedId = match?.id ?? null;
      }
      setInitialFileId(null);
      setInitialFilename(null);
      setActiveFileId(
        resolvedId && group.files.some((f) => f.id === resolvedId) ? resolvedId : group.files[0].id,
      );
    } else {
      setActiveFileId((prev) => {
        if (group.files.some((f) => f.id === prev)) return prev;
        return group.files[0].id;
      });
    }
  }

  const loadGroups = useCallback(async () => {
    try {
      const data = await fetchGroups();
      const newIds = allFileIds(data);
      const wasEmpty = knownFileIds.current.size === 0;
      const added: string[] = [];
      for (const id of newIds) {
        if (!knownFileIds.current.has(id)) {
          added.push(id);
        }
      }
      knownFileIds.current = newIds;

      setGroups(data);

      if (added.length > 0 && !wasEmpty && !version?.noNewFileAutoSelect) {
        // Only auto-select if the new file belongs to the current active group
        setActiveGroup((currentGroup) => {
          const group = data.find((g) => g.name === currentGroup);
          if (group) {
            const addedSet = new Set(added);
            const matched = group.files.filter((f) => addedSet.has(f.id));
            if (matched.length > 0) {
              setActiveFileId(matched[matched.length - 1].id);
            }
          }
          return currentGroup;
        });
      }
    } catch {
      // server may not be ready yet
    }
  }, [version]);

  // Initial data fetch (setState inside .then() is async, not flagged by linter)
  useEffect(() => {
    fetchGroups()
      .then((data) => {
        knownFileIds.current = allFileIds(data);
        setGroups(data);
      })
      .catch(() => {});
    fetchVersion()
      .then(setVersion)
      .catch(() => {});
  }, []);

  // Sync URL path with active group
  useEffect(() => {
    const expectedPath = groupToPath(activeGroup);
    if (window.location.pathname !== expectedPath) {
      window.history.replaceState(null, "", expectedPath);
    }
  }, [activeGroup]);

  // Sync ?file=<id> or ?filename=<name> in URL (shareable mode) or clear it after initial consume.
  // Wait for version to load before clearing so we don't race with shareable detection.
  useEffect(() => {
    if (version === null) return;
    if (version.shareable) {
      let search = "";
      if (activeFileId) {
        if (version.trueFilenames) {
          const group = groups.find((g) => g.name === activeGroup);
          const file = group?.files.find((f) => f.id === activeFileId);
          const isUnique = file && group!.files.filter((f) => f.name === file.name).length === 1;
          search = file && isUnique
            ? `?filename=${encodeURIComponent(file.name)}`
            : `?file=${activeFileId}`;
        } else {
          search = `?file=${activeFileId}`;
        }
      }
      const next = window.location.pathname + search;
      if (window.location.pathname + window.location.search !== next) {
        window.history.replaceState(null, "", next);
      }
    } else if (initialFileId === null && initialFilename === null && window.location.search) {
      window.history.replaceState(null, "", window.location.pathname);
    }
  }, [activeFileId, activeGroup, groups, initialFileId, initialFilename, version]);

  const activeFileName = useMemo(
    () =>
      groups.find((g) => g.name === activeGroup)?.files.find((f) => f.id === activeFileId)?.name ??
      "",
    [groups, activeGroup, activeFileId],
  );

  useEffect(() => {
    document.title = activeFileName || "mo";
  }, [activeFileName]);

  // Auto-close ToC panel when switching to a non-markdown file
  useEffect(() => {
    if (activeFileName && !isMarkdownFile(activeFileName)) {
      setTocOpen(false);
    }
  }, [activeFileName]);

  useSSE({
    onUpdate: () => {
      loadGroups();
    },
    onFileChanged: (fileId) => {
      captureScrollPosition();
      setActiveFileId((current) => {
        if (current === fileId) {
          setContentRevision((r) => r + 1);
        }
        return current;
      });
    },
  });

  const { isDragging } = useFileDrop(activeGroup);

  const currentViewMode: ViewMode = viewModes[activeGroup] ?? "flat";

  useEffect(() => {
    localStorage.setItem(VIEWMODE_STORAGE_KEY, JSON.stringify(viewModes));
  }, [viewModes]);

  useEffect(() => {
    try {
      localStorage.setItem(WIDTH_STORAGE_KEY, isWide ? "wide" : "narrow");
    } catch {
      /* ignore */
    }
  }, [isWide]);

  useEffect(() => {
    try {
      localStorage.setItem(TIMESTAMPS_STORAGE_KEY, timestampMode);
    } catch {
      /* ignore */
    }
  }, [timestampMode]);

  const handleViewModeToggle = useCallback(() => {
    const current = viewModes[activeGroup] ?? "flat";
    const nextMode: ViewMode = current === "flat" ? "tree" : "flat";
    setViewModes((prev) => ({ ...prev, [activeGroup]: nextMode }));
    if (nextMode === "flat") setTreeAllCollapsed(false);
  }, [activeGroup, viewModes]);

  const handleSearchToggle = useCallback(() => {
    setSearchQuery((prev) => (prev != null ? null : ""));
  }, []);

  const handleGroupChange = (name: string) => {
    setActiveGroup(name);
    setActiveFileId(null);
    setTreeAllCollapsed(false);
    window.history.pushState(null, "", groupToPath(name));
  };

  const handleFileOpened = useCallback((fileId: string) => {
    setActiveFileId(fileId);
  }, []);

  const handleRemoveFile = useCallback(() => {
    if (activeFileId != null) {
      removeFile(activeFileId);
    }
  }, [activeFileId]);

  const handleFilesReorder = useCallback((groupName: string, fileIds: string[]) => {
    // Optimistic update
    setGroups((prev) =>
      prev.map((g) => {
        if (g.name !== groupName) return g;
        const idToFile = new Map(g.files.map((f) => [f.id, f]));
        const reordered = fileIds
          .map((id) => idToFile.get(id))
          .filter((f): f is NonNullable<typeof f> => f != null);
        return { ...g, files: reordered };
      }),
    );
    reorderFiles(groupName, fileIds);
  }, []);

  const headingIds = useMemo(() => headings.map((h) => h.id), [headings]);

  const activeHeadingId = useActiveHeading(headingIds, scrollContainer);

  const { captureScrollPosition, onContentRendered } = useScrollRestoration(
    scrollContainer,
    activeHeadingId,
    activeFileId,
  );

  const handleHeadingClick = useCallback((id: string) => {
    const el = document.getElementById(id);
    el?.scrollIntoView({ behavior: "smooth", block: "start" });
  }, []);

  return (
    <div className="flex flex-col h-full font-sans text-gh-text bg-gh-bg">
      <header className="h-12 shrink-0 flex items-center gap-3 px-4 bg-gh-header-bg text-gh-header-text border-b border-gh-header-border">
        <button
          type="button"
          className="flex items-center justify-center bg-transparent border border-gh-border rounded-md p-1.5 cursor-pointer text-gh-header-text transition-colors duration-150 hover:bg-gh-bg-hover"
          onClick={() => setSidebarOpen((v) => !v)}
          aria-label="Sidebar"
          aria-expanded={sidebarOpen}
          title="Toggle sidebar"
        >
          <svg
            className="size-5"
            fill="none"
            stroke="currentColor"
            strokeWidth={1.5}
            viewBox="0 0 24 24"
          >
            <rect x="2" y="3" width="20" height="18" rx="2" />
            <line x1="9" y1="3" x2="9" y2="21" />
            {sidebarOpen ? (
              <polyline points="6,10 4,12 6,14" />
            ) : (
              <polyline points="5,10 7,12 5,14" />
            )}
          </svg>
        </button>
        <GroupDropdown
          groups={groups}
          activeGroup={activeGroup}
          onGroupChange={handleGroupChange}
        />
        <ViewModeToggle viewMode={currentViewMode} onToggle={handleViewModeToggle} />
        {currentViewMode === "tree" && (
          <TreeCollapseToggle
            collapsed={treeAllCollapsed}
            onToggle={() => {
              if (treeAllCollapsed) {
                treeViewRef.current?.expandAll();
                setTreeAllCollapsed(false);
              } else {
                treeViewRef.current?.collapseAll();
                setTreeAllCollapsed(true);
              }
            }}
          />
        )}
        <SearchToggle isOpen={searchQuery != null} onToggle={handleSearchToggle} />
        <TimestampToggle mode={timestampMode} onToggle={() => setTimestampMode((v) => v === "off" ? "relative" : v === "relative" ? "absolute" : "off")} />
        <div className="ml-auto flex items-center gap-2">
          <WidthToggle isWide={isWide} onToggle={() => setIsWide((v) => !v)} />
          <ThemeToggle />
        </div>
      </header>
      <div className="flex flex-1 overflow-hidden">
        {sidebarOpen && (
          <Sidebar
            groups={groups}
            activeGroup={activeGroup}
            activeFileId={activeFileId}
            onFileSelect={setActiveFileId}
            onFilesReorder={handleFilesReorder}
            viewMode={currentViewMode}
            searchQuery={searchQuery}
            onSearchQueryChange={setSearchQuery}
            treeViewRef={treeViewRef}
            noDelete={version?.noDelete}
            noFileMove={version?.noFileMove}
            timestampMode={timestampMode}
          />
        )}
        <main className="flex-1 flex flex-col overflow-hidden">
          <div ref={setScrollContainer} className="flex-1 overflow-y-auto p-8 bg-gh-bg">
            {activeFileId != null ? (
              <MarkdownViewer
                fileId={activeFileId}
                fileName={activeFileName}
                revision={contentRevision}
                onFileOpened={handleFileOpened}
                onHeadingsChange={setHeadings}
                onContentRendered={onContentRendered}
                isTocOpen={tocOpen}
                onTocToggle={() => setTocOpen((v) => !v)}
                onRemoveFile={handleRemoveFile}
                isWide={isWide}
                noDelete={version?.noDelete}
                shareable={version?.shareable}
              />
            ) : (
              <div className="flex items-center justify-center h-50 text-gh-text-secondary text-sm">
                No file selected
              </div>
            )}
          </div>
        </main>
        {tocOpen && (
          <TocPanel
            headings={headings}
            activeHeadingId={activeHeadingId}
            onHeadingClick={handleHeadingClick}
          />
        )}
      </div>
      <RestartButton version={version} noRestart={version?.noRestart} />
      {isDragging && <DropOverlay />}
    </div>
  );
}
