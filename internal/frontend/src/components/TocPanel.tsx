import { useCallback, useEffect, useRef, useState } from "react";

export interface TocHeading {
  id: string;
  text: string;
  level: number;
}

interface TocPanelProps {
  headings: TocHeading[];
  activeHeadingId: string | null;
  onHeadingClick: (id: string) => void;
}

const MIN_WIDTH = 180;
const MAX_WIDTH = 480;
const DEFAULT_WIDTH = 240;
const STORAGE_KEY = "mo-toc-width";

function getInitialWidth(): number {
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored) {
    const n = parseInt(stored, 10);
    if (n >= MIN_WIDTH && n <= MAX_WIDTH) return n;
  }
  return DEFAULT_WIDTH;
}

const INDENT: Record<number, string> = {
  1: "pl-3",
  2: "pl-3",
  3: "pl-6",
  4: "pl-9",
  5: "pl-12",
  6: "pl-15",
};

export function TocPanel({ headings, activeHeadingId, onHeadingClick }: TocPanelProps) {
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
      const clamped = Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, window.innerWidth - e.clientX));
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
      className="relative shrink-0 bg-gh-bg-sidebar border-l border-gh-border flex flex-col overflow-y-auto"
      style={{ width }}
    >
      {/* Resize handle */}
      <div
        className="absolute top-0 left-0 w-1 h-full cursor-col-resize hover:bg-gh-border active:bg-gh-border transition-colors"
        onMouseDown={onMouseDown}
      />
      <nav className="flex flex-col pb-1">
        {headings.length === 0 ? (
          <div className="px-3 py-2 text-gh-text-secondary text-sm">
            No headings
          </div>
        ) : (
          headings.map((h) => (
            <button
              key={h.id}
              className={`flex items-center w-full ${INDENT[h.level] ?? "pl-3"} pr-3 py-1.5 border-none cursor-pointer text-left text-sm transition-colors duration-150 ${
                h.id === activeHeadingId
                  ? "bg-gh-bg-active text-gh-text font-semibold"
                  : "bg-transparent text-gh-text-secondary hover:bg-gh-bg-hover"
              }`}
              onClick={() => onHeadingClick(h.id)}
              title={h.text}
            >
              <span className="overflow-hidden text-ellipsis whitespace-nowrap">
                {h.text}
              </span>
            </button>
          ))
        )}
      </nav>
    </aside>
  );
}
