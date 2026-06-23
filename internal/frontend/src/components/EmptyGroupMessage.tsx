import { useEffect, useState } from "react";
import type { Group } from "../hooks/useApi";

interface EmptyGroupMessageProps {
  group: Group | undefined;
}

function shellQuote(s: string): string {
  if (/^[a-zA-Z0-9_\-./@]+$/.test(s)) return s;
  return `'${s.replace(/'/g, "'\\''")}'`;
}

const DEFAULT_PORT = "6275";

function buildUnwatchCommand(pattern: string, groupName: string, port: string): string {
  const groupFlag = groupName && groupName !== "default" ? ` -t ${shellQuote(groupName)}` : "";
  const portFlag = port && port !== DEFAULT_PORT ? ` -p ${port}` : "";
  return `mo --unwatch ${shellQuote(pattern)}${groupFlag}${portFlag}`;
}

interface CommandRowProps {
  command: string;
}

function CommandRow({ command }: CommandRowProps) {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(timer);
  }, [copied]);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
    } catch {
      // clipboard API may fail in insecure contexts
    }
  };

  return (
    <div className="flex items-center gap-2 w-full">
      <code className="flex-1 px-3 py-2 bg-gh-bg-secondary border border-gh-border rounded text-xs text-gh-text font-mono overflow-x-auto whitespace-pre">
        $ {command}
      </code>
      <button
        type="button"
        onClick={handleCopy}
        className="shrink-0 flex items-center justify-center bg-transparent border border-gh-border rounded-md p-1.5 text-gh-text-secondary cursor-pointer transition-colors duration-150 hover:bg-gh-bg-hover"
        title="Copy command"
        aria-label={copied ? "Command copied" : "Copy command"}
      >
        {copied ? (
          <svg
            className="size-4"
            fill="none"
            stroke="currentColor"
            strokeWidth={2}
            viewBox="0 0 24 24"
          >
            <path strokeLinecap="round" strokeLinejoin="round" d="m4.5 12.75 6 6 9-13.5" />
          </svg>
        ) : (
          <svg
            className="size-4"
            fill="none"
            stroke="currentColor"
            strokeWidth={1.5}
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M15.666 3.888A2.25 2.25 0 0 0 13.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 0 1-.75.75H9.75a.75.75 0 0 1-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 0 1-2.25 2.25H6.75A2.25 2.25 0 0 1 4.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 0 1 1.927-.184"
            />
          </svg>
        )}
      </button>
    </div>
  );
}

export function EmptyGroupMessage({ group }: EmptyGroupMessageProps) {
  const patterns = group?.patterns ?? [];
  const groupName = group?.name ?? "";

  if (patterns.length === 0) {
    return (
      <div className="flex items-center justify-center h-50 text-gh-text-secondary text-sm">
        No file selected
      </div>
    );
  }

  const multiple = patterns.length > 1;
  const port = typeof window !== "undefined" ? window.location.port : "";

  return (
    <div className="flex flex-col items-start max-w-2xl mx-auto mt-12 p-6 text-gh-text-secondary text-sm gap-4">
      <p className="text-gh-text font-semibold">No file in this group.</p>
      <p>
        This group is kept alive by the following watch pattern
        {multiple ? "s" : ""}. Run the command{multiple ? "s" : ""} below to unwatch{" "}
        {multiple ? "them" : "it"}. The group will disappear once all patterns are removed.
      </p>
      <div className="flex flex-col gap-2 w-full">
        {patterns.map((p) => (
          <CommandRow key={p} command={buildUnwatchCommand(p, groupName, port)} />
        ))}
      </div>
    </div>
  );
}
