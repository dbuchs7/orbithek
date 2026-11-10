// Unit tests of the user manual (manual.js, assets/manual/*.md; ADR-0182).
// They run in Node.js without a browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// The manual's way into the UI (the account menu, #/manual) is in
// e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { manualBlocks, manualLanguage, manualLanguages, manualPath } from "../static/assets/manual.js";
import { markdownToBlocks } from "../static/assets/markdown.js";
import { isSafeLink, docFromBlocks } from "../static/assets/editor.js";

const source = (lang) => readFileSync(new URL(`../static/assets/manual/${lang}.md`, import.meta.url), "utf8");

test("the manual's language is the one asked for, else the UI's, else English", () => {
  assert.equal(manualLanguage("de", "en"), "de");
  assert.equal(manualLanguage("", "de"), "de");
  assert.equal(manualLanguage("fr", "de"), "de");
  assert.equal(manualLanguage("", "fr"), "en");
  assert.equal(manualLanguage("../x", ""), "en");
});

test("the manual's address names only a language it is written in", () => {
  assert.equal(manualPath("de"), "/assets/manual/de.md");
  assert.equal(manualPath("../../etc/passwd"), "/assets/manual/en.md");
});

test("the title stands apart, the chapters move up a level, the contents come before the first chapter", () => {
  const { title, blocks } = manualBlocks("# Title\n\nIntro\n\n## Part\n\n### Section\n");
  assert.equal(title, "Title");
  assert.deepEqual(blocks, [
    { kind: "paragraph", text: "Intro" },
    { kind: "toc" },
    { kind: "heading", level: 1, text: "Part" },
    { kind: "heading", level: 2, text: "Section" },
  ]);
  const untitled = manualBlocks("Text only");
  assert.equal(untitled.title, "");
  assert.deepEqual(untitled.blocks, [{ kind: "paragraph", text: "Text only" }, { kind: "toc" }]);
});

for (const lang of manualLanguages) {
  test(`the ${lang} manual converts whole into blocks the editor shows`, () => {
    const { blocks, pictures, dropped } = markdownToBlocks(source(lang), isSafeLink);
    assert.equal(pictures, 0, "no picture became a link");
    assert.equal(dropped, 0, "no link lost its target");
    assert.equal(blocks[0].kind, "heading");
    assert.equal(blocks[0].level, 1);
    assert.equal(blocks.filter((b) => b.kind === "heading" && b.level === 1).length, 1, "one title");
    assert.ok(blocks.every((b) => b.kind !== "heading" || b.text.trim()), "no empty heading");
    // The schema accepts every block, as the reading view needs.
    assert.doesNotThrow(() => docFromBlocks(manualBlocks(source(lang), isSafeLink).blocks).check());
  });
}

// headings is the outline of a manual: the level of each heading in order.
const outline = (lang) => markdownToBlocks(source(lang)).blocks.filter((b) => b.kind === "heading").map((b) => b.level);

test("every manual has the outline of the English one, so they stay translations of each other", () => {
  const en = outline("en");
  assert.ok(en.length > 20, "the manual has its chapters");
  for (const lang of manualLanguages) assert.deepEqual(outline(lang), en, `${lang}.md has another outline than en.md`);
});

test("the German manual is in Swiss spelling", () => {
  assert.ok(!source("de").includes("ß"), "de.md writes ss, never ß");
});
