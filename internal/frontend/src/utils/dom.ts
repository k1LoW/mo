/**
 * Finds an element by ID within a subtree.
 *
 * Split panes render several documents at once, and heading IDs come from the
 * heading text (rehype-slug), so the same ID legitimately exists in more than
 * one column. `document.getElementById` would always return the leftmost
 * column's copy, which makes every pane scroll pane 0. Scoping the lookup to
 * the pane's own container is what keeps the columns independent.
 */
export function findElementById(root: ParentNode | null, id: string): HTMLElement | null {
  if (root == null || id === "") return null;
  return root.querySelector<HTMLElement>(`[id="${CSS.escape(id)}"]`);
}

/**
 * Height reserved for the sticky file-name bar pinned to the top of a pane, so
 * a heading scrolled to does not end up hidden behind it.
 */
const STICKY_LABEL_HEIGHT = 40;

/**
 * Scrolls `el` to the top of `container`.
 *
 * Deliberately not `el.scrollIntoView({behavior: "smooth"})`: in a pane the
 * scroll container sits under several `overflow: hidden` ancestors, and Chrome
 * silently does nothing for a smooth scrollIntoView in that situation (an
 * instant one still works). Driving the container's own scrollTo avoids the
 * quirk, and also lets us offset the sticky label — which scrollIntoView cannot
 * do without a `scroll-margin-top` rule it would then have to honour.
 */
export function scrollElementToTop(container: HTMLElement | null, el: HTMLElement | null): void {
  if (container == null || el == null) return;

  const offsetWithinContent =
    container.scrollTop + el.getBoundingClientRect().top - container.getBoundingClientRect().top;
  const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  container.scrollTo({
    top: Math.max(0, offsetWithinContent - STICKY_LABEL_HEIGHT),
    behavior: reduced ? "auto" : "smooth",
  });
}
