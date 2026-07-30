import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { MarkdownViewer } from "./MarkdownViewer";
import type { FontSize } from "./FontSizeToggle";
import type { TocHeading } from "./TocPanel";
import type { ZoomContent } from "./ZoomModal";
import { useActiveHeading } from "../hooks/useActiveHeading";
import { useScrollRestoration } from "../hooks/useScrollRestoration";
import type { SyncPane } from "../hooks/useScrollSync";
import type { FileEntry } from "../hooks/useApi";
import { findElementById, scrollElementToTop } from "../utils/dom";

/**
 * What the focused pane publishes for the shared table-of-contents panel. The
 * panel is a single column on the right, so only one pane at a time feeds it.
 */
export interface FocusedToc {
  headings: TocHeading[];
  activeHeadingId: string | null;
  scrollToHeading: (id: string) => void;
}

interface PaneProps {
  fileId: string;
  file: FileEntry | undefined;
  activeGroup: string;
  paneIndex: number;
  isFocused: boolean;
  /** True when more than one column is open; drives the split-only chrome. */
  isSplit: boolean;
  /** False while the layout is collapsed to one column: no neighbour to drag against. */
  canResize: boolean;
  revision: number;
  isTocOpen: boolean;
  isWide: boolean;
  fontSize: FontSize;
  searchQuery: string | null;
  scrollToHeading: string | null;
  /** flex-grow share of the row; columns are equal until dragged. */
  weight: number;
  onResizeStart: (boundary: number, clientX: number) => void;
  onRegisterForSync: (index: number, pane: SyncPane | null) => void;
  onRequestFocus: (index: number) => void;
  onClosePane: (index: number) => void;
  onRemoveFile: (fileId: string) => void;
  onFileOpened: (fileId: string) => void;
  onTocToggle: (fileId: string) => void;
  onFocusedTocChange: (toc: FocusedToc) => void;
  onScrolledToHeading: () => void;
  onZoom: (content: ZoomContent) => void;
}

export function Pane({
  fileId,
  file,
  activeGroup,
  paneIndex,
  isFocused,
  isSplit,
  canResize,
  revision,
  isTocOpen,
  isWide,
  fontSize,
  searchQuery,
  scrollToHeading,
  weight,
  onResizeStart,
  onRegisterForSync,
  onRequestFocus,
  onClosePane,
  onRemoveFile,
  onFileOpened,
  onTocToggle,
  onFocusedTocChange,
  onScrolledToHeading,
  onZoom,
}: PaneProps) {
  const [scrollContainer, setScrollContainer] = useState<HTMLDivElement | null>(null);
  const [headings, setHeadings] = useState<TocHeading[]>([]);

  const headingIds = useMemo(() => headings.map((h) => h.id), [headings]);
  const activeHeadingId = useActiveHeading(headingIds, scrollContainer);
  const { captureScrollPosition, onContentRendered } = useScrollRestoration(
    scrollContainer,
    activeHeadingId,
    fileId,
    paneIndex,
  );

  const scrollToHeadingId = useCallback(
    (id: string) => {
      scrollElementToTop(scrollContainer, findElementById(scrollContainer, id));
    },
    [scrollContainer],
  );

  useEffect(() => {
    if (!isFocused) return;
    onFocusedTocChange({ headings, activeHeadingId, scrollToHeading: scrollToHeadingId });
  }, [isFocused, headings, activeHeadingId, scrollToHeadingId, onFocusedTocChange]);

  // A file-changed SSE event bumps `revision`, which makes MarkdownViewer refetch
  // the content. Capture the scroll position now rather than in App: the refetch
  // is async, so the DOM still holds the old content and scrollTop is still
  // meaningful, and only this pane knows its own scroll container.
  const previousRevision = useRef(revision);
  useEffect(() => {
    if (previousRevision.current === revision) return;
    previousRevision.current = revision;
    captureScrollPosition();
  }, [revision, captureScrollPosition]);

  // Scroll sync needs every pane's container and heading order, not just the
  // focused one's, so this registration is unconditional.
  useEffect(() => {
    if (scrollContainer == null) return;
    onRegisterForSync(paneIndex, { container: scrollContainer, headingIds });
    return () => onRegisterForSync(paneIndex, null);
  }, [onRegisterForSync, paneIndex, scrollContainer, headingIds]);

  const handleRemoveFile = useCallback(() => onRemoveFile(fileId), [onRemoveFile, fileId]);
  const handleTocToggle = useCallback(() => onTocToggle(fileId), [onTocToggle, fileId]);
  const handleClosePane = useCallback(() => onClosePane(paneIndex), [onClosePane, paneIndex]);

  // Capture phase so a click still focuses the column when the target stops
  // propagation (buttons inside the viewer toolbar do).
  const handleMouseDownCapture = useCallback(() => {
    if (!isFocused) onRequestFocus(paneIndex);
  }, [isFocused, onRequestFocus, paneIndex]);

  const focusBorder = isSplit && isFocused ? "border-t-gh-accent" : "border-t-transparent";
  const columnDivider = isSplit && paneIndex > 0 ? "border-l border-l-gh-border" : "";

  const handleResizeMouseDown = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      onResizeStart(paneIndex, e.clientX);
    },
    [onResizeStart, paneIndex],
  );

  return (
    <section
      className={`relative flex min-w-0 flex-col overflow-hidden border-t-2 ${focusBorder} ${columnDivider}`}
      style={{ flexGrow: weight, flexShrink: 1, flexBasis: 0 }}
      onMouseDownCapture={handleMouseDownCapture}
      aria-label={file?.name ?? fileId}
      aria-current={isSplit && isFocused ? "true" : undefined}
    >
      {canResize && paneIndex > 0 && (
        <div
          className="absolute top-0 left-0 z-20 h-full w-1 cursor-col-resize transition-colors hover:bg-gh-accent active:bg-gh-accent"
          onMouseDown={handleResizeMouseDown}
          role="separator"
          aria-orientation="vertical"
          aria-label="Resize pane"
        />
      )}
      <div
        ref={setScrollContainer}
        className="flex-1 overflow-y-auto overscroll-contain bg-gh-bg p-8"
      >
        <MarkdownViewer
          fileId={fileId}
          fileName={file?.name ?? ""}
          title={file?.title}
          filePath={file?.path}
          uploaded={file?.uploaded}
          scrollContainer={scrollContainer}
          activeGroup={activeGroup}
          revision={revision}
          onFileOpened={onFileOpened}
          onHeadingsChange={setHeadings}
          onContentRendered={onContentRendered}
          isTocOpen={isTocOpen}
          onTocToggle={handleTocToggle}
          onRemoveFile={handleRemoveFile}
          onClosePane={isSplit ? handleClosePane : undefined}
          isWide={isWide}
          fontSize={fontSize}
          onZoom={onZoom}
          scrollToHeading={scrollToHeading}
          onScrolledToHeading={onScrolledToHeading}
          searchQuery={searchQuery}
        />
      </div>
    </section>
  );
}
