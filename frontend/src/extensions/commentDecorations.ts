import { Extension } from "@tiptap/core";
import { Plugin, PluginKey } from "@tiptap/pm/state";
import { Decoration, DecorationSet } from "@tiptap/pm/view";
import type { Node as PMNode } from "@tiptap/pm/model";
import type { Comment } from "../types";

export const commentDecorationsKey = new PluginKey("commentDecorations");

// First in-block occurrence of anchor.exact; skips orphans and empty anchors.
export function buildCommentDecorations(doc: PMNode, comments: Comment[], activeId?: string) {
  const decos: Decoration[] = [];
  const live = comments.filter((c) => !c.orphan && c.anchor?.exact);
  const done = new Set<string>();
  doc.descendants((block, blockPos) => {
    if (!block.isTextblock) return true;
    const segs: { pos: number; len: number }[] = [];
    let text = "";
    block.forEach((child, off) => {
      if (!child.isText) return;
      segs.push({ pos: blockPos + 1 + off, len: text.length });
      text += child.text;
    });
    const toPos = (i: number) => {
      let s = segs[0];
      for (const x of segs) if (x.len <= i) s = x;
      return s.pos + (i - s.len);
    };
    for (const c of live) {
      if (done.has(c.id)) continue;
      const i = text.indexOf(c.anchor.exact);
      if (i < 0) continue;
      done.add(c.id);
      const cls = ["comment-anchor"];
      if (c.resolved) cls.push("comment-anchor-resolved");
      if (c.id === activeId) cls.push("comment-anchor-active");
      decos.push(
        Decoration.inline(toPos(i), toPos(i + c.anchor.exact.length - 1) + 1, {
          class: cls.join(" "),
          "data-comment-id": c.id,
        }),
      );
    }
    return false;
  });
  return DecorationSet.create(doc, decos);
}

export const commentDecorations = Extension.create<{
  getComments: () => Comment[];
  getActiveId: () => string | undefined;
}>({
  name: "commentDecorations",
  addOptions() {
    return { getComments: () => [], getActiveId: () => undefined };
  },
  addProseMirrorPlugins() {
    const { getComments, getActiveId } = this.options;
    return [
      new Plugin({
        key: commentDecorationsKey,
        props: {
          decorations: (state) => buildCommentDecorations(state.doc, getComments(), getActiveId()),
        },
      }),
    ];
  },
});
