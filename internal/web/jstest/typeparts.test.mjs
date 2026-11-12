// Unit tests of the type editor's parts (typeparts.js, ADR-0199,
// ADR-0200): drafts and definitions, what is wrong with a draft, start
// texts as Markdown and how values of the new field kinds read. They run in Node.js
// without a browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// The form itself is in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  fieldKinds, usableIn, presetWorkflow, emptyDraft, newField, draftOf, definitionOf, problems,
  stars, valueText, moved, renamedState,
} from "../static/assets/typeparts.js";

test("every kind of field is offered, the new ones too", () => {
  for (const k of ["text", "enum", "datetime", "item", "multi", "rating", "file"]) assert.ok(fieldKinds.includes(k), k);
});

test("a space's items may have the installation's types and its own", () => {
  const types = [{ name: "page" }, { name: "incident", space: "OPS" }, { name: "story", space: "WEB" }];
  assert.deepEqual(usableIn(types, "OPS").map((t) => t.name), ["page", "incident"]);
});

test("a draft becomes a definition with only what each kind uses", () => {
  const d = emptyDraft("OPS");
  d.name = " incident ";
  d.fields = [
    { ...newField(), name: "severity", kind: "enum", values: ["low", " high ", "", "low"], targets: ["page"], unique: true },
    { ...newField(), name: "about", kind: "item", targets: ["page"], values: ["x"] },
    { ...newField(), name: "code", kind: "text", unique: true, required: true, recommended: true },
    { ...newField(), name: "platforms", kind: "multi", values: ["web", "ios"] },
  ];
  d.workflow = presetWorkflow();
  d.startText = "## Impact\n\n[ ] Inform the users";
  assert.deepEqual(definitionOf(d), {
    name: "incident", space: "OPS", body: true,
    fields: [
      { name: "severity", kind: "enum", values: ["low", "high"] },
      { name: "about", kind: "item", targets: ["page"] },
      { name: "code", kind: "text", required: true, unique: true },
      { name: "platforms", kind: "multi", values: ["web", "ios"] },
    ],
    workflow: presetWorkflow(),
    startMarkdown: "## Impact\n\n[ ] Inform the users",
  });
  d.body = false;
  assert.equal(definitionOf(d).startMarkdown, undefined, "a type without a text has no start text");
});

test("a type's definition reads back as its draft", () => {
  const type = {
    name: "story", version: 3, space: "WEB", body: true,
    fields: [{ name: "epic", kind: "item", targets: ["page"] }, { name: "rating", kind: "rating" }],
    workflow: presetWorkflow(), start: [{ kind: "bullet", text: "one" }], startMarkdown: "- one", template: "letter",
  };
  const d = draftOf(type);
  assert.equal(d.existing, true);
  assert.equal(d.startText, "- one");
  assert.deepEqual(definitionOf(d), { name: "story", space: "WEB", body: true, fields: type.fields, workflow: type.workflow, template: "letter", startMarkdown: "- one" });
  assert.equal(draftOf({ name: "task", fields: [] }, "OPS").space, "OPS", "a template is added to a room");
});

test("a workflow names only its states", () => {
  const d = emptyDraft();
  d.name = "t";
  d.workflow = { initial: "gone", states: ["open", "done"], transitions: [{ name: "finish", from: ["open", "gone"], to: "done" }, { name: "", from: [], to: "" }] };
  const w = definitionOf(d).workflow;
  assert.equal(w.initial, "open");
  assert.deepEqual(w.transitions, [{ name: "finish", from: ["open"], to: "done" }]);
});

test("what is wrong with a draft is named before it is sent", () => {
  const d = emptyDraft();
  d.fields = [{ ...newField(), name: "title" }, { ...newField(), name: "a" }, { ...newField(), name: "a", kind: "multi" }, newField()];
  d.workflow = { initial: "", states: ["x", "x"], transitions: [{ name: "go", from: [], to: "y" }] };
  const keys = problems(d).map((p) => p.key);
  for (const k of ["types.problemName", "types.problemReserved", "types.problemTwice", "types.problemValues", "types.problemFieldName", "types.problemStateTwice", "types.problemTransitionStates"]) {
    assert.ok(keys.includes(k), k);
  }
  const ok = emptyDraft();
  ok.name = "fine";
  ok.workflow = presetWorkflow();
  assert.deepEqual(problems(ok), []);
});

test("values of the new kinds read as one line", () => {
  assert.equal(stars(3), "★★★☆☆");
  assert.equal(stars(9), "★★★★★");
  assert.equal(valueText({ kind: "multi" }, ["web", "ios"]), "web, ios");
  assert.equal(valueText({ kind: "rating" }, 2), "★★☆☆☆");
  assert.equal(valueText({ kind: "item" }, "id1", { ref: () => "PLAT-1 Epic" }), "PLAT-1 Epic");
  assert.equal(valueText({ kind: "file" }, "f1", { file: () => "cover.png" }), "cover.png");
  assert.equal(valueText({ kind: "text" }, null), "");
});

test("entries move and states are renamed everywhere", () => {
  assert.deepEqual(moved([1, 2, 3], 0, 1), [2, 1, 3]);
  assert.deepEqual(moved([1, 2, 3], 0, -1), [1, 2, 3]);
  assert.deepEqual(renamedState(presetWorkflow(), "open", "new"), {
    initial: "new", states: ["new", "done"],
    transitions: [{ name: "finish", from: ["new"], to: "done" }, { name: "reopen", from: ["done"], to: "new" }],
  });
});
