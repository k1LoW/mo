import { describe, expect, it } from "vitest";
import {
  addPane,
  buildPanesUrl,
  closePane,
  EMPTY_PANES,
  focusedFileId,
  focusPane,
  MAX_PANES,
  parsePanesFromSearch,
  reconcilePanes,
  setPaneFile,
  type PaneState,
} from "./panes";

function panes(fileIds: string[], focusIndex = 0): PaneState {
  return { fileIds, focusIndex };
}

describe("parsePanesFromSearch", () => {
  it("returns null when no file params are present", () => {
    expect(parsePanesFromSearch("")).toBeNull();
    expect(parsePanesFromSearch("?q=hello")).toBeNull();
  });

  it("parses the legacy single-file form", () => {
    expect(parsePanesFromSearch("?file=aaa11111")).toEqual(panes(["aaa11111"]));
  });

  it("parses a multi-pane layout", () => {
    expect(parsePanesFromSearch("?files=aaa11111,bbb22222,ccc33333")).toEqual(
      panes(["aaa11111", "bbb22222", "ccc33333"]),
    );
  });

  it("parses the focused pane index", () => {
    expect(parsePanesFromSearch("?files=aaa11111,bbb22222&focus=1")).toEqual(
      panes(["aaa11111", "bbb22222"], 1),
    );
  });

  it("clamps a focus index that points past the last pane", () => {
    expect(parsePanesFromSearch("?files=aaa11111,bbb22222&focus=9")).toEqual(
      panes(["aaa11111", "bbb22222"], 1),
    );
  });

  it("falls back to the first pane for a non-numeric or negative focus", () => {
    expect(parsePanesFromSearch("?files=aaa11111,bbb22222&focus=abc")).toEqual(
      panes(["aaa11111", "bbb22222"], 0),
    );
    expect(parsePanesFromSearch("?files=aaa11111,bbb22222&focus=-3")).toEqual(
      panes(["aaa11111", "bbb22222"], 0),
    );
  });

  it("drops empty entries produced by stray commas", () => {
    expect(parsePanesFromSearch("?files=,aaa11111,,bbb22222,")).toEqual(
      panes(["aaa11111", "bbb22222"]),
    );
  });

  it("caps the number of panes at MAX_PANES", () => {
    const ids = Array.from({ length: MAX_PANES + 3 }, (_, i) => `id${i}`);
    const parsed = parsePanesFromSearch(`?files=${ids.join(",")}`);
    expect(parsed?.fileIds).toHaveLength(MAX_PANES);
  });

  it("prefers ?files= over a stale ?file=", () => {
    expect(parsePanesFromSearch("?file=zzz99999&files=aaa11111,bbb22222")).toEqual(
      panes(["aaa11111", "bbb22222"]),
    );
  });

  it("ignores an empty ?files= and falls through to ?file=", () => {
    expect(parsePanesFromSearch("?files=&file=aaa11111")).toEqual(panes(["aaa11111"]));
  });
});

describe("buildPanesUrl", () => {
  it("returns the bare group path when nothing is open", () => {
    expect(buildPanesUrl("default", EMPTY_PANES)).toBe("/");
    expect(buildPanesUrl("design", EMPTY_PANES)).toBe("/design");
  });

  it("emits the legacy ?file= form for a single pane", () => {
    expect(buildPanesUrl("default", panes(["aaa11111"]))).toBe("/?file=aaa11111");
    expect(buildPanesUrl("design", panes(["aaa11111"]))).toBe("/design?file=aaa11111");
  });

  it("emits ?files= for multiple panes", () => {
    expect(buildPanesUrl("default", panes(["aaa11111", "bbb22222"]))).toBe(
      "/?files=aaa11111,bbb22222",
    );
  });

  it("omits focus=0 and includes any other focus", () => {
    expect(buildPanesUrl("default", panes(["aaa11111", "bbb22222"], 0))).toBe(
      "/?files=aaa11111,bbb22222",
    );
    expect(buildPanesUrl("default", panes(["aaa11111", "bbb22222"], 1))).toBe(
      "/?files=aaa11111,bbb22222&focus=1",
    );
  });

  it("round-trips through parsePanesFromSearch", () => {
    const original = panes(["aaa11111", "bbb22222", "ccc33333"], 2);
    const url = buildPanesUrl("design", original);
    expect(parsePanesFromSearch(url.slice(url.indexOf("?")))).toEqual(original);
  });
});

