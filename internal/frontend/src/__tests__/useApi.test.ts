import { describe, it, expect, vi, beforeEach } from "vitest";
import { fetchGroups, fetchFileContent, openRelativeFile } from "../hooks/useApi";

beforeEach(() => {
  vi.restoreAllMocks();
});

describe("fetchGroups", () => {
  it("returns groups on success", async () => {
    const data = [{ name: "default", files: [{ id: 1, name: "a.md", path: "/a.md" }] }];
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

    const result = await fetchFileContent(42);
    expect(result).toEqual(data);
    expect(fetch).toHaveBeenCalledWith("/_/api/files/42/content");
  });

  it("throws on error response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
    }));

    await expect(fetchFileContent(99)).rejects.toThrow("Failed to fetch file content");
  });
});

describe("openRelativeFile", () => {
  it("sends POST with correct body", async () => {
    const entry = { id: 5, name: "other.md", path: "/other.md" };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(entry),
    }));

    const result = await openRelativeFile(3, "./other.md");
    expect(result).toEqual(entry);
    expect(fetch).toHaveBeenCalledWith("/_/api/files/open", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ fileId: 3, path: "./other.md" }),
    });
  });

  it("throws on error response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
    }));

    await expect(openRelativeFile(1, "missing.md")).rejects.toThrow("Failed to open file");
  });
});
