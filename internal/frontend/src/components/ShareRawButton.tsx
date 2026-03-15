import { useEffect, useState } from "react";
import { CheckIcon } from "./icons/CheckIcon";
import { ShareRawIcon } from "./icons/ShareRawIcon";

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
      {copied ? <CheckIcon /> : <ShareRawIcon />}
    </button>
  );
}
