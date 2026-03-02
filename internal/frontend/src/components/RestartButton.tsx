import { useCallback, useEffect, useState } from "react";
import { restartServer, fetchVersion, type VersionInfo } from "../hooks/useApi";

type Status = "idle" | "restarting";

export function RestartButton() {
  const [status, setStatus] = useState<Status>("idle");
  const [version, setVersion] = useState<VersionInfo | null>(null);

  useEffect(() => {
    fetchVersion().then(setVersion).catch(() => {});
  }, []);

  const handleClick = useCallback(async () => {
    if (status === "restarting") return;
    setStatus("restarting");

    try {
      await restartServer();
    } catch {
      // Server may close before responding
    }

    // Poll until the new server is ready
    const poll = () => {
      setTimeout(async () => {
        try {
          await fetchVersion();
          window.location.reload();
        } catch {
          poll();
        }
      }, 1000);
    };
    poll();
  }, [status]);

  const title = version
    ? `mo ${version.version} (${version.revision})\nClick to restart`
    : "Restart server";

  return (
    <>
      <button
        className="fixed bottom-4 right-4 flex items-center justify-center bg-transparent border border-gh-border rounded-md p-1.5 text-gh-text-secondary cursor-pointer transition-colors duration-150 hover:bg-gh-bg-hover disabled:opacity-50 disabled:cursor-not-allowed"
        onClick={handleClick}
        disabled={status === "restarting"}
        title={title}
      >
        <svg className={`size-5${status === "restarting" ? " animate-spin" : ""}`} fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.992 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182M20.016 4.356v4.992" />
        </svg>
      </button>
      {status === "restarting" && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-gh-bg/80">
          <div className="flex items-center gap-3 text-gh-text-secondary">
            <svg className="size-6 animate-spin" fill="none" stroke="currentColor" strokeWidth={1.5} viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.992 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182M20.016 4.356v4.992" />
            </svg>
            <span className="text-lg">Restarting...</span>
          </div>
        </div>
      )}
    </>
  );
}
