// The user manual (ADR-0183): one Markdown file per UI language in
// assets/manual, embedded with the rest of the UI, shown read-only at
// #/manual through the same blocks a body is made of (ADR-0133). The module
// does not touch the DOM, so it is tested in Node.js (internal/web/jstest).
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

// manualBlocks turns the manual's Markdown into its title and blocks of
// the item model. The title (the first heading of level 1) stands above the
// text as an item's title does, so the chapters move up one level, as a
// body's headings render below the title; a table of contents comes before
// the first chapter. Links keep their targets only where isSafe allows
// them, as in a body.
export function manualBlocks(markdown, isSafe = () => false) {
  const { blocks } = markdownToBlocks(markdown, isSafe);
  const title = blocks[0]?.kind === "heading" && blocks[0].level === 1 ? blocks.shift().text : "";
  const body = blocks.map((b) => (b.kind === "heading" ? { ...b, level: Math.max(1, b.level - 1) } : b));
  const first = body.findIndex((b) => b.kind === "heading");
  body.splice(first < 0 ? body.length : first, 0, { kind: "toc" });
  return { title, blocks: body };
}
