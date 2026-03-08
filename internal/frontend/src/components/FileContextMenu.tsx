import type { FileEntry, Group } from "../hooks/useApi";

const MENU_ITEM_CLASS =
  "w-full px-3 py-1.5 text-left text-sm bg-transparent border-none cursor-pointer text-gh-text-secondary hover:bg-gh-bg-hover hover:text-gh-text transition-colors duration-150 flex items-center gap-2";

interface FileContextMenuProps {
  file: FileEntry;
  isOpen: boolean;
  otherGroups: Group[];
  onToggle: (id: string) => void;
  onOpenInNewTab: (id: string) => void;
  onMoveToGroup: (id: string, group: string) => void;
  onRemove: (id: string) => void;
  menuRef: React.RefObject<HTMLDivElement | null>;
}

export function FileContextMenu({
  file,
  isOpen,
  otherGroups,
  onToggle,
  onOpenInNewTab,
  onMoveToGroup,
  onRemove,
  menuRef,
}: FileContextMenuProps) {
  return (
    <>
      <button
        className="absolute right-1 top-1/2 -translate-y-1/2 opacity-0 group-hover/file:opacity-100 flex items-center justify-center bg-transparent border-none cursor-pointer text-gh-text-secondary hover:text-gh-text rounded p-0.5 transition-opacity duration-150"
        onClick={(e) => {
          e.stopPropagation();
          onToggle(file.id);
        }}
        title="More actions"
      >
        <svg className="size-4" viewBox="0 0 16 16" fill="currentColor">
          <path d="M8 9a1.5 1.5 0 1 0 0-3 1.5 1.5 0 0 0 0 3ZM1.5 9a1.5 1.5 0 1 0 0-3 1.5 1.5 0 0 0 0 3Zm13 0a1.5 1.5 0 1 0 0-3 1.5 1.5 0 0 0 0 3Z" />
        </svg>
      </button>
      {isOpen && (
        <div
          ref={menuRef}
          className="absolute right-0 top-full z-10 bg-gh-bg border border-gh-border rounded-md shadow-lg py-1 min-w-[160px]"
        >
          <button
            className={MENU_ITEM_CLASS}
            onClick={() => onOpenInNewTab(file.id)}
          >
            <svg className="size-4" viewBox="0 0 16 16" fill="currentColor">
              <path d="M3.75 2h3.5a.75.75 0 0 1 0 1.5h-3.5a.25.25 0 0 0-.25.25v8.5c0 .138.112.25.25.25h8.5a.25.25 0 0 0 .25-.25v-3.5a.75.75 0 0 1 1.5 0v3.5A1.75 1.75 0 0 1 12.25 14h-8.5A1.75 1.75 0 0 1 2 12.25v-8.5C2 2.784 2.784 2 3.75 2Zm6.854-1h4.146a.25.25 0 0 1 .25.25v4.146a.25.25 0 0 1-.427.177L13.03 4.03 9.28 7.78a.751.751 0 0 1-1.042-.018.751.751 0 0 1-.018-1.042l3.75-3.75-1.543-1.543A.25.25 0 0 1 10.604 1Z" />
            </svg>
            Open in new tab
          </button>
          {otherGroups.length > 0 && (
            <>
              <div className="border-t border-gh-border my-1" />
              <div className="px-3 py-1.5 text-sm text-gh-text-secondary flex items-center gap-2">
                <svg className="size-4" viewBox="0 0 16 16" fill="currentColor">
                  <path d="M12.25 2a.75.75 0 0 1 0 1.5H3.75a.25.25 0 0 0-.25.25v8.5a.25.25 0 0 0 .25.25h8.5a.75.75 0 0 0 0 1.5H3.75A1.75 1.75 0 0 1 2 12.25V3.75A1.75 1.75 0 0 1 3.75 2Z" />
                  <path d="M12 5l3.5 3-3.5 3ZM8.75 7.25a.75.75 0 0 0 0 1.5H12.5V7.25H8.75Z" />
                </svg>
                Move to...
              </div>
              {otherGroups.map((g) => (
                <button
                  key={g.name}
                  className={`${MENU_ITEM_CLASS} !pl-9`}
                  onClick={() => onMoveToGroup(file.id, g.name)}
                >
                  <svg className="size-4 shrink-0" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
                    <rect x="3" y="3" width="7.5" height="7.5" rx="1.5" />
                    <rect x="13.5" y="3" width="7.5" height="7.5" rx="1.5" />
                    <rect x="3" y="13.5" width="7.5" height="7.5" rx="1.5" />
                    <rect x="13.5" y="13.5" width="7.5" height="7.5" rx="1.5" />
                  </svg>
                  {g.name === "default" ? "(default)" : g.name}
                </button>
              ))}
            </>
          )}
          <div className="border-t border-gh-border my-1" />
          <button
            className={MENU_ITEM_CLASS}
            onClick={() => onRemove(file.id)}
          >
            <svg className="size-4" viewBox="0 0 16 16" fill="currentColor">
              <path d="M11 1.75V3h2.25a.75.75 0 0 1 0 1.5H2.75a.75.75 0 0 1 0-1.5H5V1.75C5 .784 5.784 0 6.75 0h2.5C10.216 0 11 .784 11 1.75ZM4.496 6.675l.66 6.6a.25.25 0 0 0 .249.225h5.19a.25.25 0 0 0 .249-.225l.66-6.6a.75.75 0 0 1 1.492.149l-.66 6.6A1.748 1.748 0 0 1 10.595 15h-5.19a1.75 1.75 0 0 1-1.741-1.575l-.66-6.6a.75.75 0 1 1 1.492-.15ZM6.5 1.75V3h3V1.75a.25.25 0 0 0-.25-.25h-2.5a.25.25 0 0 0-.25.25Z" />
            </svg>
            Remove
          </button>
        </div>
      )}
    </>
  );
}
