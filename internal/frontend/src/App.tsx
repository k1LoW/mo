import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Sidebar } from "./components/Sidebar";
import { MarkdownViewer } from "./components/MarkdownViewer";
import { ThemeToggle } from "./components/ThemeToggle";
import { GroupDropdown } from "./components/GroupDropdown";
import { ViewModeToggle, type ViewMode } from "./components/ViewModeToggle";
import { RestartButton } from "./components/RestartButton";
import { TocPanel } from "./components/TocPanel";
import type { TocHeading } from "./components/TocPanel";
import { useSSE } from "./hooks/useSSE";
import { useActiveHeading } from "./hooks/useActiveHeading";
import type { Group } from "./hooks/useApi";
import { fetchGroups, removeFile, reorderFiles } from "./hooks/useApi";
import {
  allFileIds,
  parseGroupFromPath,
  parseFileIdFromSearch,
  groupToPath,
} from "./utils/groups";

const VIEWMODE_STORAGE_KEY = "mo-sidebar-viewmode";

export function App() {
  const [groups, setGroups] = useState<Group[]>([]);
  const [activeGroup, setActiveGroup] = useState<string>("default");
  const [activeFileId, setActiveFileId] = useState<number | null>(null);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [tocOpen, setTocOpen] = useState(false);
  const [headings, setHeadings] = useState<TocHeading[]>([]);
  const [contentRevision, setContentRevision] = useState(0);
  const [viewModes, setViewModes] = useState<Record<string, ViewMode>>(() => {
    try {
      const stored = localStorage.getItem(VIEWMODE_STORAGE_KEY);
      if (stored) return JSON.parse(stored);
    } catch { /* ignore */ }
    return {};
  });
  const knownFileIds = useRef<Set<number>>(new Set());
  const initialFileId = useRef<number | null>(
    parseFileIdFromSearch(window.location.search),
  );
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  const loadGroups = useCallback(async () => {
    try {
      const data = await fetchGroups();
      const newIds = allFileIds(data);
      const wasEmpty = knownFileIds.current.size === 0;
      const added: number[] = [];
      for (const id of newIds) {
        if (!knownFileIds.current.has(id)) {
          added.push(id);
        }
      }
      knownFileIds.current = newIds;

      setGroups(data);

      if (added.length > 0 && !wasEmpty && initialFileId.current == null) {
        // Only auto-select if the new file belongs to the current active group
        setActiveGroup((currentGroup) => {
          const group = data.find((g) => g.name === currentGroup);
          if (group) {
            const addedInGroup = added.filter((id) =>
              group.files.some((f) => f.id === id),
            );
            if (addedInGroup.length > 0) {
              setActiveFileId(Math.max(...addedInGroup));
            }
          }
          return currentGroup;
        });
      }
    } catch {
      // server may not be ready yet
    }
  }, []);

  useEffect(() => {
    loadGroups();
  }, [loadGroups]);

  useEffect(() => {
    const group = parseGroupFromPath(window.location.pathname);
    if (group !== "default") {
      setActiveGroup(group);
    }
  }, []);

  useEffect(() => {
    const group = groups.find((g) => g.name === activeGroup);
    if (!group || group.files.length === 0) {
      setActiveFileId(null);
      return;
    }
    if (initialFileId.current != null) {
      const requestedId = initialFileId.current;
      initialFileId.current = null;
      window.history.replaceState(null, "", groupToPath(activeGroup));
      if (group.files.some((f) => f.id === requestedId)) {
        setActiveFileId(requestedId);
        return;
      }
    }
    setActiveFileId((prev) => {
      const stillExists = group.files.some((f) => f.id === prev);
      if (stillExists) return prev;
      return group.files[0].id;
    });
  }, [groups, activeGroup]);

  useEffect(() => {
    const group = groups.find((g) => g.name === activeGroup);
    const file = group?.files.find((f) => f.id === activeFileId);
    document.title = file ? file.name : "mo";
  }, [groups, activeGroup, activeFileId]);

  // Auto open/close sidebar based on file count in active group
  useEffect(() => {
    const group = groups.find((g) => g.name === activeGroup);
    setSidebarOpen(group != null && group.files.length >= 2);
  }, [groups, activeGroup]);

  useSSE({
    onUpdate: () => {
      loadGroups();
    },
    onFileChanged: (fileId) => {
      setActiveFileId((current) => {
        if (current === fileId) {
          setContentRevision((r) => r + 1);
        }
        return current;
      });
    },
  });

  const currentViewMode: ViewMode = viewModes[activeGroup] ?? "flat";

  useEffect(() => {
    localStorage.setItem(VIEWMODE_STORAGE_KEY, JSON.stringify(viewModes));
  }, [viewModes]);

  const handleViewModeToggle = useCallback(() => {
    setViewModes((prev) => {
      const current = prev[activeGroup] ?? "flat";
      const nextMode: ViewMode = current === "flat" ? "tree" : "flat";
      return { ...prev, [activeGroup]: nextMode };
    });
  }, [activeGroup]);

  const handleGroupChange = (name: string) => {
    setActiveGroup(name);
    setActiveFileId(null);
    window.history.pushState(null, "", groupToPath(name));
  };

  const handleFileOpened = useCallback((fileId: number) => {
    setActiveFileId(fileId);
  }, []);

  const handleRemoveFile = useCallback(() => {
    if (activeFileId != null) {
      removeFile(activeFileId);
    }
  }, [activeFileId]);

  const handleFilesReorder = useCallback(
    (groupName: string, fileIds: number[]) => {
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
    },
    [],
  );

  const headingIds = useMemo(() => headings.map((h) => h.id), [headings]);

  const activeHeadingId = useActiveHeading(
    headingIds,
    scrollContainerRef.current,
  );

  const handleHeadingClick = useCallback((id: string) => {
    const el = document.getElementById(id);
    el?.scrollIntoView({ behavior: "smooth", block: "start" });
  }, []);

  return (
    <div className="flex flex-col h-full font-sans text-gh-text bg-gh-bg">
      <header className="h-12 shrink-0 flex items-center gap-3 px-4 bg-gh-header-bg text-gh-header-text border-b border-gh-header-border">
        <button
          className="flex items-center justify-center bg-transparent border border-gh-border rounded-md p-1.5 cursor-pointer text-gh-header-text transition-colors duration-150 hover:bg-gh-bg-hover"
          onClick={() => setSidebarOpen((v) => !v)}
          title="Toggle sidebar"
        >
          <svg className="size-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
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
        <div className="ml-auto">
          <ThemeToggle />
        </div>
      </header>
      <div className="flex flex-1 overflow-hidden">
        {sidebarOpen && <Sidebar
          groups={groups}
          activeGroup={activeGroup}
          activeFileId={activeFileId}
          onFileSelect={setActiveFileId}
          onFilesReorder={handleFilesReorder}
          viewMode={currentViewMode}
        />}
        <main className="flex-1 flex flex-col overflow-hidden">
          <div ref={scrollContainerRef} className="flex-1 overflow-y-auto p-8 bg-gh-bg">
            {activeFileId != null ? (
              <MarkdownViewer
                fileId={activeFileId}
                fileName={groups.find((g) => g.name === activeGroup)?.files.find((f) => f.id === activeFileId)?.name ?? ""}
                revision={contentRevision}
                onFileOpened={handleFileOpened}
                onHeadingsChange={setHeadings}
                isTocOpen={tocOpen}
                onTocToggle={() => setTocOpen((v) => !v)}
                onRemoveFile={handleRemoveFile}
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
    </div>
  );
}
