import { useEffect, useRef } from "react";

interface SSECallbacks {
  onUpdate: () => void;
  onFileChanged?: (fileId: number) => void;
}

export function useSSE(callbacks: SSECallbacks) {
  const callbacksRef = useRef(callbacks);
  callbacksRef.current = callbacks;

  useEffect(() => {
    const es = new EventSource("/_/events");

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
      es.close();
      setTimeout(() => {
        callbacksRef.current.onUpdate();
      }, 2000);
    };

    return () => es.close();
  }, []);
}
