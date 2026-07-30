import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Sidebar } from "./components/Sidebar";
import { Pane, type FocusedToc } from "./components/Pane";
import { ThemeToggle } from "./components/ThemeToggle";
import { FontSizeToggle, type FontSize } from "./components/FontSizeToggle";
import { WidthToggle } from "./components/WidthToggle";
import { GroupDropdown } from "./components/GroupDropdown";
import { ViewModeToggle, type ViewMode } from "./components/ViewModeToggle";
import { SearchToggle } from "./components/SearchToggle";
import { ScrollSyncToggle } from "./components/ScrollSyncToggle";
import { PaneNavigator } from "./components/PaneNavigator";
import { TitleToggle } from "./components/TitleToggle";
import { RestartButton } from "./components/RestartButton";
import { DropOverlay } from "./components/DropOverlay";
import { ZoomModal } from "./components/ZoomModal";
import type { ZoomContent } from "./components/ZoomModal";
import { TocPanel } from "./components/TocPanel";
import { EmptyGroupMessage } from "./components/EmptyGroupMessage";
import { useSSE } from "./hooks/useSSE";
import { useFileDrop } from "./hooks/useFileDrop";
import { usePanes } from "./hooks/usePanes";
import { usePaneWidths } from "./hooks/usePaneWidths";
import { useScrollSync } from "./hooks/useScrollSync";
import { useIsNarrow } from "./hooks/useIsNarrow";
import { SCROLL_SESSION_KEY } from "./hooks/useScrollRestoration";
import type { FileEntry, Group, SearchResult } from "./hooks/useApi";
import {
  fetchGroups,
  fetchSearchResults,
  openRelativeFile,
  removeFile,
  reorderFiles,
} from "./hooks/useApi";
import {
  allFileIds,
  parseGroupFromPath,
  parseRelativeOpenFromSearch,
  groupToPath,
} from "./utils/groups";
import {
  buildPanesUrl,
  EMPTY_PANES,
  focusedFileId as focusedFileIdOf,
  MAX_PANES,
  parsePanesFromSearch,
  reconcilePanes,
  setPaneFile,
  type PaneState,
  type PaneTarget,
} from "./utils/panes";
import { isMarkdownFile } from "./utils/filetype";
import { formatFileLabel } from "./utils/fileLabel";

const VIEWMODE_STORAGE_KEY = "mo-sidebar-viewmode";
const WIDTH_STORAGE_KEY = "mo-layout-width";
const SHOW_TITLE_STORAGE_KEY = "mo-sidebar-show-title";
export const SCROLL_SYNC_STORAGE_KEY = "mo-scroll-sync";
export const FONT_SIZE_STORAGE_KEY = "mo-font-size";
export const TOC_OPEN_STORAGE_KEY = "mo-toc-open";

export function getInitialFontSize(): FontSize {
  try {
    const stored = localStorage.getItem(FONT_SIZE_STORAGE_KEY);
    if (stored === "small" || stored === "medium" || stored === "large" || stored === "xlarge") {
      return stored;
    }
  } catch {
    /* ignore */
  }
  return "medium";
}

export function getInitialScrollSync(): boolean {
  try {
    return localStorage.getItem(SCROLL_SYNC_STORAGE_KEY) === "on";
  } catch {
    return false;
  }
}

export function getInitialTocOpenMap(): Record<string, boolean> {
  try {
    const stored = localStorage.getItem(TOC_OPEN_STORAGE_KEY);
    if (stored) {
      const parsed = JSON.parse(stored);
      if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
        return parsed;
      }
    }
  } catch {
    /* ignore */
  }
  return {};
}

export function formatTitle(fileEntry: Pick<FileEntry, "name" | "title"> | undefined): string {
  if (fileEntry == undefined) return "mo";
  const { name, title } = fileEntry;
  return `${formatFileLabel(name, title)} | mo`;
}

export function isTocOpenForFile(
  map: Record<string, boolean>,
  fileId: string | null,
  fileName: string,
): boolean {
  if (fileId == null) return false;
  if (fileName && !isMarkdownFile(fileName)) return false;
  return map[fileId] === true;
}

