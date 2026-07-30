interface ScrollSyncToggleProps {
  isSynced: boolean;
  onToggle: () => void;
}

/**
 * Locks the panes' scroll positions together. Only shown in split view — see
 * `utils/scrollSync.ts` for why alignment is by heading ordinal.
 */
export function ScrollSyncToggle({ isSynced, onToggle }: ScrollSyncToggleProps) {
  return (
    <button
      type="button"
      className={`flex items-center justify-center border rounded-md p-1.5 cursor-pointer transition-colors duration-150 hover:bg-gh-bg-hover ${
        isSynced
          ? "border-gh-accent bg-gh-bg-active text-gh-accent"
          : "border-gh-border bg-transparent text-gh-header-text"
      }`}
      onClick={onToggle}
      aria-label="Sync scrolling"
      aria-pressed={isSynced}
      title={isSynced ? "Scroll sync: on" : "Scroll sync: off"}
    >
      <svg
        className="size-5"
        fill="none"
        stroke="currentColor"
        strokeWidth={1.5}
        viewBox="0 0 24 24"
      >
        <path strokeLinecap="round" strokeLinejoin="round" d="M6 9 3 12l3 3" />
        <path strokeLinecap="round" strokeLinejoin="round" d="m18 9 3 3-3 3" />
        <path strokeLinecap="round" d="M3 12h18" />
        <path strokeLinecap="round" d="M12 4v3m0 10v3" />
      </svg>
    </button>
  );
}
