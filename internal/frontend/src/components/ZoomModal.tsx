import { useEffect, useCallback, useRef, useState } from "react";
import { TransformWrapper, TransformComponent } from "react-zoom-pan-pinch";

export type ZoomContent =
  | { type: "image"; src: string; alt?: string }
  | { type: "svg"; svg: string };

interface ZoomModalProps {
  content: ZoomContent;
  onClose: () => void;
}

export function ZoomModal({ content, onClose }: ZoomModalProps) {
  const [initialScale, setInitialScale] = useState<number | null>(null);
  const contentRef = useRef<HTMLDivElement>(null);

  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    },
    [onClose],
  );

  useEffect(() => {
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [handleKeyDown]);

  // Measure actual rendered content size and calculate fit scale
  useEffect(() => {
    // Wait one frame for the hidden content to render and get measured
    const raf = requestAnimationFrame(() => {
      const el = contentRef.current;
      if (!el) {
        setInitialScale(1);
        return;
      }
      const contentW = el.scrollWidth;
      const contentH = el.scrollHeight;
      if (contentW <= 0 || contentH <= 0) {
        setInitialScale(1);
        return;
      }
      const vw = window.innerWidth * 0.85;
      const vh = window.innerHeight * 0.85;
      const scale = Math.min(vw / contentW, vh / contentH);
      setInitialScale(scale);
    });
    return () => cancelAnimationFrame(raf);
  }, [content]);

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) onClose();
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/70"
      onClick={handleBackdropClick}
    >
      <button
        type="button"
        className="absolute top-4 right-4 z-10 flex items-center justify-center rounded-md p-2 cursor-pointer text-white/70 hover:text-white transition-colors duration-150"
        onClick={onClose}
        aria-label="Close"
      >
        <svg
          className="size-6"
          fill="none"
          stroke="currentColor"
          strokeWidth={2}
          viewBox="0 0 24 24"
        >
          <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
      {/* Hidden content for measuring actual size */}
      {initialScale === null && (
        <div
          ref={contentRef}
          style={{ position: "absolute", visibility: "hidden", pointerEvents: "none" }}
        >
          {content.type === "image" ? (
            <img
              src={content.src}
              alt={content.alt ?? ""}
              onLoad={() => {
                const el = contentRef.current;
                if (el) {
                  const vw = window.innerWidth * 0.85;
                  const vh = window.innerHeight * 0.85;
                  setInitialScale(Math.min(vw / el.scrollWidth, vh / el.scrollHeight));
                }
              }}
            />
          ) : (
            <div dangerouslySetInnerHTML={{ __html: content.svg }} />
          )}
        </div>
      )}
      {initialScale !== null && (
        <TransformWrapper
          initialScale={initialScale}
          minScale={initialScale * 0.5}
          maxScale={Math.max(initialScale * 10, 10)}
          wheel={{ step: 0.1 }}
          doubleClick={{ disabled: true }}
          centerOnInit
        >
          <TransformComponent
            wrapperStyle={{ width: "100%", height: "100%" }}
            contentStyle={{
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            {content.type === "image" ? (
              <img src={content.src} alt={content.alt ?? ""} draggable={false} />
            ) : (
              <div dangerouslySetInnerHTML={{ __html: content.svg }} />
            )}
          </TransformComponent>
        </TransformWrapper>
      )}
    </div>
  );
}
