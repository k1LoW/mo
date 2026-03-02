import { useEffect, useRef } from "react";

interface SSECallbacks {
  onUpdate: () => void;
  onFileChanged?: (fileId: number) => void;
}

export function useSSE(callbacks: SSECallbacks) {
  const callbacksRef = useRef(callbacks);
  callbacksRef.current = callbacks;

  useEffect(() => {
    let disposed = false;
    let es: EventSource | null = null;

    function connect() {
      if (disposed) return;

      es = new EventSource("/_/events");

      es.addEventListener("update", () => {
        callbacksRef.current.onUpdate();
      });

      es.addEventListener("file-changed", (e) => {
        try {
          const data = JSON.parse(e.data);
          callbacksRef.current.onFileChanged?.(data.id);
        } catch {
          // ignore malformed data
        }
      });

      es.onerror = () => {
        es?.close();
        if (!disposed) {
          setTimeout(connect, 2000);
        }
      };
    }

    connect();

    return () => {
      disposed = true;
      es?.close();
    };
  }, []);
}
