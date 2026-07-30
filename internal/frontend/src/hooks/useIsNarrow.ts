import { useCallback, useEffect, useState } from "react";

/**
 * Below this, three columns would each be under ~250px and unreadable, so the
 * layout collapses to the focused pane alone.
 */
export const NARROW_BREAKPOINT = 900;

export function useIsNarrow(breakpoint = NARROW_BREAKPOINT): boolean {
  const query = `(max-width: ${breakpoint - 1}px)`;
  const evaluate = useCallback(() => {
    if (typeof window.matchMedia !== "function") return false;
    return window.matchMedia(query).matches;
  }, [query]);

  const [isNarrow, setIsNarrow] = useState(evaluate);

  useEffect(() => {
    if (typeof window.matchMedia !== "function") return;
    const list = window.matchMedia(query);
    const onChange = () => setIsNarrow(list.matches);
    onChange();
    list.addEventListener("change", onChange);
    return () => list.removeEventListener("change", onChange);
  }, [query]);

  return isNarrow;
}
