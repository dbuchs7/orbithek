// Unit tests of the views of the items of a type (itemviews.js, ADR-0201)
// and the order of a label's page set by hand (ADR-0202): the address and
// the block, the columns, the groupings and groups, the sort, the cover of
// a card and the text of a value of any kind. They run in Node.js without
// a browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// The view itself is in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import { FIELD, NONE, address, columnsOf, coverOf, defaultColumns, editableColumn, fieldText, fromBlock, groupChoices, groupItems, listURL,
  moveWithin, readState, shownColumns, sortItems, toBlock, transitionsFrom, valueOf } from "../static/assets/itemviews.js";

const task = {
  name: "task",
  fields: [{ name: "priority", kind: "enum", values: ["low", "normal", "high"] }, { name: "assignee", kind: "user" }, { name: "due", kind: "date" },
    { name: "done", kind: "bool" }, { name: "estimate", kind: "number" }, { name: "cover", kind: "file" }],
  workflow: { initial: "open", states: ["open", "in progress", "done"], transitions: [{ name: "start", from: ["open"], to: "in progress" }, { name: "finish", from: ["open", "in progress"], to: "done" }] },
};
const page = { name: "page", fields: [{ name: "owner", kind: "user" }] };
const it = (key, props = {}, extra = {}) => ({ id: "id-" + key, key, title: "T " + key, properties: props, labels: [], state: "open", ...extra });

test("the address keeps the state, leaving out the defaults", () => {
  const state = { layout: "gallery", space: "PLAT", type: "task", query: "state = open", group: "field:priority", columns: ["title", "field:due date"], sort: "-updated" };
  const back = readState(new URL("http://x/" + address(state).replace("#/view", "")).searchParams);
  assert.deepEqual(back, state);
  assert.equal(address({ layout: "table", columns: [] }), "#/view");
  const odd = readState(new URLSearchParams("layout=board&col=key&col=key&col=colour&col=field:"));
  assert.equal(odd.layout, "table");
  assert.deepEqual(odd.columns, ["key"]);
});

test("a view goes into a text as a block and comes back", () => {
  const state = { layout: "list", space: "", type: "task", query: "", group: "state", columns: ["key"], sort: "" };
  const b = toBlock(state);
  assert.deepEqual(b, { kind: "view", layout: "list", itemType: "task", group: "state", columns: ["key"] });
  assert.deepEqual(fromBlock(b), state);
  assert.equal(toBlock({ layout: "nonsense" }).layout, "table");
  assert.equal(fromBlock({ layout: "x" }).layout, "table");
});

test("a type offers its columns; a view shows those it names or the default", () => {
  assert.ok(columnsOf(task).includes("state"));
  assert.ok(!columnsOf(page).includes("state"));
  assert.ok(columnsOf(task).includes(FIELD + "estimate"));
  assert.deepEqual(defaultColumns(task), ["key", "title", "state", "field:priority", "field:assignee", "field:due", "field:done", "labels", "updated"]);
  assert.deepEqual(defaultColumns(null), ["key", "title", "labels", "updated"]);
  assert.deepEqual(shownColumns({ columns: ["title", "field:gone"] }, task), ["title"]);
  assert.deepEqual(shownColumns({ columns: ["field:gone"] }, task), defaultColumns(task));
  assert.deepEqual(shownColumns({ columns: ["field:any"] }, null), ["field:any"]);
});

test("a view is grouped by the state, the labels and fields of a few values", () => {
  assert.deepEqual(groupChoices(task), ["state", "labels", "field:priority", "field:assignee", "field:done"]);
  assert.deepEqual(groupChoices(page), ["labels", "field:owner"]);
  assert.deepEqual(groupChoices(null), ["state", "labels"]);
});

test("groups follow the type's order, the empty group last", () => {
  const items = [it("A", { priority: "high" }), it("B"), it("C", { priority: "low" }), it("D", { priority: "high" }), it("E", { priority: "urgent" })];
  const g = groupItems(items, "field:priority", task);
  assert.deepEqual(g.map((x) => [x.key, x.items.map((i) => i.key)]), [["low", ["C"]], ["high", ["A", "D"]], ["urgent", ["E"]], [NONE, ["B"]]]);
  const states = groupItems([it("A", {}, { state: "done" }), it("B")], "state", task);
  assert.deepEqual(states.map((x) => x.key), ["open", "done"]);
  const bools = groupItems([it("A", { done: false }), it("B", { done: true }), it("C")], "field:done", task, { text: (v) => (v === true ? "yes" : "no") });
  assert.deepEqual(bools.map((x) => [x.name, x.items.length]), [["yes", 1], ["no", 1], ["", 1]]);
  const labels = groupItems([it("A", {}, { labels: ["b", "a"] }), it("B", {}, { labels: ["a"] }), it("C")], "labels", task);
  assert.deepEqual(labels.map((x) => [x.key, x.items.map((i) => i.key)]), [["a", ["A", "B"]], ["b", ["A"]], [NONE, ["C"]]]);
  const users = groupItems([it("A", { assignee: "u2" }), it("B", { assignee: "u1" })], "field:assignee", task, { text: (v) => ({ u1: "zoe", u2: "adam" })[v] });
  assert.deepEqual(users.map((x) => x.name), ["adam", "zoe"]);
  assert.deepEqual(groupItems(items, "", task).map((x) => x.items.length), [5]);
});

