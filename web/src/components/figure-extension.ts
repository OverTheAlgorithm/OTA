import { Node, mergeAttributes } from "@tiptap/core";

export interface FigureAttrs {
  src: string;
  alt?: string | null;
}

declare module "@tiptap/core" {
  interface Commands<ReturnType> {
    figure: {
      setFigure: (attrs: FigureAttrs) => ReturnType;
    };
  }
}

// Figure renders an image with an editable caption underneath. The img is held
// as a node attribute (not editable content) so the user cannot accidentally
// destroy it while writing the caption. The figcaption text lives in the
// node's inline content slot, making it freely editable like any paragraph.
export const Figure = Node.create({
  name: "figure",
  group: "block",
  content: "inline*",
  draggable: true,
  isolating: true,
  selectable: true,

  addAttributes() {
    return {
      src: { default: null },
      alt: { default: null },
    };
  },

  parseHTML() {
    return [
      {
        tag: "figure",
        // Tiptap pulls the editable text from the figcaption child; the img
        // is captured into the node's attrs via getAttrs below.
        contentElement: "figcaption",
        getAttrs: (dom) => {
          if (!(dom instanceof HTMLElement)) return false;
          const img = dom.querySelector("img");
          if (!img) return false;
          return {
            src: img.getAttribute("src"),
            alt: img.getAttribute("alt"),
          };
        },
      },
    ];
  },

  renderHTML({ HTMLAttributes }) {
    const { src, alt, ...rest } = HTMLAttributes;
    return [
      "figure",
      mergeAttributes(rest),
      ["img", { src, alt: alt ?? "" }],
      ["figcaption", 0],
    ];
  },

  addCommands() {
    return {
      setFigure:
        (attrs) =>
        ({ chain }) =>
          chain()
            .insertContent({
              type: this.name,
              attrs,
              content: [],
            })
            .run(),
    };
  },
});
