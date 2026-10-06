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
});
