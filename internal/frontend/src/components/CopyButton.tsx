import { useEffect, useRef, useState } from "react";

interface CopyButtonProps {
  content: string;
}

type CopyFormat = "markdown" | "text" | "html";

const formats: { key: CopyFormat; label: string }[] = [
  { key: "markdown", label: "Copy as Markdown" },
  { key: "text", label: "Copy as Text" },
  { key: "html", label: "Copy as HTML" },
];

export function CopyButton({ content }: CopyButtonProps) {
  const [open, setOpen] = useState(false);
  const [copied, setCopied] = useState(false);
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

  const handleCopy = async (format: CopyFormat) => {
    setOpen(false);
    try {
      if (format === "markdown") {
        await navigator.clipboard.writeText(content);
      } else if (format === "text") {
        const el = document.querySelector<HTMLElement>(".markdown-body");
        const text = el ? el.innerText : content;
        await navigator.clipboard.writeText(text);
      } else {
        const el = document.querySelector<HTMLElement>(".markdown-body");
        const html = el ? el.innerHTML : content;
        const blob = new Blob([html], { type: "text/html" });
        const textBlob = new Blob([el ? el.innerText : content], { type: "text/plain" });
        await navigator.clipboard.write([
          new ClipboardItem({
            "text/html": blob,
            "text/plain": textBlob,
          }),
        ]);
      }
      setCopied(true);
    } catch {
      // clipboard API may fail in insecure contexts
    }
  };

  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(timer);
  }, [copied]);

  return (
    <div ref={ref} className="relative">
      <button
        className="flex items-center justify-center bg-transparent border border-gh-border rounded-md p-1.5 text-gh-text-secondary cursor-pointer transition-colors duration-150 hover:bg-gh-bg-hover"
        onClick={() => setOpen((v) => !v)}
        title="Copy content"
      >
        {copied ? (
          <svg className="size-5" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" d="m4.5 12.75 6 6 9-13.5" />
          </svg>
        ) : (
          <svg className="size-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" d="M15.666 3.888A2.25 2.25 0 0 0 13.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 0 1-.75.75H9.75a.75.75 0 0 1-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 0 1-2.25 2.25H6.75A2.25 2.25 0 0 1 4.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 0 1 1.927-.184" />
          </svg>
        )}
      </button>
      {open && (
        <div className="absolute right-0 top-full mt-1 min-w-44 bg-gh-bg-sidebar border border-gh-border rounded-md shadow-lg z-10 py-1">
          {formats.map((f) => (
            <button
              key={f.key}
              className="flex items-center w-full px-3 py-1.5 border-none cursor-pointer text-left text-xs bg-transparent text-gh-text-secondary hover:bg-gh-bg-hover transition-colors duration-150"
              onClick={() => handleCopy(f.key)}
            >
              {f.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
