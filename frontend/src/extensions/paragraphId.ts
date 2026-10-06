import { Extension } from "@tiptap/core";

export const paragraphId = Extension.create({
  name: "paragraphId",

  addGlobalAttributes() {
    return [
      {
        types: ["paragraph"],
        attributes: {
          paragraphId: {
            default: null,
            parseHTML: (element: HTMLElement) => element.getAttribute("data-paragraph-id"),
            renderHTML: (attributes: Record<string, any>) =>
              attributes.paragraphId ? { "data-paragraph-id": attributes.paragraphId } : {},
          },
        },
      },
    ];
  },
});