/**
 * The layout the URL (or a pre-reload session) asks for, before the group's file
 * list is known. Consumed once the groups arrive.
 */
export function getInitialPanes(): PaneState | null {
  const fromUrl = parsePanesFromSearch(window.location.search);
  if (fromUrl) return fromUrl;
  // Restore the active file from the scroll context saved before a reload.
  try {
    const stored = sessionStorage.getItem(SCROLL_SESSION_KEY);
    if (stored) {
      const ctx = JSON.parse(stored);
      if (ctx.url === window.location.pathname && ctx.fileId) {
        return { fileIds: [ctx.fileId], focusIndex: 0 };
      }
    }
  } catch {
    /* ignore */
  }
  return null;
}

export function App() {
  const [groups, setGroups] = useState<Group[]>([]);
  const [activeGroup, setActiveGroup] = useState<string>(
    () => parseGroupFromPath(window.location.pathname) || "default",
  );
  const { panes, setPanes, openInFocusedPane, openInNewPane, closePaneAt, focusPaneAt } = usePanes(
    activeGroup,
    EMPTY_PANES,
  );
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [scrollSync, setScrollSync] = useState<boolean>(getInitialScrollSync);
  const [tocOpenMap, setTocOpenMap] = useState<Record<string, boolean>>(getInitialTocOpenMap);
  // Only the focused pane feeds the shared ToC panel on the right.
  const [focusedToc, setFocusedToc] = useState<FocusedToc>({
    headings: [],
    activeHeadingId: null,
    scrollToHeading: () => {},
  });
  // Per-file so a file-changed event reloads every pane showing that file, not
  // just the focused one.
  const [revisions, setRevisions] = useState<Record<string, number>>({});
  const [searchQuery, setSearchQuery] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<SearchResult[]>([]);
  const [searchLoading, setSearchLoading] = useState(false);
  const [pendingSearchHeading, setPendingSearchHeading] = useState<string | null>(null);
  const [viewModes, setViewModes] = useState<Record<string, ViewMode>>(() => {
    try {
      const stored = localStorage.getItem(VIEWMODE_STORAGE_KEY);
      if (stored) return JSON.parse(stored);
    } catch {
      /* ignore */
    }
    return {};
  });
  const [showTitles, setShowTitles] = useState<Record<string, boolean>>(() => {
    try {
      const stored = localStorage.getItem(SHOW_TITLE_STORAGE_KEY);
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
  const [fontSize, setFontSize] = useState<FontSize>(getInitialFontSize);
  const knownFileIds = useRef<Set<string>>(new Set());
  const [pendingPanes, setPendingPanes] = useState<PaneState | null>(getInitialPanes);
  const [zoomContent, setZoomContent] = useState<ZoomContent | null>(null);

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
      setPanes(EMPTY_PANES);
    } else if (!group) {
      const sortedGroups = [...groups].sort((a, b) => {
        if (a.name === "default") return 1;
        if (b.name === "default") return -1;
        return a.name.localeCompare(b.name);
      });
      setActiveGroup(sortedGroups[0].name);
    } else if (group.files.length === 0) {
      setPanes(EMPTY_PANES);
    } else {
      const availableIds = new Set(group.files.map((f) => f.id));
      const firstFilePane: PaneState = { fileIds: [group.files[0].id], focusIndex: 0 };

      if (pendingPanes != null) {
        setPendingPanes(null);
        const requested = reconcilePanes(pendingPanes, availableIds);
        setPanes(requested.fileIds.length > 0 ? requested : firstFilePane);
      } else {
        // reconcilePanes returns its input untouched when every pane survives,
        // which is what keeps this render-time adjustment from looping.
        setPanes((prev) => {
          const reconciled = reconcilePanes(prev, availableIds);
          return reconciled.fileIds.length > 0 ? reconciled : firstFilePane;
        });
      }
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

      if (added.length > 0 && !wasEmpty) {
        // Only auto-select if the new file belongs to the current active group.
        // It lands in the focused pane so the other columns keep their content.
        setActiveGroup((currentGroup) => {
          const group = data.find((g) => g.name === currentGroup);
          if (group) {
            const addedSet = new Set(added);
            const matched = group.files.filter((f) => addedSet.has(f.id));
            if (matched.length > 0) {
              const newest = matched[matched.length - 1].id;
              setPanes((prev) => setPaneFile(prev, prev.focusIndex, newest));
            }
          }
          return currentGroup;
        });
      }
    } catch {
      // server may not be ready yet
    }
  }, [setPanes]);

  // Initial data fetch (setState inside .then() is async, not flagged by linter)
  useEffect(() => {
    fetchGroups()
      .then((data) => {
        knownFileIds.current = allFileIds(data);
        setGroups(data);
      })
      .catch(() => {});
  }, []);

  // A relative Markdown link opened in a new tab lands here with from/open params
  // because the target file has no ID until the server resolves it. Resolve it once
  // on load, then rewrite the URL to the canonical ?file= form.
  const relativeOpen = useRef(parseRelativeOpenFromSearch(window.location.search));
  const relativeOpenStarted = useRef(false);
  useEffect(() => {
    if (relativeOpenStarted.current) return;
    const rel = relativeOpen.current;
    if (!rel) return;
    relativeOpenStarted.current = true;
    const group = parseGroupFromPath(window.location.pathname);
    openRelativeFile(group, rel.from, rel.open)
      .then((entry) => {
        relativeOpen.current = null;
        const opened: PaneState = { fileIds: [entry.id], focusIndex: 0 };
        setPendingPanes(opened);
        window.history.replaceState(null, "", buildPanesUrl(group, opened));
        loadGroups();
      })
      .catch(() => {
        relativeOpen.current = null;
        window.history.replaceState(null, "", groupToPath(group));
      });
  }, [loadGroups]);

  // User-initiated navigation (file/group selection) calls pushState directly at
  // the call site. This effect only reconciles the URL with state for automatic
  // changes (initial mount, SSE updates, render-time fallbacks) via replaceState.
  useEffect(() => {
    // A relative-open resolve is in flight; it owns the URL until it settles.
    if (relativeOpen.current != null) return;
    // The requested layout hasn't been consumed yet — keep the URL as the user
    // landed on it.
    if (pendingPanes != null) return;
    const expectedUrl = buildPanesUrl(activeGroup, panes);
    if (window.location.pathname + window.location.search === expectedUrl) return;
    window.history.replaceState(null, "", expectedUrl);
  }, [activeGroup, panes, pendingPanes]);

  useEffect(() => {
    const handlePopState = () => {
      setActiveGroup(parseGroupFromPath(window.location.pathname));
      setPanes(parsePanesFromSearch(window.location.search) ?? EMPTY_PANES);
    };
    window.addEventListener("popstate", handlePopState);
    return () => window.removeEventListener("popstate", handlePopState);
  }, [setPanes]);

  useEffect(() => {
    if (!searchQuery?.trim()) {
      setSearchResults([]);
      setSearchLoading(false);
      return;
    }

    let cancelled = false;
    setSearchLoading(true);

    const timer = setTimeout(() => {
      fetchSearchResults(searchQuery, activeGroup)
        .then((resp) => {
          if (!cancelled) {
            setSearchResults(resp.results);
            setSearchLoading(false);
          }
        })
        .catch(() => {
          if (!cancelled) {
            setSearchResults([]);
            setSearchLoading(false);
          }
        });
    }, 300);

    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [searchQuery, activeGroup]);

  const activeGroupData = useMemo(
    () => groups.find((g) => g.name === activeGroup),
    [groups, activeGroup],
  );
  const filesById = useMemo(() => {
    const map = new Map<string, FileEntry>();
    for (const file of activeGroupData?.files ?? []) map.set(file.id, file);
    return map;
  }, [activeGroupData]);

  const activeFileId = focusedFileIdOf(panes);
  const activeFile = activeFileId != null ? filesById.get(activeFileId) : undefined;
  const openFileIds = useMemo(() => new Set(panes.fileIds), [panes.fileIds]);
  const isSplit = panes.fileIds.length > 1;

  const paneRowRef = useRef<HTMLElement | null>(null);
  const { weights, startResize } = usePaneWidths(panes.fileIds.length, paneRowRef);
  const { registerPane, syncFrom } = useScrollSync(scrollSync && isSplit);

  // Too narrow for columns: show the focused pane only, without touching the
  // layout, so widening the window brings every column straight back.
  const isNarrow = useIsNarrow();
  const isCollapsed = isNarrow && isSplit;
  const visiblePaneIndexes = useMemo(
    () =>
      isCollapsed ? [panes.focusIndex] : Array.from({ length: panes.fileIds.length }, (_, i) => i),
    [isCollapsed, panes.focusIndex, panes.fileIds.length],
  );

  // Align immediately when sync is switched on, rather than waiting for a scroll.
  useEffect(() => {
    if (scrollSync) syncFrom(panes.focusIndex);
  }, [scrollSync, syncFrom, panes.focusIndex]);

  useEffect(() => {
    try {
      localStorage.setItem(SCROLL_SYNC_STORAGE_KEY, scrollSync ? "on" : "off");
    } catch {
      /* ignore */
    }
  }, [scrollSync]);

  // The ToC panel shows the focused pane's outline, so its visibility follows
  // that pane's per-file toggle.
  const tocOpen = isTocOpenForFile(tocOpenMap, activeFileId, activeFile?.name ?? "");
  const currentShowTitle: boolean = showTitles[activeGroup] ?? false;

  const handleTocToggle = useCallback((fileId: string) => {
    setTocOpenMap((prev) => ({ ...prev, [fileId]: prev[fileId] !== true }));
  }, []);

  useEffect(() => {
    document.title = formatTitle(activeFile);
  }, [activeFile]);

  useSSE({
    onUpdate: () => {
      loadGroups();
    },
    onFileChanged: (fileId) => {
      // Keyed by file, so every pane showing it reloads. Each pane captures its
      // own scroll position off this revision bump.
      setRevisions((prev) => ({ ...prev, [fileId]: (prev[fileId] ?? 0) + 1 }));
    },
  });

  const { isDragging } = useFileDrop(activeGroup);

  const currentViewMode: ViewMode = viewModes[activeGroup] ?? "flat";

  useEffect(() => {
    localStorage.setItem(VIEWMODE_STORAGE_KEY, JSON.stringify(viewModes));
  }, [viewModes]);

  useEffect(() => {
    localStorage.setItem(SHOW_TITLE_STORAGE_KEY, JSON.stringify(showTitles));
  }, [showTitles]);

  useEffect(() => {
    try {
      localStorage.setItem(TOC_OPEN_STORAGE_KEY, JSON.stringify(tocOpenMap));
    } catch {
      /* ignore */
    }
  }, [tocOpenMap]);

  useEffect(() => {
    try {
      localStorage.setItem(WIDTH_STORAGE_KEY, isWide ? "wide" : "narrow");
    } catch {
      /* ignore */
    }
  }, [isWide]);

  useEffect(() => {
    try {
      localStorage.setItem(FONT_SIZE_STORAGE_KEY, fontSize);
    } catch {
      /* ignore */
    }
  }, [fontSize]);

  const handleViewModeToggle = useCallback(() => {
    setViewModes((prev) => {
      const current = prev[activeGroup] ?? "flat";
      const nextMode: ViewMode = current === "flat" ? "tree" : "flat";
      return { ...prev, [activeGroup]: nextMode };
    });
  }, [activeGroup]);

  const handleTitleToggle = useCallback(() => {
    setShowTitles((prev) => ({ ...prev, [activeGroup]: !prev[activeGroup] }));
  }, [activeGroup]);

  const handleSearchToggle = useCallback(() => {
    setSearchQuery((prev) => {
      if (prev != null) return null;
      setSidebarOpen(true);
      return "";
    });
  }, []);

  const handleGroupChange = useCallback(
    (name: string) => {
      window.history.pushState(null, "", groupToPath(name));
      setActiveGroup(name);
      // The new group's files decide the layout; reconciliation fills it in.
      setPanes(EMPTY_PANES);
    },
    [setPanes],
  );

  const handleFileSelect = useCallback(
    (fileId: string, target: PaneTarget) => {
      if (target === "new-pane") {
        openInNewPane(fileId);
        return;
      }
      openInFocusedPane(fileId);
    },
    [openInFocusedPane, openInNewPane],
  );

  const handleFileOpened = useCallback(
    (fileId: string) => {
      openInFocusedPane(fileId);
      setPendingSearchHeading(null);
    },
    [openInFocusedPane],
  );

  const handleSearchResultSelect = useCallback(
    (fileId: string, heading?: string) => {
      openInFocusedPane(fileId);
      setPendingSearchHeading(heading || null);
    },
    [openInFocusedPane],
  );

  const handleRemoveFile = useCallback(
    (fileId: string) => {
      removeFile(activeGroup, fileId);
    },
    [activeGroup],
  );

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

  const handleScrolledToHeading = useCallback(() => {
    setPendingSearchHeading(null);
  }, []);

  const handleZoom = useCallback((content: ZoomContent) => {
    setZoomContent(content);
  }, []);

  const handleZoomClose = useCallback(() => {
    setZoomContent(null);
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
        <TitleToggle showTitle={currentShowTitle} onToggle={handleTitleToggle} />
        <SearchToggle isOpen={searchQuery != null} onToggle={handleSearchToggle} />
        {isSplit && (
          <ScrollSyncToggle isSynced={scrollSync} onToggle={() => setScrollSync((v) => !v)} />
        )}
        {isCollapsed && (
          <PaneNavigator
            focusIndex={panes.focusIndex}
            paneCount={panes.fileIds.length}
            onFocusPane={focusPaneAt}
          />
        )}
        <div className="ml-auto flex items-center gap-2">
          <FontSizeToggle fontSize={fontSize} onChange={setFontSize} />
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
            openFileIds={openFileIds}
            canOpenNewPane={panes.fileIds.length < MAX_PANES}
            onFileSelect={handleFileSelect}
            onFilesReorder={handleFilesReorder}
            viewMode={currentViewMode}
            showTitle={currentShowTitle}
            searchQuery={searchQuery}
            onSearchQueryChange={setSearchQuery}
            searchResults={searchResults}
            searchLoading={searchLoading}
            onSearchResultSelect={handleSearchResultSelect}
          />
        )}
        <main ref={paneRowRef} className="flex flex-1 overflow-hidden">
          {panes.fileIds.length > 0 ? (
            visiblePaneIndexes.map((index) => {
              const fileId = panes.fileIds[index];
              const file = filesById.get(fileId);
              return (
                <Pane
                  // Keyed by position so swapping a column's file refetches in
                  // place instead of remounting the viewer.
                  key={index}
                  fileId={fileId}
                  file={file}
                  activeGroup={activeGroup}
                  paneIndex={index}
                  isFocused={index === panes.focusIndex}
                  isSplit={isSplit}
                  canResize={isSplit && !isCollapsed}
                  revision={revisions[fileId] ?? 0}
                  isTocOpen={isTocOpenForFile(tocOpenMap, fileId, file?.name ?? "")}
                  isWide={isWide}
                  fontSize={fontSize}
                  searchQuery={searchQuery}
                  scrollToHeading={index === panes.focusIndex ? pendingSearchHeading : null}
                  weight={isCollapsed ? 1 : (weights[index] ?? 1)}
                  onResizeStart={startResize}
                  onRegisterForSync={registerPane}
                  onRequestFocus={focusPaneAt}
                  onClosePane={closePaneAt}
                  onRemoveFile={handleRemoveFile}
                  onFileOpened={handleFileOpened}
                  onTocToggle={handleTocToggle}
                  onFocusedTocChange={setFocusedToc}
                  onScrolledToHeading={handleScrolledToHeading}
                  onZoom={handleZoom}
                />
              );
            })
          ) : (
            <div className="flex-1 overflow-y-auto overscroll-contain bg-gh-bg p-8">
              <EmptyGroupMessage group={activeGroupData} />
            </div>
          )}
        </main>
        {tocOpen && (
          <TocPanel
            headings={focusedToc.headings}
            activeHeadingId={focusedToc.activeHeadingId}
            onHeadingClick={focusedToc.scrollToHeading}
          />
        )}
      </div>
      <RestartButton />
      {isDragging && <DropOverlay />}
      {zoomContent && <ZoomModal content={zoomContent} onClose={handleZoomClose} />}
    </div>
  );
}
