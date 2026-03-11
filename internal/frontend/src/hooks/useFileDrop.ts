import { useCallback, useEffect, useRef, useState } from "react";
import { uploadFile } from "./useApi";

function hasFiles(e: DragEvent): boolean {
  return e.dataTransfer?.types.includes("Files") ?? false;
}

export function useFileDrop(activeGroup: string): { isDragging: boolean } {
  const [isDragging, setIsDragging] = useState(false);
  const dragCounter = useRef(0);

  const handleDragEnter = useCallback((e: DragEvent) => {
    if (!hasFiles(e)) return;
    e.preventDefault();
    dragCounter.current++;
    if (dragCounter.current === 1) {
      setIsDragging(true);
    }
  }, []);

  const handleDragOver = useCallback((e: DragEvent) => {
    if (!hasFiles(e)) return;
    e.preventDefault();
  }, []);

  const handleDragLeave = useCallback((e: DragEvent) => {
    if (!hasFiles(e)) return;
    e.preventDefault();
    if (dragCounter.current > 0) {
      dragCounter.current--;
      if (dragCounter.current === 0) {
        setIsDragging(false);
      }
    }
  }, []);

  const handleDrop = useCallback(
    async (e: DragEvent) => {
      if (!hasFiles(e)) return;
      e.preventDefault();
      dragCounter.current = 0;
      setIsDragging(false);

      if (!e.dataTransfer) return;

      const maxSize = 10 * 1024 * 1024; // 10MB
      const fileList = e.dataTransfer.files;
      const uploads: Promise<void>[] = [];
      for (let i = 0; i < fileList.length; i++) {
        const file = fileList[i];
        if (file.size <= maxSize) {
          uploads.push(
            file
              .text()
              .then((content) => uploadFile(file.name, content, activeGroup))
              .catch(() => {}),
          );
        }
      }
      await Promise.all(uploads);
    },
    [activeGroup],
  );

  useEffect(() => {
    document.addEventListener("dragenter", handleDragEnter);
    document.addEventListener("dragover", handleDragOver);
    document.addEventListener("dragleave", handleDragLeave);
    document.addEventListener("drop", handleDrop);
    return () => {
      document.removeEventListener("dragenter", handleDragEnter);
      document.removeEventListener("dragover", handleDragOver);
      document.removeEventListener("dragleave", handleDragLeave);
      document.removeEventListener("drop", handleDrop);
    };
  }, [handleDragEnter, handleDragOver, handleDragLeave, handleDrop]);

  return { isDragging };
}
