interface PaneNavigatorProps {
  focusIndex: number;
  paneCount: number;
  onFocusPane: (index: number) => void;
}

const ARROW_CLASS =
  "flex items-center justify-center bg-transparent border-none rounded p-0.5 cursor-pointer text-gh-header-text transition-colors duration-150 hover:bg-gh-bg-hover disabled:opacity-40 disabled:cursor-default";

/**
 * Only rendered when the window is too narrow to show the columns side by side.
 * Without it the hidden panes would be unreachable: a sidebar click replaces the
 * focused pane's file rather than moving between panes.
 */
export function PaneNavigator({ focusIndex, paneCount, onFocusPane }: PaneNavigatorProps) {
  return (
    <div
      className="flex items-center gap-0.5 rounded-md border border-gh-border px-1 text-xs text-gh-header-text"
      title={`Window too narrow for ${paneCount} panes — showing one at a time`}
    >
      <button
        type="button"
        className={ARROW_CLASS}
        onClick={() => onFocusPane(focusIndex - 1)}
        disabled={focusIndex <= 0}
        aria-label="Previous pane"
      >
        <svg className="size-4" viewBox="0 0 16 16" fill="currentColor">
          <path d="M9.573 4.427 6.177 7.823a.25.25 0 0 0 0 .354l3.396 3.396A.25.25 0 0 0 10 11.396V4.604a.25.25 0 0 0-.427-.177Z" />
        </svg>
      </button>
      <span className="tabular-nums">
        {focusIndex + 1}/{paneCount}
      </span>
      <button
        type="button"
        className={ARROW_CLASS}
        onClick={() => onFocusPane(focusIndex + 1)}
        disabled={focusIndex >= paneCount - 1}
        aria-label="Next pane"
      >
        <svg className="size-4" viewBox="0 0 16 16" fill="currentColor">
          <path d="M6.427 4.427l3.396 3.396a.25.25 0 0 1 0 .354l-3.396 3.396A.25.25 0 0 1 6 11.396V4.604a.25.25 0 0 1 .427-.177Z" />
        </svg>
      </button>
    </div>
  );
}
