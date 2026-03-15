export type TimestampMode = "off" | "relative" | "absolute";

interface TimestampToggleProps {
  mode: TimestampMode;
  onToggle: () => void;
}

const titles: Record<TimestampMode, string> = {
  off: "Show relative timestamps",
  relative: "Show absolute timestamps",
  absolute: "Hide timestamps",
};

export function TimestampToggle({ mode, onToggle }: TimestampToggleProps) {
  return (
    <button
      type="button"
      className={`flex items-center justify-center bg-transparent border border-gh-border rounded-md p-1.5 cursor-pointer transition-colors duration-150 hover:bg-gh-bg-hover ${
        mode !== "off" ? "text-gh-header-text bg-gh-bg-hover" : "text-gh-header-text opacity-50"
      }`}
      onClick={onToggle}
      aria-label={titles[mode]}
      title={titles[mode]}
    >
      {mode === "absolute" ? (
        /* calendar / ls-style icon */
        <svg
          className="size-5"
          fill="none"
          stroke="currentColor"
          strokeWidth={1.5}
          viewBox="0 0 24 24"
        >
          <rect x="3" y="4" width="18" height="17" rx="2" />
          <line x1="3" y1="9" x2="21" y2="9" />
          <line x1="8" y1="2" x2="8" y2="5" strokeLinecap="round" />
          <line x1="16" y1="2" x2="16" y2="5" strokeLinecap="round" />
        </svg>
      ) : (
        /* clock icon */
        <svg
          className="size-5"
          fill="none"
          stroke="currentColor"
          strokeWidth={1.5}
          viewBox="0 0 24 24"
        >
          <circle cx="12" cy="12" r="9" />
          <polyline points="12,7 12,12 15.5,14" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      )}
    </button>
  );
}
