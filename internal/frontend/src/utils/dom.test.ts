import { afterEach, describe, expect, it, vi } from "vitest";
import { findElementById, scrollElementToTop } from "./dom";

describe("findElementById", () => {
  afterEach(() => {
    document.body.innerHTML = "";
  });

  it("returns null for a null root or an empty id", () => {
    expect(findElementById(null, "heading")).toBeNull();
    expect(findElementById(document, "")).toBeNull();
  });

  it("returns null when nothing matches", () => {
    document.body.innerHTML = `<div id="other"></div>`;
    expect(findElementById(document, "heading")).toBeNull();
  });

  it("finds the element inside the given root", () => {
    document.body.innerHTML = `<div id="pane"><h2 id="overview">Overview</h2></div>`;
    const pane = document.getElementById("pane");
    expect(findElementById(pane, "overview")?.textContent).toBe("Overview");
  });

  it("resolves duplicated ids per root instead of always hitting the first", () => {
    document.body.innerHTML = `
      <div id="pane-0"><h2 id="overview">v1</h2></div>
      <div id="pane-1"><h2 id="overview">v2</h2></div>
    `;
    expect(findElementById(document.getElementById("pane-0"), "overview")?.textContent).toBe("v1");
    expect(findElementById(document.getElementById("pane-1"), "overview")?.textContent).toBe("v2");
  });

  it("does not throw on ids that are not valid CSS identifiers", () => {
    document.body.innerHTML = `<div id="pane"><h2 id="1.2 概要">Heading</h2></div>`;
    const pane = document.getElementById("pane");
    expect(findElementById(pane, "1.2 概要")?.textContent).toBe("Heading");
  });
});

describe("scrollElementToTop", () => {
  function makeContainer(scrollTop: number, containerTop: number) {
    const el = document.createElement("div");
    Object.defineProperty(el, "scrollTop", { value: scrollTop, writable: true });
    el.scrollTo = vi.fn();
    el.getBoundingClientRect = () => ({ top: containerTop }) as DOMRect;
    return el;
  }

  function makeHeading(top: number) {
    const el = document.createElement("h2");
    el.getBoundingClientRect = () => ({ top }) as DOMRect;
    return el;
  }

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("does nothing when the container or element is missing", () => {
    const container = makeContainer(0, 0);
    scrollElementToTop(container, null);
    scrollElementToTop(null, makeHeading(100));
    expect(container.scrollTo).not.toHaveBeenCalled();
  });

  it("scrolls the container itself rather than calling scrollIntoView", () => {
    // Heading sits 500px below the container's top edge, container already at 200.
    const container = makeContainer(200, 50);
    scrollElementToTop(container, makeHeading(550));

    // 200 + (550 - 50) = 700 content offset, minus the 40px sticky label.
    expect(container.scrollTo).toHaveBeenCalledWith({ top: 660, behavior: "smooth" });
  });

  it("never scrolls to a negative offset", () => {
    const container = makeContainer(0, 0);
    scrollElementToTop(container, makeHeading(10));
    expect(container.scrollTo).toHaveBeenCalledWith({ top: 0, behavior: "smooth" });
  });

  it("skips the animation when the reader prefers reduced motion", () => {
    vi.spyOn(window, "matchMedia").mockReturnValue({ matches: true } as MediaQueryList);
    const container = makeContainer(0, 0);
    scrollElementToTop(container, makeHeading(300));
    expect(container.scrollTo).toHaveBeenCalledWith({ top: 260, behavior: "auto" });
  });
});
