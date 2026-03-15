import { useEffect, useState } from "react";

export function ShareRawButton({ fileId }: { fileId: string }) {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(timer);
  }, [copied]);

  const handleClick = async () => {
    try {
      const url = `${window.location.origin}/_/api/files/${fileId}/raw`;
      await navigator.clipboard.writeText(url);
      setCopied(true);
    } catch {
      // clipboard API may fail in insecure contexts
    }
  };

  return (
    <button
      type="button"
      className="flex items-center justify-center bg-transparent border border-gh-border rounded-md p-1.5 text-gh-text-secondary cursor-pointer transition-colors duration-150 hover:bg-gh-bg-hover"
      onClick={handleClick}
      title="Copy link to raw file content"
    >
      {copied ? (
        <svg className="size-5" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" d="m4.5 12.75 6 6 9-13.5" />
        </svg>
      ) : (
        <svg className="size-5" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" d="M13.19 8.688a4.5 4.5 0 0 1 1.242 7.244l-4.5 4.5a4.5 4.5 0 0 1-6.364-6.364l1.757-1.757m13.35-.622 1.757-1.757a4.5 4.5 0 0 0-6.364-6.364l-4.5 4.5a4.5 4.5 0 0 0 1.242 7.244" />
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 7l3-3m0 0h-3.5m3.5 0v3.5" />
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 17l-3 3m0 0h3.5m-3.5 0v-3.5" />
          <circle cx={12} cy={12} r={1.5} fill="currentColor" stroke="none" />
        </svg>
      )}
    </button>
  );
}
