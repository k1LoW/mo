interface ClosePaneButtonProps {
  onClose: () => void;
}

/**
 * Removes a column from the split layout. Deliberately distinct from
 * CloseFileButton, which removes the file from the group entirely — the icon
 * shows a column being dismissed rather than a document.
 */
export function ClosePaneButton({ onClose }: ClosePaneButtonProps) {
  return (
    <button
      type="button"
      className="flex items-center justify-center bg-transparent border border-gh-border rounded-md p-1.5 text-gh-text-secondary cursor-pointer transition-colors duration-150 hover:bg-gh-bg-hover"
      onClick={onClose}
      aria-label="Close pane"
      title="Close pane (keeps the file in the group)"
    >
      <svg
        className="size-5"
        fill="none"
        stroke="currentColor"
        strokeWidth={1.5}
        viewBox="0 0 24 24"
      >
        <rect x="3" y="4" width="18" height="16" rx="2" />
        <line x1="12" y1="4" x2="12" y2="20" />
        <path strokeLinecap="round" d="m15 10 4 4m0-4-4 4" />
      </svg>
    </button>
  );
}
