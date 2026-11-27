import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useHashNavigation } from "./useHashNavigation";

function addHeading(id: string) {
  const el = document.createElement("h2");
  el.id = id;
  el.scrollIntoView = vi.fn();
  document.body.appendChild(el);
  return el;
}

function makeContainer() {
  const el = document.createElement("div");
  el.scrollTo = vi.fn() as typeof el.scrollTo;
  return el;
}

// Browsers fire popstate and then hashchange after a traversal has already
// updated the URL.
function traverseTo(url: string) {
  window.history.replaceState(null, "", url);
  window.dispatchEvent(new PopStateEvent("popstate"));
  window.dispatchEvent(new HashChangeEvent("hashchange"));
}

function placeAt(el: HTMLElement, top: number) {
  el.getBoundingClientRect = () => ({ top }) as DOMRect;
}

// Mounts the hook as App does and reports the file's first content render.
function renderForFile(container: HTMLElement, fileId: string) {
  const hook = renderHook(({ id }) => useHashNavigation(container, "default", id), {
    initialProps: { id: fileId },
  });
  hook.result.current.onContentRendered();
  return hook;
}

beforeEach(() => {
  window.history.replaceState(null, "", "/?file=file1#intro");
});

afterEach(() => {
  document.body.innerHTML = "";
  window.history.replaceState(null, "", "/");
});

