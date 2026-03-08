import { describe, it, expect, vi, beforeEach } from "vitest";
import { fetchGroups, fetchFileContent, openRelativeFile, reorderFiles, moveFile, uploadFile } from "./useApi";

beforeEach(() => {
  vi.restoreAllMocks();
});

describe("fetchGroups", () => {
  it("returns groups on success", async () => {
    const data = [{ name: "default", files: [{ id: "abc12345", name: "a.md", path: "/a.md" }] }];
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(data),
    }));

    const result = await fetchGroups();
    expect(result).toEqual(data);
    expect(fetch).toHaveBeenCalledWith("/_/api/groups");
  });

  it("throws on error response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
    }));

    await expect(fetchGroups()).rejects.toThrow("Failed to fetch groups");
  });
});

describe("fetchFileContent", () => {
  it("fetches content with correct URL", async () => {
    const data = { content: "# Hello", baseDir: "/tmp" };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(data),
    }));

    const result = await fetchFileContent("abc12345");
    expect(result).toEqual(data);
    expect(fetch).toHaveBeenCalledWith("/_/api/files/abc12345/content");
  });

  it("throws on error response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
    }));

    await expect(fetchFileContent("nonexist")).rejects.toThrow("Failed to fetch file content");
  });
});

describe("openRelativeFile", () => {
  it("sends POST with correct body", async () => {
    const entry = { id: "eee55555", name: "other.md", path: "/other.md" };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(entry),
    }));

    const result = await openRelativeFile("ccc33333", "./other.md");
    expect(result).toEqual(entry);
    expect(fetch).toHaveBeenCalledWith("/_/api/files/open", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ fileId: "ccc33333", path: "./other.md" }),
    });
  });

  it("throws on error response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
    }));

    await expect(openRelativeFile("aaa11111", "missing.md")).rejects.toThrow("Failed to open file");
  });
});

describe("reorderFiles", () => {
  it("sends PUT with correct URL and body", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
    }));

    await reorderFiles("default", ["ccc", "aaa", "bbb"]);
    expect(fetch).toHaveBeenCalledWith("/_/api/reorder", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ group: "default", fileIds: ["ccc", "aaa", "bbb"] }),
    });
  });

  it("sends group name in body for slash-containing names", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
    }));

    await reorderFiles("api/docs", ["aaa", "bbb"]);
    expect(fetch).toHaveBeenCalledWith("/_/api/reorder", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ group: "api/docs", fileIds: ["aaa", "bbb"] }),
    });
  });

  it("throws on error response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
    }));

    await expect(reorderFiles("default", ["aaa"])).rejects.toThrow("Failed to reorder files");
  });
});

describe("moveFile", () => {
  it("sends PUT with correct URL and body", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
    }));

    await moveFile("eee55555", "docs");
    expect(fetch).toHaveBeenCalledWith("/_/api/files/eee55555/group", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ group: "docs" }),
    });
  });

  it("throws with server error message", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: false,
      status: 409,
      text: () => Promise.resolve('file "a.md" already exists in group "docs"\n'),
    }));

    await expect(moveFile("aaa11111", "docs")).rejects.toThrow('file "a.md" already exists in group "docs"');
  });

  it("throws default message when response body is empty", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      text: () => Promise.resolve(""),
    }));

    await expect(moveFile("aaa11111", "docs")).rejects.toThrow("Failed to move file");
  });
});

describe("uploadFile", () => {
  it("sends POST with correct body", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true }));

    await uploadFile("test.md", "# Hello", "default");
    expect(fetch).toHaveBeenCalledWith("/_/api/files/upload", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: "test.md", content: "# Hello", group: "default" }),
    });
  });

  it("throws on error response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      text: () => Promise.resolve(""),
    }));

    await expect(uploadFile("test.md", "# Hello", "default")).rejects.toThrow("Failed to upload file");
  });
});
