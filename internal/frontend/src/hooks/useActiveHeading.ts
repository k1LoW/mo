import { useEffect, useState } from "react";

export function useActiveHeading(
  headingIds: string[],
  scrollContainer: HTMLElement | null,
): string | null {
  const [activeId, setActiveId] = useState<string | null>(null);

  useEffect(() => {
    if (!scrollContainer || headingIds.length === 0) {
      setActiveId(null);
      return;
    }

    const elements = headingIds
      .map((id) => document.getElementById(id))
      .filter((el): el is HTMLElement => el !== null);

    if (elements.length === 0) return;

    const observer = new IntersectionObserver(
      (entries) => {
        const intersecting = new Set<string>();
        for (const entry of entries) {
          if (entry.isIntersecting) {
            intersecting.add(entry.target.id);
          }
        }

        // Find the topmost intersecting heading (in document order)
        for (const id of headingIds) {
          if (intersecting.has(id)) {
            setActiveId(id);
            return;
          }
        }
      },
      {
        root: scrollContainer,
        rootMargin: "0px 0px -80% 0px",
      },
    );

    for (const el of elements) {
      observer.observe(el);
    }

    return () => observer.disconnect();
  }, [headingIds, scrollContainer]);

  return activeId;
}
