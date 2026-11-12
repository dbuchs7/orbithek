// Unit tests of capturing from anywhere (capture.js, ADR-0207) and of the
// parts of a web link's card (weblink.js, ADR-0206): what a share passes
// on, the blocks a capture adds, the targets it may go to, the
// bookmarklet, and what a card shows. They run in Node.js without a
// browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// The views themselves are in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import { LIMITS, TARGETS, bookmarklet, captureAddress, captureBlocks, isWebAddress, paragraphs, readCapture, targetsFor } from "../static/assets/capture.js";
import { isWebLink, pictureOf, webLinkParts } from "../static/assets/weblink.js";

test("a share is read from the address, trimmed and bounded", () => {
  const c = readCapture(new URLSearchParams({ title: "  A page ", text: " Some words. ", url: "https://example.org/a?b=1" }));
  assert.deepEqual(c, { title: "A page", text: "Some words.", url: "https://example.org/a?b=1" });
  assert.deepEqual(readCapture({}), { title: "", text: "", url: "" });
  assert.equal([...readCapture({ title: "x".repeat(LIMITS.title + 10) }).title].length, LIMITS.title);
  assert.equal(readCapture({ url: "javascript:alert(1)" }).url, "", "only web addresses");
});

test("an address at the end of the text is the url when none came", () => {
  assert.deepEqual(readCapture({ title: "Story", text: "Worth reading\nhttps://example.org/story" }),
    { title: "Story", text: "Worth reading", url: "https://example.org/story" });
  assert.deepEqual(readCapture({ text: "https://example.org/only" }), { title: "", text: "", url: "https://example.org/only" });
  // An address within the text stays text; a given url wins.
  assert.equal(readCapture({ text: "see https://a.example/x for more" }).url, "");
  assert.equal(readCapture({ text: "x https://a.example/", url: "https://b.example/" }).url, "https://b.example/");
  // A text that only repeats the title says nothing more.
  assert.equal(readCapture({ title: "Same", text: "Same" }).text, "");
});

test("web addresses are http and https only", () => {
  for (const ok of ["https://example.org", "http://example.org/a b".split(" ")[0]]) assert.ok(isWebAddress(ok), ok);
  for (const bad of ["", "example.org", "ftp://example.org", "javascript:alert(1)", "https://", "https://exa mple.org"]) assert.ok(!isWebAddress(bad), bad);
});

test("a capture adds its text as quotes and names its source", () => {
  assert.deepEqual(paragraphs("one\n\n  two\nlines \n\n\n"), ["one", "two lines"]);
  assert.deepEqual(captureBlocks({ title: "Über uns", text: "First.\n\nSecond.", url: "https://example.org/" }, "Source:"), [
    { kind: "quote", text: "First." },
    { kind: "quote", text: "Second." },
    { kind: "paragraph", text: "Source: Über uns", marks: [{ type: "link", start: 8, end: 16, href: "https://example.org/" }] },
  ]);
  assert.deepEqual(captureBlocks({ title: "", text: "", url: "https://example.org/x" }, "Source:"),
    [{ kind: "paragraph", text: "Source: https://example.org/x", marks: [{ type: "link", start: 8, end: 29, href: "https://example.org/x" }] }]);
  assert.deepEqual(captureBlocks({ title: "A thought", text: "Why", url: "" }, "Source:"), [{ kind: "paragraph", text: "A thought" }, { kind: "quote", text: "Why" }]);
  assert.deepEqual(captureBlocks({ title: "", text: "Only text", url: "" }, "Source:"), [{ kind: "quote", text: "Only text" }]);
});

test("the targets are data, and a web link needs an address", () => {
  assert.deepEqual(TARGETS.map((tg) => tg.id), ["weblink", "item", "append"]);
  assert.deepEqual(targetsFor({ title: "t", text: "", url: "https://example.org" }).map((tg) => tg.id), ["weblink", "item", "append"]);
  assert.deepEqual(targetsFor({ title: "t", text: "", url: "" }).map((tg) => tg.id), ["item", "append"]);
  assert.deepEqual(targetsFor({ title: "", text: "", url: "" }), []);
  // One more target is one more entry.
  const daily = { id: "daily", takes: () => true };
  assert.deepEqual(targetsFor({ title: "", text: "", url: "" }, [...TARGETS, daily]).map((tg) => tg.id), ["daily"]);
});

test("the capture view's address carries what it shows", () => {
  assert.equal(captureAddress({ title: "A & B", text: "", url: "https://example.org/?x=1" }), "#/capture?title=A+%26+B&url=https%3A%2F%2Fexample.org%2F%3Fx%3D1");
  assert.equal(captureAddress({ title: "", text: "", url: "" }), "#/capture");
});

test("the bookmarklet opens this site's capture view with the page", () => {
  const code = bookmarklet("https://wiki.example.org/");
  assert.ok(code.startsWith("javascript:(()=>{"), code);
  assert.ok(code.includes(`window.open("https://wiki.example.org/#/capture?"+q`), code);
  assert.ok(code.includes("getSelection()") && code.includes("location.href") && code.includes("document.title"), code);
  // The origin is quoted as a string; nothing of it runs.
  assert.ok(bookmarklet(`https://x.example/"+alert(1)+"`).includes(`"https://x.example/\\"+alert(1)+\\"/#/capture?"`));
  // Run it in a stand-in browser: it opens the capture view with the page.
  let opened = null;
  const fn = new Function("document", "location", "getSelection", "window", code.slice("javascript:".length));
  fn({ title: "Page title" }, { href: "https://example.org/read" }, () => "Selected words", { open: (u, name) => { opened = { u, name }; } });
  assert.equal(opened.name, "orbithek-capture");
  const q = new URLSearchParams(opened.u.split("#/capture?")[1]);
  assert.deepEqual(readCapture(q), { title: "Page title", text: "Selected words", url: "https://example.org/read" });
});

test("a web link's card shows its page's parts and picture", () => {
  const item = { id: "i1", title: "The story", properties: { url: "https://www.example.org/story", site: "Example News", author: " Ada ", published: "2026-10-30T00:00:00Z", description: "About it" } };
  const files = [
    { id: "a", name: "preview.png", type: "image/png" },
    { id: "b", name: "other.png", type: "image/png" },
    { id: "c", name: "preview.jpg", type: "image/jpeg" },
    { id: "d", name: "preview.svg", type: "image/svg+xml" },
  ];
  assert.equal(pictureOf(files).id, "c", "the newest picture named as a page's");
  assert.equal(pictureOf([{ id: "b", name: "other.png", type: "image/png" }]), null);
  assert.deepEqual(webLinkParts(item, files), { url: "https://www.example.org/story", title: "The story", site: "Example News", author: "Ada",
    published: "2026-10-30", description: "About it", picture: files[2] });
  const bare = webLinkParts({ id: "i2", title: "", properties: { url: "https://www.example.org/x" } }, []);
  assert.equal(bare.site, "example.org");
  assert.equal(bare.title, "example.org");
  assert.ok(isWebLink(item));
  assert.ok(!isWebLink({ properties: { url: "javascript:alert(1)" } }));
  assert.ok(!isWebLink({ properties: {} }));
});
