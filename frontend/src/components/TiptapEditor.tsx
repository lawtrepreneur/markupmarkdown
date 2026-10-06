import { useEffect, useRef } from "react";
import { Editor, EditorContent, useEditor } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import Link from "@tiptap/extension-link";
import { Table, TableRow, TableCell, TableHeader } from "@tiptap/extension-table";
import { Markdown } from "@tiptap/markdown";
import { paragraphId } from "../extensions/paragraphId";
import { commentDecorations, commentDecorationsKey } from "../extensions/commentDecorations";
import type { Comment } from "../types";

export const serializerVersion = "1" as const;

type Props = {
  content?: string;
  comments?: Comment[];
  activeCommentId?: string;
  onChange?: (markdown: string, editor: Editor) => void;
  onReady?: (editor: Editor) => void;
};

export function TiptapEditor({ content = "", comments = [], activeCommentId, onChange, onReady }: Props) {
  const commentsRef = useRef(comments);
  commentsRef.current = comments;
  const activeRef = useRef(activeCommentId);
  activeRef.current = activeCommentId;
  const onReadyRef = useRef(onReady);
  onReadyRef.current = onReady;
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;
  const editor = useEditor({
    extensions: [
      StarterKit,
      Link,
      Table.configure({ resizable: false }),
      TableRow,
      TableCell,
      TableHeader,
      paragraphId,
      commentDecorations.configure({
        getComments: () => commentsRef.current,
        getActiveId: () => activeRef.current,
      }),
      Markdown.configure({ markedOptions: { gfm: true } }),
    ],
    content,
    contentType: "markdown",
    onUpdate: ({ editor: current }) => onChangeRef.current?.(current.getMarkdown(), current),
  });

  useEffect(() => {
    if (editor) onReadyRef.current?.(editor);
  }, [editor]);

  const lastContent = useRef(content);
  useEffect(() => {
    if (!editor || lastContent.current === content) return;
    lastContent.current = content;
    editor.commands.setContent(content, { contentType: "markdown" });
  }, [editor, content]);

  useEffect(() => {
    if (editor) editor.view.dispatch(editor.state.tr.setMeta(commentDecorationsKey, true));
  }, [editor, comments, activeCommentId]);

  useEffect(() => () => editor?.destroy(), [editor]);

  return <EditorContent editor={editor} />;
}