describe("reconcilePanes", () => {
  const available = new Set(["aaa11111", "bbb22222", "ccc33333"]);

  it("returns the same object when every pane is still available", () => {
    const state = panes(["aaa11111", "bbb22222"], 1);
    expect(reconcilePanes(state, available)).toBe(state);
  });

  it("drops panes whose file left the group", () => {
    expect(reconcilePanes(panes(["aaa11111", "zzz99999", "ccc33333"], 0), available)).toEqual(
      panes(["aaa11111", "ccc33333"], 0),
    );
  });

  it("keeps the focus on the same document when an earlier pane is dropped", () => {
    const state = panes(["zzz99999", "bbb22222"], 1);
    const result = reconcilePanes(state, available);
    expect(result.fileIds).toEqual(["bbb22222"]);
    expect(focusedFileId(result)).toBe("bbb22222");
  });

  it("clamps the focus when the focused pane itself is dropped", () => {
    expect(reconcilePanes(panes(["aaa11111", "zzz99999"], 1), available)).toEqual(
      panes(["aaa11111"], 0),
    );
  });

  it("empties the layout when no pane survives", () => {
    expect(reconcilePanes(panes(["zzz99999"], 0), available)).toEqual(panes([], 0));
  });

  it("keeps the focus index when the same file is open in two panes", () => {
    // indexOf would snap the focus onto the leftmost copy of the file.
    const state = panes(["aaa11111", "aaa11111", "bbb22222"], 1);
    expect(reconcilePanes(state, available)).toBe(state);
  });

  it("clamps a focus index that points past the last pane", () => {
    expect(reconcilePanes(panes(["aaa11111"], 4), available)).toEqual(panes(["aaa11111"], 0));
  });
});

describe("setPaneFile", () => {
  it("opens the file as the only pane when nothing is open yet", () => {
    expect(setPaneFile(EMPTY_PANES, 0, "aaa11111")).toEqual(panes(["aaa11111"]));
  });

  it("replaces the file in the target pane and focuses it", () => {
    expect(setPaneFile(panes(["aaa11111", "bbb22222"], 0), 1, "ccc33333")).toEqual(
      panes(["aaa11111", "ccc33333"], 1),
    );
  });

  it("only moves the focus when the pane already shows that file", () => {
    expect(setPaneFile(panes(["aaa11111", "bbb22222"], 0), 1, "bbb22222")).toEqual(
      panes(["aaa11111", "bbb22222"], 1),
    );
  });

  it("returns the same object for an out-of-range pane", () => {
    const state = panes(["aaa11111"]);
    expect(setPaneFile(state, 3, "bbb22222")).toBe(state);
    expect(setPaneFile(state, -1, "bbb22222")).toBe(state);
  });
});

describe("addPane", () => {
  it("appends a column and focuses it", () => {
    expect(addPane(panes(["aaa11111"]), "bbb22222")).toEqual(panes(["aaa11111", "bbb22222"], 1));
  });

  it("opens the first pane when nothing is open", () => {
    expect(addPane(EMPTY_PANES, "aaa11111")).toEqual(panes(["aaa11111"]));
  });

  it("focuses the existing column instead of duplicating the file", () => {
    expect(addPane(panes(["aaa11111", "bbb22222"], 1), "aaa11111")).toEqual(
      panes(["aaa11111", "bbb22222"], 0),
    );
  });

  it("leaves the layout untouched once MAX_PANES columns are open", () => {
    const state = panes(Array.from({ length: MAX_PANES }, (_, i) => `id${i}`));
    expect(addPane(state, "another")).toBe(state);
  });
});

describe("closePane", () => {
  it("removes the column", () => {
    expect(closePane(panes(["aaa11111", "bbb22222", "ccc33333"], 0), 1)).toEqual(
      panes(["aaa11111", "ccc33333"], 0),
    );
  });

  it("keeps the focus on the same document when an earlier column closes", () => {
    const result = closePane(panes(["aaa11111", "bbb22222", "ccc33333"], 2), 0);
    expect(result.fileIds).toEqual(["bbb22222", "ccc33333"]);
    expect(focusedFileId(result)).toBe("ccc33333");
  });

  it("keeps the focus index when a later column closes", () => {
    const result = closePane(panes(["aaa11111", "bbb22222", "ccc33333"], 0), 2);
    expect(result.fileIds).toEqual(["aaa11111", "bbb22222"]);
    expect(focusedFileId(result)).toBe("aaa11111");
  });

  it("clamps the focus when the last column is the focused one", () => {
    expect(closePane(panes(["aaa11111", "bbb22222"], 1), 1)).toEqual(panes(["aaa11111"], 0));
  });

  it("returns the same object for an out-of-range index", () => {
    const state = panes(["aaa11111"]);
    expect(closePane(state, 4)).toBe(state);
  });
});

describe("focusPane", () => {
  it("moves the focus", () => {
    expect(focusPane(panes(["aaa11111", "bbb22222"], 0), 1)).toEqual(
      panes(["aaa11111", "bbb22222"], 1),
    );
  });

  it("returns the same object when the pane is already focused or out of range", () => {
    const state = panes(["aaa11111", "bbb22222"], 1);
    expect(focusPane(state, 1)).toBe(state);
    expect(focusPane(state, 5)).toBe(state);
  });
});

describe("focusedFileId", () => {
  it("returns the focused pane's file", () => {
    expect(focusedFileId(panes(["aaa11111", "bbb22222"], 1))).toBe("bbb22222");
  });

  it("returns null when nothing is open", () => {
    expect(focusedFileId(EMPTY_PANES)).toBeNull();
  });
});
