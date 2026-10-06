import { describe, expect, it, vi, afterEach } from "vitest";
import { api } from "./api";

afterEach(() => vi.unstubAllGlobals());

describe("revision request", () => {
  it("sends selected model", async () => {
    const body = new TextEncoder().encode(
      'event: done\ndata: {"originalContent":"a","revisedContent":"b","model":"selected","tokensIn":1,"tokensOut":1,"costEstimateUsd":0,"appliedCommentIds":[],"identical":false}\n\n',
    );
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        body: {
          getReader: () => ({
            read: vi.fn()
              .mockResolvedValueOnce({ done: false, value: body })
              .mockResolvedValueOnce({ done: true, value: undefined }),
          }),
        },
      }),
    );

    await api.previewRevisionStream("doc-1", vi.fn(), undefined, ["comment-1"], "selected");

    expect(fetch).toHaveBeenCalledWith(
      "/api/documents/doc-1/revise",
      expect.objectContaining({
        body: JSON.stringify({ commentIds: ["comment-1"], model: "selected" }),
      }),
    );
  });

  it("sends matter revision requests and maps stale conflicts", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ ok: true, status: 200, json: async () => ({ sha: "abc" }) })
      .mockResolvedValueOnce({ ok: true, status: 200, json: async () => ({ diff: "-a\\n+b" }) })
      .mockResolvedValueOnce({ ok: false, status: 409, statusText: "Conflict", json: async () => ({ error: "stale parent" }) });
    vi.stubGlobal("fetch", fetchMock);
    const payload = { matterId: "m1", parentSHA: "old", files: { "a.md": "b" }, meta: { session: "s", operation: "save", revisionId: "r", serializerVersion: "1" } };
    await api.createMatterRevision("doc-1", payload);
    await api.diffMatterRevision("doc-1", "m1", "old", "abc", "a.md");
    await expect(api.revertMatterRevision("doc-1", "abc", { matterId: "m1", parentSHA: "old" })).rejects.toMatchObject({ message: "stale parent" });
    expect(fetchMock.mock.calls[0][0]).toBe("/api/documents/doc-1/matter-revisions");
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual(payload);
    expect(fetchMock.mock.calls[1][0]).toBe("/api/documents/doc-1/matter-revisions/diff?matterId=m1&from=old&to=abc&path=a.md");
    expect(fetchMock.mock.calls[2][0]).toBe("/api/documents/doc-1/matter-revisions/abc/revert");
  });

  it("fetches matter revision history with matterId query", async () => {
    const history = [{ sha: "abc", parentSHA: "", createdAt: "t", actor: "u", operation: "save", message: "m" }];
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => history });
    vi.stubGlobal("fetch", fetchMock);

    await expect(api.getMatterRevisionHistory("doc-1", "m 1")).resolves.toEqual(history);
    expect(fetchMock.mock.calls[0][0]).toBe("/api/documents/doc-1/matter-revisions?matterId=m%201");
  });
});
