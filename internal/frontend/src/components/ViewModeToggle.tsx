export type ViewMode = "flat" | "tree";

interface ViewModeToggleProps {
  viewMode: ViewMode;
  onToggle: () => void;
}

export function ViewModeToggle({ viewMode, onToggle }: ViewModeToggleProps) {
  return (
    <button
      className="flex items-center justify-center bg-transparent border border-gh-border rounded-md p-1.5 text-gh-header-text cursor-pointer transition-colors duration-150 hover:bg-gh-bg-hover"
      onClick={onToggle}
      title={viewMode === "flat" ? "Switch to tree view" : "Switch to flat view"}
    >
      {viewMode === "flat" ? (
        <svg className="size-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 12.75V12A2.25 2.25 0 0 1 4.5 9.75h15A2.25 2.25 0 0 1 21.75 12v.75m-8.69-6.44-2.12-2.12a1.5 1.5 0 0 0-1.061-.44H4.5A2.25 2.25 0 0 0 2.25 6v12a2.25 2.25 0 0 0 2.25 2.25h15A2.25 2.25 0 0 0 21.75 18V9a2.25 2.25 0 0 0-2.25-2.25h-5.379a1.5 1.5 0 0 1-1.06-.44Z" />
        </svg>
      ) : (
        <svg className="size-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
          <line x1="4" y1="6" x2="20" y2="6" strokeLinecap="round" />
          <line x1="4" y1="12" x2="20" y2="12" strokeLinecap="round" />
          <line x1="4" y1="18" x2="20" y2="18" strokeLinecap="round" />
        </svg>
      )}
    </button>
  );
}
