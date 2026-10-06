import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from "react";
import type { Editor } from "@tiptap/react";
import type { Comment } from "../types";
import { TiptapEditor } from "./TiptapEditor";
import MarkdownRender from "./MarkdownRender";
import { baseURLForDoc } from "../utils/baseUrl";

export interface EditorPaneHandle {
  coordsForAnchor(exact: string): { top: number; bottom: number } | null;
  scrollAnchorIntoView(exact: string): void;
}

interface Props {
  initialContent: string;
  sourceUrl?: string;
  saving: boolean;
  onSave: (content: string) => Promise<void> | void;
  onCancel: () => void;
  activeAnchorExact?: string;
  comments?: Comment[];
  activeCommentId?: string;
  onLayoutTick?: () => void;
}

const EditorPane = forwardRef<EditorPaneHandle, Props>(function EditorPane({
  initialContent, sourceUrl, saving, onSave, onCancel, activeAnchorExact, comments, activeCommentId, onLayoutTick,
}, ref) {
  const [content, setContent] = useState(initialContent);
  const [showPreview, setShowPreview] = useState(false);
  const [editor, setEditor] = useState<Editor | null>(null);
  const contentRef = useRef(content);
  contentRef.current = content;
  const dirty = content !== initialContent;

  useEffect(() => {
    if (!editor || !activeAnchorExact) return;
    const root = editor.view.dom;
    const text = root.textContent ?? "";
    const index = text.indexOf(activeAnchorExact);
    if (index < 0) return;
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    let offset = 0;
    let node: Node | null;
    while ((node = walker.nextNode())) {
      const length = node.textContent?.length ?? 0;
      if (index < offset + length && index + activeAnchorExact.length <= offset + length) {
        node.parentElement?.scrollIntoView({ behavior: "smooth", block: "center" });
        return;
      }
      offset += length;
    }
  }, [editor, activeAnchorExact]);

  useImperativeHandle(ref, () => ({
    coordsForAnchor(exact) {
      if (!editor || !exact) return null;
      const root = editor.view.dom;
      const text = root.textContent ?? "";
      const index = text.indexOf(exact);
      if (index < 0) return null;
      const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
      let offset = 0;
      let node: Node | null;
      while ((node = walker.nextNode())) {
        const length = node.textContent?.length ?? 0;
        if (index < offset + length && index + exact.length <= offset + length) {
          const range = document.createRange();
          range.setStart(node, index - offset);
          range.setEnd(node, index - offset + exact.length);
          const rect = range.getBoundingClientRect();
          return { top: rect.top, bottom: rect.bottom };
        }
        offset += length;
      }
      return null;
    },
    scrollAnchorIntoView(exact) {
      if (!editor || !exact) return;
      const root = editor.view.dom;
      const text = root.textContent ?? "";
      const index = text.indexOf(exact);
      if (index < 0) return;
      const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
      let offset = 0;
      let node: Node | null;
      while ((node = walker.nextNode())) {
        const length = node.textContent?.length ?? 0;
        if (index < offset + length && index + exact.length <= offset + length) {
          const range = document.createRange();
          range.setStart(node, index - offset);
          range.setEnd(node, index - offset + exact.length);
          range.getBoundingClientRect();
          node.parentElement?.scrollIntoView({ behavior: "smooth", block: "center" });
          return;
        }
        offset += length;
      }
    },
  }), [editor]);

  return (
    <div className="space-y-3">
      <div className="sticky top-0 z-10 bg-card border border-rule rounded-md shadow-sm">
        <div className="flex items-center gap-2 px-3 py-2">
          <div className="text-sm font-medium">Editing</div>
          <div className="text-xs text-muted">{dirty ? "Unsaved changes" : "No changes yet"}</div>
          <button className="ml-auto text-xs px-2 py-1 rounded text-muted hover:text-ink hover:bg-soft" onClick={() => setShowPreview((v) => !v)}>{showPreview ? "Hide preview" : "Preview"}</button>
          <button className="text-xs px-2 py-1 rounded text-muted hover:text-ink hover:bg-soft" disabled={!dirty || saving} onClick={() => onSave(contentRef.current)}>Save</button>
          <button className="text-xs px-2 py-1 rounded text-muted hover:text-ink hover:bg-soft" disabled={saving} onClick={onCancel}>Cancel</button>
        </div>
      </div>
      <div className={`grid gap-3 ${showPreview ? "md:grid-cols-2" : "grid-cols-1"}`}>
        <div className="border border-rule rounded-md bg-card p-3">
          <TiptapEditor content={initialContent} comments={comments} activeCommentId={activeCommentId} onReady={setEditor} onChange={(markdown, current) => { setEditor(current); setContent(markdown); onLayoutTick?.(); }} />
        </div>
        {showPreview && <div className="border border-rule rounded-md p-3 bg-card"><MarkdownRender content={content} baseUrl={baseURLForDoc(sourceUrl)} sourceUrl={sourceUrl} /></div>}
      </div>
    </div>
  );
});

export default EditorPane;
