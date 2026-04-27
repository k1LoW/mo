import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Sidebar } from "./components/Sidebar";
import { MarkdownViewer } from "./components/MarkdownViewer";
import { ThemeToggle } from "./components/ThemeToggle";
import { FontSizeToggle, type FontSize } from "./components/FontSizeToggle";
import { WidthToggle } from "./components/WidthToggle";
import { GroupDropdown } from "./components/GroupDropdown";
import { ViewModeToggle, type ViewMode } from "./components/ViewModeToggle";
import { SearchToggle } from "./components/SearchToggle";
import { TitleToggle } from "./components/TitleToggle";
import { RestartButton } from "./components/RestartButton";
import { DropOverlay } from "./components/DropOverlay";
import { ZoomModal } from "./components/ZoomModal";
import type { ZoomContent } from "./components/ZoomModal";
import { TocPanel } from "./components/TocPanel";
import type { TocHeading } from "./components/TocPanel";
import { useSSE } from "./hooks/useSSE";
import { useFileDrop } from "./hooks/useFileDrop";
import { useActiveHeading } from "./hooks/useActiveHeading";
import { useScrollRestoration, SCROLL_SESSION_KEY } from "./hooks/useScrollRestoration";
import type { FileEntry, Group, SearchResult } from "./hooks/useApi";
import { fetchGroups, fetchSearchResults, removeFile, reorderFiles } from "./hooks/useApi";
import {
  allFileIds,
  parseGroupFromPath,
  parseFileIdFromSearch,
  groupToPath,
  buildFileUrl,
} from "./utils/groups";
import { isMarkdownFile } from "./utils/filetype";

const VIEWMODE_STORAGE_KEY = "mo-sidebar-viewmode";
const WIDTH_STORAGE_KEY = "mo-layout-width";
const SHOW_TITLE_STORAGE_KEY = "mo-sidebar-show-title";
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
  const fullTitle = title === undefined ? name : `${title} - ${name}`;
  return `${fullTitle} | mo`;
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

export function App() {
  const [groups, setGroups] = useState<Group[]>([]);
  const [activeGroup, setActiveGroup] = useState<string>(
    () => parseGroupFromPath(window.location.pathname) || "default",
  );
  const [activeFileId, setActiveFileId] = useState<string | null>(null);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [tocOpenMap, setTocOpenMap] = useState<Record<string, boolean>>(getInitialTocOpenMap);
  const [headings, setHeadings] = useState<TocHeading[]>([]);
  const [contentRevision, setContentRevision] = useState(0);
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
    } else if (initialFileId != null) {
      setInitialFileId(null);
      setActiveFileId(
        group.files.some((f) => f.id === initialFileId) ? initialFileId : group.files[0].id,
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

      if (added.length > 0 && !wasEmpty) {
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
  }, []);

  // Initial data fetch (setState inside .then() is async, not flagged by linter)
  useEffect(() => {
    fetchGroups()
      .then((data) => {
        knownFileIds.current = allFileIds(data);
        setGroups(data);
      })
      .catch(() => {});
  }, []);

  // User-initiated navigation (file/group selection) calls pushState directly at
  // the call site. This effect only reconciles the URL with state for automatic
  // changes (initial mount, SSE updates, render-time fallbacks) via replaceState.
  useEffect(() => {
    // initialFileId hasn't been consumed yet — keep the URL as the user landed.
    if (initialFileId != null) return;
    const expectedUrl = activeFileId
      ? buildFileUrl(activeGroup, activeFileId)
      : groupToPath(activeGroup);
    if (window.location.pathname + window.location.search === expectedUrl) return;
    window.history.replaceState(null, "", expectedUrl);
  }, [activeGroup, activeFileId, initialFileId]);

  useEffect(() => {
    const handlePopState = () => {
      setActiveGroup(parseGroupFromPath(window.location.pathname));
      setActiveFileId(parseFileIdFromSearch(window.location.search));
    };
    window.addEventListener("popstate", handlePopState);
    return () => window.removeEventListener("popstate", handlePopState);
  }, []);

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

  const activeFile = useMemo(
    () => groups.find((g) => g.name === activeGroup)?.files.find((f) => f.id === activeFileId),
    [groups, activeGroup, activeFileId],
  );
  const activeFileName = activeFile?.name ?? "";
  const tocOpen = isTocOpenForFile(tocOpenMap, activeFileId, activeFileName);
  const currentShowTitle: boolean = showTitles[activeGroup] ?? false;

  const setTocOpen = useCallback(
    (open: boolean) => {
      if (activeFileId == null) return;
      setTocOpenMap((prev) => ({ ...prev, [activeFileId]: open }));
    },
    [activeFileId],
  );

  useEffect(() => {
    document.title = formatTitle(activeFile);
  }, [activeFile]);

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

  const handleGroupChange = useCallback((name: string) => {
    window.history.pushState(null, "", groupToPath(name));
    setActiveGroup(name);
    setActiveFileId(null);
  }, []);

  const handleFileSelect = useCallback(
    (fileId: string) => {
      window.history.pushState(null, "", buildFileUrl(activeGroup, fileId));
      setActiveFileId(fileId);
    },
    [activeGroup],
  );

  const handleFileOpened = useCallback(
    (fileId: string) => {
      window.history.pushState(null, "", buildFileUrl(activeGroup, fileId));
      setActiveFileId(fileId);
      setPendingSearchHeading(null);
    },
    [activeGroup],
  );

  const handleSearchResultSelect = useCallback(
    (fileId: string, heading?: string) => {
      window.history.pushState(null, "", buildFileUrl(activeGroup, fileId));
      setActiveFileId(fileId);
      setPendingSearchHeading(heading || null);
    },
    [activeGroup],
  );

  const handleRemoveFile = useCallback(() => {
    if (activeFileId != null) {
      removeFile(activeGroup, activeFileId);
    }
  }, [activeFileId, activeGroup]);

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
    const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    el?.scrollIntoView({ behavior: reduced ? "auto" : "smooth", block: "start" });
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
        <main className="flex-1 flex flex-col overflow-hidden">
          <div
            ref={setScrollContainer}
            className="flex-1 overflow-y-auto overscroll-contain p-8 bg-gh-bg"
          >
            {activeFileId != null ? (
              <MarkdownViewer
                fileId={activeFileId}
                fileName={activeFileName}
                activeGroup={activeGroup}
                revision={contentRevision}
                onFileOpened={handleFileOpened}
                onHeadingsChange={setHeadings}
                onContentRendered={onContentRendered}
                isTocOpen={tocOpen}
                onTocToggle={() => setTocOpen(!tocOpen)}
                onRemoveFile={handleRemoveFile}
                uploaded={activeFile?.uploaded}
                isWide={isWide}
                fontSize={fontSize}
                onZoom={handleZoom}
                scrollToHeading={pendingSearchHeading}
                onScrolledToHeading={() => setPendingSearchHeading(null)}
                searchQuery={searchQuery}
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
      <RestartButton />
      {isDragging && <DropOverlay />}
      {zoomContent && <ZoomModal content={zoomContent} onClose={handleZoomClose} />}
    </div>
  );
}
