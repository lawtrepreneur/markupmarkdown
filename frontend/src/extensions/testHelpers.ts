import { Editor } from "@tiptap/core";
import StarterKit from "@tiptap/starter-kit";
import Link from "@tiptap/extension-link";
import { Table, TableRow, TableCell, TableHeader } from "@tiptap/extension-table";
import { Markdown } from "@tiptap/markdown";
import { paragraphId } from "./paragraphId";

export function createEditor(content: string) {
  return new Editor({
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
  });
}