test("items sort by a column either way, empty values last, or by the order set by hand", () => {
  const items = [it("A", { estimate: 3, due: "2026-12-01T00:00:00Z" }), it("B", { estimate: 10 }), it("C", { due: "2026-11-01T00:00:00Z" }), it("D10"), it("D9")];
  assert.deepEqual(sortItems(items, "field:estimate", task).map((x) => x.key), ["A", "B", "C", "D10", "D9"]);
  assert.deepEqual(sortItems(items, "-field:estimate", task).map((x) => x.key), ["B", "A", "C", "D10", "D9"]);
  assert.deepEqual(sortItems(items, "field:due", task).map((x) => x.key), ["C", "A", "B", "D10", "D9"]);
  assert.deepEqual(sortItems(items, "key", task).map((x) => x.key), ["A", "B", "C", "D9", "D10"]);
  assert.deepEqual(sortItems(items, "", task, { order: ["id-D9", "id-B"] }).map((x) => x.key), ["D9", "B", "A", "C", "D10"]);
  assert.deepEqual(sortItems(items, "", task).map((x) => x.key), ["A", "B", "C", "D10", "D9"]);
  const updated = [it("A", {}, { updated: "2026-11-02T10:00:00Z" }), it("B", {}, { updated: "2026-11-03T10:00:00Z" })];
  assert.deepEqual(sortItems(updated, "-updated", task).map((x) => x.key), ["B", "A"]);
});

test("values are written by their kind, an unknown kind as text", () => {
  const f = (kind, extra) => ({ name: "x", kind, ...extra });
  assert.equal(fieldText(f("bool"), true, { yes: "ja", no: "nein" }), "ja");
  assert.equal(fieldText(f("bool"), false, { yes: "ja", no: "nein" }), "nein");
  assert.equal(fieldText(f("user"), "u1", { person: (id) => "name of " + id }), "name of u1");
  assert.equal(fieldText(f("date"), "2026-11-03T00:00:00Z"), "2026-11-03");
  assert.equal(fieldText(f("datetime"), "2026-11-03T10:00:00Z", { moment: () => "local" }), "local");
  assert.equal(fieldText(f("number"), 4.5), "4.5");
  assert.equal(fieldText(f("rating"), 3), "★★★");
  assert.equal(fieldText(f("multi"), ["a", "b"]), "a, b");
  assert.equal(fieldText(f("item"), { id: "x1", title: "Plan" }), "Plan");
  assert.equal(fieldText(f("someday"), { other: 1 }), '{"other":1}');
  assert.equal(fieldText(null, 5), "5");
  assert.equal(fieldText(f("text"), null), "");
  assert.deepEqual(valueOf(it("A"), "labels"), []);
  assert.equal(valueOf(it("A", { due: "x" }), "field:due"), "x");
  assert.equal(valueOf(it("A"), "field:due"), null);
});

test("a card's cover is the text's first picture, else a file field's file", () => {
  assert.deepEqual(coverOf(it("A", {}, { body: [{ kind: "paragraph", text: "x" }, { kind: "image", ref: "f1" }, { kind: "image", ref: "f2" }] }), task), { item: "id-A", attachment: "f1" });
  assert.deepEqual(coverOf(it("A", { cover: "f9" }), task), { item: "id-A", attachment: "f9" });
  assert.deepEqual(coverOf(it("A", { cover: [{ id: "f8", item: "other" }] }), task), { item: "other", attachment: "f8" });
  assert.equal(coverOf(it("A"), task), null);
  assert.equal(coverOf(it("A"), null), null);
});

test("moving within an order set by hand", () => {
  assert.deepEqual(moveWithin(["a", "b", "c", "d"], ["d"], "b"), ["a", "d", "b", "c"]);
  assert.deepEqual(moveWithin(["a", "b", "c"], ["a", "b"], ""), ["c", "a", "b"]);
  assert.deepEqual(moveWithin(["a", "b"], ["c"], "a"), ["c", "a", "b"]);
  assert.deepEqual(moveWithin(["a", "b"], ["a"], "gone"), ["b", "a"]);
});

test("cells of the title, of fields the item page edits and of the state change in place", () => {
  assert.ok(editableColumn("title", task));
  assert.ok(editableColumn("state", task));
  assert.ok(!editableColumn("state", page));
  assert.ok(editableColumn("field:priority", task));
  assert.ok(!editableColumn("field:cover", task));
  assert.ok(!editableColumn("labels", task));
  assert.ok(!editableColumn("updated", task));
  assert.deepEqual(transitionsFrom(task.workflow, "open").map((tr) => tr.name), ["start", "finish"]);
  assert.deepEqual(transitionsFrom(null, "open"), []);
});

test("the listing a view reads", () => {
  assert.equal(listURL({ space: "PLAT", type: "task", query: "state = open" }), "/api/items?limit=100&space=PLAT&type=task&query=state+%3D+open");
  assert.equal(listURL({}, "onboarding"), "/api/items?limit=100&label=onboarding");
});
