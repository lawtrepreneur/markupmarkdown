import { describe, expect, it } from "vitest";
import { markdownSerializerVersion, getMarkdown } from "./markdownSerializer";
import { createEditor } from "./testHelpers";

describe("markdownSerializer", () => {
  it("has pinned serializer version", () => {
    expect(markdownSerializerVersion).toBe("1");
  });

  it("round-trips headings + inline", () => {
    const md = [
      "# Heading one",
      "",
      "A paragraph with **bold** and *italic* and `code`.",
    ].join("\n");
    const editor = createEditor(md);
    const out = getMarkdown(editor);
    expect(out).toContain("# Heading one");
    expect(out).toContain("**bold**");
    expect(out).toContain("*italic*");
    editor.destroy();
  });

  it("round-trips bullet and ordered lists", () => {
    const md = ["- item one", "- item two", "", "1. first", "2. second"].join("\n");
    const editor = createEditor(md);
    const out = getMarkdown(editor);
    expect(out).toContain("- item one");
    expect(out).toContain("1. first");
    editor.destroy();
  });

  it("round-trips blockquote", () => {
    const editor = createEditor("> a wise quote");
    const out = getMarkdown(editor);
    expect(out).toContain("> a wise quote");
    editor.destroy();
  });

  it("round-trips fenced code block", () => {
    const md = ["```", "const x = 1;", "```"].join("\n");
    const editor = createEditor(md);
    const out = getMarkdown(editor);
    expect(out).toContain("const x = 1;");
    editor.destroy();
  });

  it("round-trips links", () => {
    const editor = createEditor("A [link](https://example.com) inline.");
    const out = getMarkdown(editor);
    expect(out).toContain("[link](https://example.com)");
    editor.destroy();
  });

  it("round-trips tables with GFM pipes", () => {
    const md = ["| a | b |", "| --- | --- |", "| 1 | 2 |"].join("\n");
    const editor = createEditor(md);
    const out = getMarkdown(editor);
    expect(out).toMatch(/\| a\s+\| b\s+\|/);
    expect(out).toMatch(/\| 1\s+\| 2\s+\|/);
    editor.destroy();
  });

  it("paragraph ids do not appear in markdown output", () => {
    const editor = createEditor("plain text");
    const json = editor.getJSON();
    // manually stamp paragraphId into first paragraph
    const patched = {
      ...json,
      content: json.content?.map((node, i) =>
        i === 0 ? { ...node, attrs: { ...node.attrs, paragraphId: "abc-123" } } : node,
      ),
    };
    editor.commands.setContent(patched);
    const out = getMarkdown(editor);
    expect(out).not.toContain("abc-123");
    editor.destroy();
  });
});
