// Tests of which block a click in the editor selects (editor.js,
// ADR-0042): a click inside a block's own content — an embedded item's
// text, a view, a search field, a starmap, a slide deck — selects the
// block, as a click on its frame does, so a person sees which block the
// next line, cut or insertion goes after. A click on a control or field
// of the block, on a table's cells or on the text around leaves the
// selection to them. They run in Node.js without a browser.
import { test } from "node:test";
import assert from "node:assert/strict";
import { docFromBlocks, clickedBlock } from "../static/assets/editor.js";

// el makes an element of a fake DOM: tag names the element, attrs are
// matched by closest's simple selectors (tag, [attr], [attr=value]).
function el(tag, attrs = {}, ...children) {
  const node = { tag, attrs, parent: null, children };
  children.forEach((c) => { c.parent = node; });
  const matches = (sel) => sel.split(",").map((s) => s.trim()).some((s) => {
    const m = /^([a-z]*)(?:\[([a-z-]+)(?:=([a-z]+))?\])?$/.exec(s);
    if (!m) return false;
    const [, t, a, v] = m;
    if (t && t !== tag) return false;
    if (a && !(a in attrs)) return false;
    return !v || attrs[a] === v;
  });
  node.closest = (sel) => {
    for (let n = node; n; n = n.parent) if (n.matches(sel)) return n;
    return null;
  };
  node.matches = matches;
  node.contains = (other) => {
    for (let n = other; n; n = n.parent) if (n === node) return true;
    return false;
  };
  return node;
}

// page lays out a body and its DOM: each top-level block gets the
// element given for it, inside the editor's editable root.
function page(blocks, doms) {
  const doc = docFromBlocks(blocks);
  const root = el("div", { contenteditable: "true" }, ...doms);
  const offsets = [];
  doc.forEach((_, offset) => offsets.push(offset));
  return { root, offsets, view: { state: { doc }, dom: root, nodeDOM: (offset) => doms[offsets.indexOf(offset)] || null } };
}

test("a click inside a view, a search field or an embedded text selects the block", () => {
  const cell = el("td");
  const viewDOM = el("section", { contenteditable: "false" }, el("table", {}, el("tr", {}, cell)));
  const hit = el("span");
  const searchDOM = el("section", { contenteditable: "false" }, el("ul", {}, el("li", {}, hit)));
  const embedText = el("p");
  const embedDOM = el("div", { contenteditable: "false" }, el("div", {}, embedText));
  const { view, offsets } = page(
    [{ kind: "paragraph", text: "First." }, { kind: "view", layout: "table" }, { kind: "search" }, { kind: "embed", ref: "a" }],
    [el("p"), viewDOM, searchDOM, embedDOM]);
  assert.equal(clickedBlock(view, cell), offsets[1], "a cell of a view's table");
  assert.equal(clickedBlock(view, viewDOM), offsets[1], "the view's frame");
  assert.equal(clickedBlock(view, hit), offsets[2], "a hit of a search field");
  assert.equal(clickedBlock(view, embedText), offsets[3], "the text of an embedded item");
});

test("a click on a block's controls and fields is left to them", () => {
  const button = el("button");
  const inButton = el("span");
  const field = el("input");
  const link = el("a", { href: "#/item/A-1" });
  const choice = el("select");
  const area = el("textarea");
  const caption = el("figcaption", { contenteditable: "true" });
  const searchDOM = el("section", { contenteditable: "false" }, el("p", {}, button, el("button", {}, inButton)), field, link, choice, area, caption);
  const { view } = page([{ kind: "search" }], [searchDOM]);
  for (const target of [button, inButton, field, link, choice, area, caption]) assert.equal(clickedBlock(view, target), null, target.tag);
});

test("text, a table's cells and the space between blocks are left to the editor", () => {
  const word = el("span");
  const cell = el("td");
  const tableDOM = el("div", { contenteditable: "false" }, el("table", {}, el("tr", {}, cell)));
  const { view, root } = page([{ kind: "paragraph", text: "Words." }, { kind: "table", text: "a\tb" }], [el("p", {}, word), tableDOM]);
  assert.equal(clickedBlock(view, word), null, "text is the editor's");
  assert.equal(clickedBlock(view, cell), null, "a table's cells are edited and chosen in the table (ADR-0214)");
  assert.equal(clickedBlock(view, root), null, "the editor itself");
  assert.equal(clickedBlock(view, null), null, "no target");
});
