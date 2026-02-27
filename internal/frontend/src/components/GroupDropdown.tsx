import { useEffect, useRef, useState } from "react";
import type { Group } from "../hooks/useApi";

interface GroupDropdownProps {
  groups: Group[];
  activeGroup: string;
  onGroupChange: (name: string) => void;
}

export function GroupDropdown({
  groups,
  activeGroup,
  onGroupChange,
}: GroupDropdownProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [open]);

  const isDefault = activeGroup === "default";

  // Single non-default group: show name only (no dropdown)
  if (groups.length <= 1) {
    if (isDefault) return null;
    return (
      <span className="text-sm text-gh-header-text font-bold">
        {activeGroup}
      </span>
    );
  }

  return (
    <div ref={ref} className="relative">
      <button
        className="flex items-center gap-1.5 bg-transparent border border-gh-border rounded-md p-1.5 cursor-pointer text-sm text-gh-header-text hover:bg-gh-bg-hover transition-colors duration-150"
        onClick={() => setOpen((v) => !v)}
      >
        <svg className="size-5 shrink-0" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
          <rect x="3" y="3" width="7.5" height="7.5" rx="1.5" />
          <rect x="13.5" y="3" width="7.5" height="7.5" rx="1.5" />
          <rect x="3" y="13.5" width="7.5" height="7.5" rx="1.5" />
          <rect x="13.5" y="13.5" width="7.5" height="7.5" rx="1.5" />
        </svg>
        {!isDefault && (
          <span className="overflow-hidden text-ellipsis whitespace-nowrap max-w-32 font-bold">
            {activeGroup}
          </span>
        )}
        <svg className="size-3 shrink-0" fill="currentColor" viewBox="0 0 12 12">
          <path d="M2.5 4.5 6 8l3.5-3.5z" />
        </svg>
      </button>
      {open && (
        <div className="absolute left-0 top-full mt-1 min-w-40 bg-gh-bg-sidebar border border-gh-border rounded-md shadow-lg z-10 py-1 max-h-60 overflow-y-auto">
          {groups.map((g) => (
            <button
              key={g.name}
              className={`flex items-center gap-2 w-full px-3 py-1.5 border-none cursor-pointer text-left text-xs transition-colors duration-150 ${
                g.name === activeGroup
                  ? "bg-gh-bg-active text-gh-text font-semibold"
                  : "bg-transparent text-gh-text-secondary hover:bg-gh-bg-hover"
              }`}
              onClick={() => {
                onGroupChange(g.name);
                setOpen(false);
              }}
            >
              {g.name === activeGroup ? (
                <svg className="size-3.5 shrink-0" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" d="m4.5 12.75 6 6 9-13.5" />
                </svg>
              ) : (
                <span className="inline-block size-3.5 shrink-0" />
              )}
              {g.name === "default" ? (
                <svg className="size-4 shrink-0 text-gh-text-secondary" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 17.25v3.375c0 .621-.504 1.125-1.125 1.125h-9.75a1.125 1.125 0 0 1-1.125-1.125V7.875c0-.621.504-1.125 1.125-1.125H6.75a9.06 9.06 0 0 1 1.5.124m7.5 10.376h3.375c.621 0 1.125-.504 1.125-1.125V11.25c0-4.46-3.243-8.161-7.5-8.876a9.06 9.06 0 0 0-1.5-.124H9.375c-.621 0-1.125.504-1.125 1.125v3.5m7.5 10.375H9.375a1.125 1.125 0 0 1-1.125-1.125v-9.25m12 6.625v-1.875a3.375 3.375 0 0 0-3.375-3.375h-1.5a1.125 1.125 0 0 1-1.125-1.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H9.75" />
                </svg>
              ) : (
                <span className="overflow-hidden text-ellipsis whitespace-nowrap">
                  {g.name}
                </span>
              )}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