describe("useHashNavigation", () => {
  it("scrolls to the heading named by the new hash", () => {
    const heading = addHeading("usage");
    renderForFile(makeContainer(), "file1");

    traverseTo("/?file=file1#usage");

    expect(heading.scrollIntoView).toHaveBeenCalledWith({ behavior: "smooth", block: "start" });
  });

  it("decodes a percent-encoded hash before looking up the heading", () => {
    const heading = addHeading("使い方");
    renderForFile(makeContainer(), "file1");

    traverseTo(`/?file=file1#${encodeURIComponent("使い方")}`);

    expect(heading.scrollIntoView).toHaveBeenCalled();
  });

  it("scrolls the container to the top when the hash is removed", () => {
    const container = makeContainer();
    renderForFile(container, "file1");

    traverseTo("/?file=file1");

    expect(container.scrollTo).toHaveBeenCalledWith({ top: 0, behavior: "smooth" });
  });

  it("ignores a hash change that belongs to another file", () => {
    const heading = addHeading("usage");
    const container = makeContainer();
    renderForFile(container, "file1");

    traverseTo("/?file=file2#usage");

    expect(heading.scrollIntoView).not.toHaveBeenCalled();
    expect(container.scrollTo).not.toHaveBeenCalled();
  });

  it("follows the hash into another group that shows the same file", () => {
    const heading = addHeading("usage");
    renderForFile(makeContainer(), "file1");

    traverseTo("/design?file=file1#usage");

    expect(heading.scrollIntoView).toHaveBeenCalled();
  });

  it("stops listening after unmount", () => {
    const heading = addHeading("usage");
    const { unmount } = renderForFile(makeContainer(), "file1");
    unmount();

    traverseTo("/?file=file1#usage");

    expect(heading.scrollIntoView).not.toHaveBeenCalled();
  });

  it("lands on the heading after a traversal to another file once it renders", () => {
    const container = makeContainer();
    placeAt(container, 50);
    const { result, rerender } = renderForFile(container, "file1");

    traverseTo("/?file=file2#usage");
    rerender({ id: "file2" });
    placeAt(addHeading("usage"), 350);
    result.current.onContentRendered();

    expect(container.scrollTop).toBe(300);
  });

  it("lands on the heading when the file switch re-rendered before the popstate listener ran", () => {
    const container = makeContainer();
    placeAt(container, 50);
    const { result, rerender } = renderForFile(container, "file1");

    window.history.replaceState(null, "", "/?file=file2#usage");
    rerender({ id: "file2" });
    window.dispatchEvent(new PopStateEvent("popstate"));
    window.dispatchEvent(new HashChangeEvent("hashchange"));
    placeAt(addHeading("usage"), 350);
    result.current.onContentRendered();

    expect(container.scrollTop).toBe(300);
  });

  it("does not apply the heading to the file being left", () => {
    const container = makeContainer();
    placeAt(container, 0);
    const { result } = renderForFile(container, "file1");

    traverseTo("/?file=file2#usage");
    placeAt(addHeading("usage"), 300);
    result.current.onContentRendered();

    expect(container.scrollTop).toBe(0);
  });

  it("applies the heading only on the first render of the target file", () => {
    const container = makeContainer();
    placeAt(container, 0);
    const heading = addHeading("usage");
    const { result, rerender } = renderForFile(container, "file1");

    traverseTo("/?file=file2#usage");
    rerender({ id: "file2" });
    placeAt(heading, 300);
    result.current.onContentRendered();
    placeAt(heading, 100);
    result.current.onContentRendered();

    expect(container.scrollTop).toBe(300);
  });

  it("leaves a file reached without a hash at its own starting position", () => {
    const container = makeContainer();
    placeAt(container, 0);
    placeAt(addHeading("usage"), 300);
    const { result, rerender } = renderForFile(container, "file1");

    traverseTo("/?file=file2");
    rerender({ id: "file2" });
    result.current.onContentRendered();

    expect(container.scrollTop).toBe(0);
  });

  it("drops the pending heading when a later traversal stays on the active file", () => {
    const container = makeContainer();
    placeAt(container, 0);
    placeAt(addHeading("usage"), 300);
    const { result, rerender } = renderForFile(container, "file1");

    traverseTo("/?file=file2#usage");
    traverseTo("/?file=file1#intro");
    rerender({ id: "file2" });
    result.current.onContentRendered();

    expect(container.scrollTop).toBe(0);
  });

  it("lands on the heading after returning to a file whose content was replaced by a loading view", () => {
    const container = makeContainer();
    placeAt(container, 0);
    const heading = addHeading("usage");
    const { result, rerender } = renderForFile(container, "file1");

    // Selecting file2 unmounts file1's headings before file2 has rendered.
    window.history.pushState(null, "", "/?file=file2");
    rerender({ id: "file2" });
    heading.remove();
    traverseTo("/?file=file1#usage");
    rerender({ id: "file1" });
    placeAt(addHeading("usage"), 300);
    result.current.onContentRendered();

    expect(container.scrollTop).toBe(300);
  });

  it("drops the pending heading when sidebar navigation leaves the traversal", () => {
    const container = makeContainer();
    placeAt(container, 0);
    const { result, rerender } = renderForFile(container, "file1");

    traverseTo("/?file=file2#usage");
    window.history.pushState(null, "", "/?file=file3");
    rerender({ id: "file3" });
    result.current.onContentRendered();
    window.history.pushState(null, "", "/?file=file2");
    rerender({ id: "file2" });
    placeAt(addHeading("usage"), 300);
    result.current.onContentRendered();

    expect(container.scrollTop).toBe(0);
  });

  it("lands on the heading once a file opened with a hash renders", () => {
    const container = makeContainer();
    placeAt(container, 50);
    const { result, rerender } = renderForFile(container, "file1");

    window.history.pushState(null, "", "/?file=file2#usage");
    result.current.followHash("file2");
    result.current.onContentRendered();
    rerender({ id: "file2" });
    placeAt(addHeading("usage"), 350);
    result.current.onContentRendered();

    expect(container.scrollTop).toBe(300);
  });

  it("scrolls at once when the file opened with a hash is already shown", () => {
    const heading = addHeading("usage");
    const { result } = renderForFile(makeContainer(), "file1");

    window.history.pushState(null, "", "/?file=file1#usage");
    result.current.followHash("file1");

    expect(heading.scrollIntoView).toHaveBeenCalledWith({ behavior: "smooth", block: "start" });
  });

  it("follows hash changes after a group switch that kept the file and did not render again", () => {
    const heading = addHeading("usage");
    const container = makeContainer();
    const { result, rerender } = renderHook(
      ({ group }) => useHashNavigation(container, group, "file1"),
      { initialProps: { group: "default" } },
    );
    result.current.onContentRendered();

    window.history.pushState(null, "", "/design?file=file1#intro");
    rerender({ group: "design" });
    traverseTo("/design?file=file1#usage");

    expect(heading.scrollIntoView).toHaveBeenCalled();
  });
});
