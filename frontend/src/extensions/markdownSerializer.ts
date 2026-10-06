import type { Editor } from "@tiptap/core";

/** Bump when serialization output format changes. */
export const markdownSerializerVersion = "1" as const;

/** Get the editor content as a markdown string via @tiptap/markdown. */
export function getMarkdown(editor: Editor): string {
  return editor.getMarkdown();
}
