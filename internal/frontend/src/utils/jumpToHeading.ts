// Scroll to a heading by id and reflect it in the URL hash so the location can
// be copied as a deep link, matching how in-document anchor links behave.
export function jumpToHeading(id: string): boolean {
  const el = document.getElementById(id);
  if (!el) return false;

  const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  el.scrollIntoView({ behavior: reduced ? "auto" : "smooth", block: "start" });
  window.history.pushState(null, "", `#${id}`);
  return true;
}
