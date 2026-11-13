// A guard over the UI modules: Element.replaceChildren is the browser's own
// method, and it shows every argument that is not a node as text, so a
// child left out with `cond ? el : null` appears as the word "null" (and
// `cond && el` as "false"). Such children are passed to mount from dom.js,
// which skips them, as h does. The check reads the sources; it runs in
// Node without a browser (ADR-0133).
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { join } from "node:path";

const assets = fileURLToPath(new URL("../static/assets/", import.meta.url));

// topLevel splits the text between a call's parentheses into its
// arguments, skipping strings, template literals and nested brackets. It
// answers the arguments and where the call's closing parenthesis is.
function topLevel(src, open) {
  const args = [];
  let depth = 0, quote = null, start = open;
  for (let i = open; i < src.length; i++) {
    const c = src[i];
    if (quote) {
      if (c === "\\") i++;
      else if (c === quote) quote = null;
      continue;
    }
    if (c === '"' || c === "'" || c === "`") quote = c;
    else if (c === "/" && src[i + 1] === "/") {
      // A line comment: up to its end.
      const end = src.indexOf("\n", i);
      if (end < 0) break;
      i = end;
    }
    else if ("([{".includes(c)) depth++;
    else if (")]}".includes(c)) {
      if (depth === 0) { args.push(src.slice(start, i)); return { args, end: i }; }
      depth--;
    } else if (c === "," && depth === 0) { args.push(src.slice(start, i)); start = i + 1; }
  }
  return { args, end: src.length };
}

// leftOut says whether an argument may be null, undefined or false rather
// than a node or a string.
const leftOut = (arg) => /:\s*(null|undefined)\s*$/.test(arg.trim()) || /&&\s*h\(/.test(arg) && !/\?/.test(arg);

// offences lists the calls of replaceChildren in src that pass such an
// argument, by line.
function offences(src) {
  const out = [];
  const call = "replaceChildren(";
  for (let i = src.indexOf(call); i >= 0; i = src.indexOf(call, i + 1)) {
    const { args } = topLevel(src, i + call.length);
    if (args.some(leftOut)) out.push(src.slice(0, i).split("\n").length);
  }
  return out;
}

test("the guard finds a child left out with null, and passes the rest", () => {
  assert.deepEqual(offences('box.replaceChildren(\n  h("h3"),\n  editable ? h("button") : null);'), [1]);
  assert.deepEqual(offences("box.replaceChildren(text, ok && h(\"b\"));"), [1]);
  assert.deepEqual(offences('box.replaceChildren(h("p", { text: a ? "x" : null }), h("b"));'), []);
  assert.deepEqual(offences("box.replaceChildren(...[a, b ? c : null].filter(Boolean));"), []);
  assert.deepEqual(offences('box.replaceChildren(h("p", { text: "a, b: null" }));'), []);
});

test("no UI module passes a child that may be null to replaceChildren", () => {
  const found = [];
  for (const name of readdirSync(assets).filter((n) => n.endsWith(".js"))) {
    for (const line of offences(readFileSync(join(assets, name), "utf8"))) found.push(`${name}:${line}`);
  }
  assert.deepEqual(found, [], "use mount from dom.js, which skips null, at: " + found.join(", "));
});
