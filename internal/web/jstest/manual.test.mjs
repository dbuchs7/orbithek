// Unit tests of the user manual (manual.js, assets/manual/*.md; ADR-0183).
// They run in Node.js without a browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// The manual's way into the UI (the account menu, #/manual) is in
// e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync, existsSync } from "node:fs";
import { manualBlocks, manualLanguage, manualLanguages, manualPath, manualParts, manualPictureURL } from "../static/assets/manual.js";
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
  assert.deepEqual(manualBlocks("# T\n\n## Part\n").sections, {});
  const untitled = manualBlocks("Text only");
  assert.equal(untitled.title, "");
  assert.deepEqual(untitled.blocks, [{ kind: "paragraph", text: "Text only" }, { kind: "toc" }]);
});

for (const lang of manualLanguages) {
  test(`the ${lang} manual converts whole into blocks the editor shows`, () => {
    const { blocks, pictures, dropped } = markdownToBlocks(manualParts(source(lang)).markdown, isSafeLink);
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
const outline = (lang) => markdownToBlocks(manualParts(source(lang)).markdown).blocks.filter((b) => b.kind === "heading").map((b) => b.level);

test("every manual has the outline of the English one, so they stay translations of each other", () => {
  const en = outline("en");
  assert.ok(en.length > 20, "the manual has its chapters");
  for (const lang of manualLanguages) assert.deepEqual(outline(lang), en, `${lang}.md has another outline than en.md`);
});

test("the German manual is in Swiss spelling", () => {
  assert.ok(!source("de").includes("ß"), "de.md writes ss, never ß");
});

test("a picture of the manual is a line of its own and becomes a picture block", () => {
  const md = '# T\n\nBefore\n\n![The start page](img/en/home.png "60 left")\n\n![Bare](img/icons/shop.png)\n\nText ![inline](img/en/x.png) stays text\n\n![Elsewhere](https://example.org/x.png)\n';
  const { blocks } = manualBlocks(md, isSafeLink);
  assert.deepEqual(blocks.filter((b) => b.kind === "image"), [
    { kind: "image", ref: "img/en/home.png", text: "The start page", width: 60, align: "left" },
    { kind: "image", ref: "img/icons/shop.png", text: "Bare" },
  ]);
  assert.ok(blocks.some((b) => b.kind === "paragraph" && b.text.includes("inline")), "a picture inside a sentence is no picture block");
  assert.equal(manualPictureURL("img/en/home.png"), "/assets/manual/img/en/home.png");
  assert.doesNotThrow(() => docFromBlocks(blocks).check());
  // Not inside a code block.
  assert.equal(manualParts("```\n![x](img/en/a.png)\n```\n").pictures.length, 0);
});

test("an anchor at the end of a heading names it for a help link and is not shown", () => {
  const { blocks, sections } = manualBlocks("# Title {#title}\n\n## Shop {#shop}\n\n### Plain\n\n### Catalogue {#shop-catalogue}\n");
  assert.deepEqual(sections, { shop: 0, "shop-catalogue": 2 });
  assert.deepEqual(blocks.filter((b) => b.kind === "heading").map((b) => b.text), ["Shop", "Plain", "Catalogue"]);
});

// anchorsOf are a manual's anchors, in the order of its headings.
const anchorsOf = (lang) => manualParts(source(lang)).anchors.filter(Boolean);
// picturesOf are the pictures a manual shows, with its language in their
// path written as LANG.
const picturesOf = (lang) => manualParts(source(lang)).pictures.map((p) => p.ref.replace(`img/${lang}/`, "img/LANG/"));

test("every manual has the English one's anchors and pictures, and every picture is there", () => {
  const anchors = anchorsOf("en");
  assert.ok(anchors.includes("shop") && anchors.includes("process-engines"), "the anchors the UI's help links name");
  assert.equal(new Set(anchors).size, anchors.length, "each anchor once");
  for (const lang of manualLanguages) {
    assert.deepEqual(anchorsOf(lang), anchors, `${lang}.md has other anchors than en.md`);
    assert.deepEqual(picturesOf(lang), picturesOf("en"), `${lang}.md shows other pictures than en.md`);
    for (const p of manualParts(source(lang)).pictures) {
      assert.ok(existsSync(new URL(`../static/assets/manual/${p.ref}`, import.meta.url)), `${lang}.md shows ${p.ref}, which is missing`);
      assert.ok(p.ref.startsWith(`img/${lang}/`) || p.ref.startsWith("img/icons/"), `${lang}.md shows ${p.ref} from another language`);
    }
  }
});
