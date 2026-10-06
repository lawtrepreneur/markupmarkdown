import { useEffect, useRef } from "react";
import { Editor, EditorContent, useEditor } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import Link from "@tiptap/extension-link";
import { Table, TableRow, TableCell, TableHeader } from "@tiptap/extension-table";
import { Markdown } from "@tiptap/markdown";
import { paragraphId } from "../extensions/paragraphId";

export const serializerVersion = "1" as const;

type Props = {
  content?: string;
  onChange?: (markdown: string, editor: Editor) => void;
  onReady?: (editor: Editor) => void;
};

export function TiptapEditor({ content = "", onChange, onReady }: Props) {
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

  useEffect(() => () => editor?.destroy(), [editor]);

  return <EditorContent editor={editor} />;
}
