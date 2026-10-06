import { describe, expect, it } from "vitest";
import { buildCommentDecorations } from "./commentDecorations";
import { createEditor } from "./testHelpers";
import type { Comment } from "../types";

const c = (id: string, exact: string, extra: Partial<Comment> = {}) =>
  ({ id, anchor: { start: 0, end: 0, exact }, resolved: false, replies: [], ...extra }) as unknown as Comment;

describe("commentDecorations", () => {
  it("decorates exact text across marks, skips orphans, no DOM mutation of doc", () => {
    const ed = createEditor("Hello **bold world** end\n\nsecond para");
    const before = ed.getJSON();
    const set = buildCommentDecorations(
      ed.state.doc,
      [c("a", "bold world"), c("b", "second", { resolved: true }), c("o", "Hello", { orphan: true }), c("m", "nope")],
      "a",
    );
    const found = set.find();
    expect(found).toHaveLength(2);
    const a = found.find((d) => (d as any).type.attrs["data-comment-id"] === "a")!;
    expect(ed.state.doc.textBetween(a.from, a.to)).toBe("bold world");
    expect((a as any).type.attrs.class).toContain("comment-anchor-active");
    const b = found.find((d) => (d as any).type.attrs["data-comment-id"] === "b")!;
    expect((b as any).type.attrs.class).toContain("comment-anchor-resolved");
    expect(ed.getJSON()).toEqual(before);
  });
});
