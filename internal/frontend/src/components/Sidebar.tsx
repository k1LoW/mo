import { useCallback, useEffect, useRef, useState } from "react";
import type { Group } from "../hooks/useApi";

const MIN_WIDTH = 180;
const MAX_WIDTH = 480;
const DEFAULT_WIDTH = 260;
const STORAGE_KEY = "mo-sidebar-width";

function getInitialWidth(): number {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored) {
    const n = parseInt(stored, 10);
    if (n >= MIN_WIDTH && n <= MAX_WIDTH) return n;
  }
  return DEFAULT_WIDTH;
}

interface SidebarProps {
  groups: Group[];
  activeGroup: string;
  activeFileId: number | null;
  onFileSelect: (id: number) => void;
}

export function Sidebar({
  groups,
  activeGroup,
  activeFileId,
  onFileSelect,
}: SidebarProps) {
  const currentGroup = groups.find((g) => g.name === activeGroup);
  const files = currentGroup?.files ?? [];
  const [width, setWidth] = useState(getInitialWidth);
  const dragging = useRef(false);

  const onMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    dragging.current = true;
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
  }, []);

  useEffect(() => {
    const onMouseMove = (e: MouseEvent) => {
      if (!dragging.current) return;
      const clamped = Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, e.clientX));
      setWidth(clamped);
    };
    const onMouseUp = () => {
      if (!dragging.current) return;
      dragging.current = false;
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
    };
    document.addEventListener("mousemove", onMouseMove);
    document.addEventListener("mouseup", onMouseUp);
    return () => {
      document.removeEventListener("mousemove", onMouseMove);
      document.removeEventListener("mouseup", onMouseUp);
    };
  }, []);

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, String(width));
  }, [width]);

  return (
    <aside
      className="relative bg-gh-bg-sidebar border-r border-gh-border flex flex-col overflow-y-auto shrink-0"
      style={{ width }}
    >
      <nav className="flex flex-col py-1">
        {files.map((f) => (
          <button
            key={f.id}
            className={`flex items-center gap-2 w-full px-3 py-2 border-none cursor-pointer text-left text-sm transition-colors duration-150 ${
              f.id === activeFileId
                ? "bg-gh-bg-active text-gh-text font-semibold"
                : "bg-transparent text-gh-text-secondary hover:bg-gh-bg-hover"
            }`}
            onClick={() => onFileSelect(f.id)}
            title={f.name}
          >
            <svg className="size-4 shrink-0" viewBox="0 0 16 16" fill="currentColor">
              <path d="M2 1.75C2 .784 2.784 0 3.75 0h6.586c.464 0 .909.184 1.237.513l2.914 2.914c.329.328.513.773.513 1.237v9.586A1.75 1.75 0 0 1 13.25 16h-9.5A1.75 1.75 0 0 1 2 14.25Zm1.75-.25a.25.25 0 0 0-.25.25v12.5c0 .138.112.25.25.25h9.5a.25.25 0 0 0 .25-.25V6h-2.75A1.75 1.75 0 0 1 9 4.25V1.5Zm6.75.062V4.25c0 .138.112.25.25.25h2.688l-.011-.013-2.914-2.914-.013-.011Z" />
            </svg>
            <span className="overflow-hidden text-ellipsis whitespace-nowrap">
              {f.name}
            </span>
          </button>
        ))}
      </nav>
      {/* Resize handle */}
      <div
        className="absolute top-0 right-0 w-1 h-full cursor-col-resize hover:bg-gh-border active:bg-gh-border transition-colors"
        onMouseDown={onMouseDown}
      />
    </aside>
  );
}
