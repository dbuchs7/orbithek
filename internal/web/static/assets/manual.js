// The user manual (ADR-0183): one Markdown file per UI language in
// assets/manual, embedded with the rest of the UI, shown read-only at
// #/manual through the same blocks a body is made of (ADR-0133). The module
// does not touch the DOM, so it is tested in Node.js (internal/web/jstest).
//
// Two things the manual has that a body does not
// (ADR-draft-pictures-and-help-links-in-the-manual): pictures served with
// the UI, written as a line of their own,
//
//   ![The start page](img/en/home.png "60")
//
// whose title gives the width in percent of the column and optionally left
// or right, and an anchor at the end of a heading, {#shop}, which a help
// link in the UI jumps to (#/manual?section=shop). Both read on GitHub as
// they do here, the picture beside the Markdown file.
import { markdownToBlocks } from "./markdown.js";

// manualLanguages are the languages the manual is written in; the first is
// the fallback. Every UI language has its manual (TestManualPerLanguage).
export const manualLanguages = ["en", "de"];

// manualLanguage picks the manual's language: the one asked for in the
// address, else the UI's, else the fallback.
export function manualLanguage(asked, ui) {
  if (manualLanguages.includes(asked)) return asked;
  if (manualLanguages.includes(ui)) return ui;
  return manualLanguages[0];
}

// manualPath is the address of the manual's source in a language.
export function manualPath(lang) {
  return `/assets/manual/${manualLanguage(lang, "")}.md`;
}

// manualPictureURL is the address of a picture the manual names, relative
// to the manual's own folder.
export const manualPictureURL = (ref) => "/assets/manual/" + ref;

// A picture: a line of its own, a picture of the manual's img folder.
const pictureLine = /^!\[([^\]\n]*)\]\((img\/[a-z0-9][a-z0-9/_-]*\.(?:png|jpg|webp))(?:\s+"([^"]*)")?\)\s*$/;
// An anchor at the end of a heading.
const headingAnchor = /^(#{1,6}\s.*?)\s*\{#([a-z0-9][a-z0-9-]*)\}\s*$/;

// manualParts splits the manual's own syntax off its Markdown: the
// pictures, each replaced by a paragraph that marks its place, and the
// anchors, by the number of the heading they stand on. What is left is
// Markdown as bodies know it.
export function manualParts(markdown) {
  const pictures = [];
  const anchors = [];
  let heading = 0;
  let fence = false;
  const lines = String(markdown).split("\n").map((line) => {
    if (/^(```|~~~)/.test(line)) fence = !fence;
    if (fence) return line;
    const p = line.match(pictureLine);
    if (p) {
      const [, caption, ref, how = ""] = p;
      const width = Math.min(100, Math.max(0, parseInt(how, 10) || 0));
      const align = /\bleft\b/.test(how) ? "left" : /\bright\b/.test(how) ? "right" : "";
      pictures.push({ ref, caption, width, align });
      return `@@picture-${pictures.length - 1}@@`;
    }
    if (/^#{1,6}\s/.test(line)) {
      const a = line.match(headingAnchor);
      anchors[heading++] = a ? a[2] : null;
      return a ? a[1] : line;
    }
    return line;
  });
  return { markdown: lines.join("\n"), pictures, anchors };
}

// manualBlocks turns the manual's Markdown into its title and blocks of
// the item model. The title (the first heading of level 1) stands above the
// text as an item's title does, so the chapters move up one level, as a
// body's headings render below the title; a table of contents comes before
// the first chapter. Links keep their targets only where isSafe allows
// them, as in a body. Pictures become picture blocks whose ref is their
// path in the manual's folder; sections maps an anchor to the number of
// its heading among the headings shown.
export function manualBlocks(markdown, isSafe = () => false) {
  const parts = manualParts(markdown);
  const { blocks } = markdownToBlocks(parts.markdown, isSafe);
  const anchors = [...parts.anchors];
  let title = "";
  if (blocks[0]?.kind === "heading" && blocks[0].level === 1) {
    title = blocks.shift().text;
    anchors.shift();
  }
  const sections = {};
  let n = 0;
  const body = blocks.map((b) => {
    if (b.kind === "heading") {
      if (anchors[n]) sections[anchors[n]] = n;
      n++;
      return { ...b, level: Math.max(1, b.level - 1) };
    }
    const marker = b.kind === "paragraph" && /^@@picture-(\d+)@@$/.exec(b.text || "");
    if (marker) {
      const p = parts.pictures[Number(marker[1])];
      const pic = { kind: "image", ref: p.ref, text: p.caption };
      if (p.width) pic.width = p.width;
      if (p.align) pic.align = p.align;
      return pic;
    }
    return b;
  });
  const first = body.findIndex((b) => b.kind === "heading");
  body.splice(first < 0 ? body.length : first, 0, { kind: "toc" });
  return { title, blocks: body, sections };
}
