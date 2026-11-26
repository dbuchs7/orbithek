// A starmap in a text (ADR-0252): a block of the
// kind starmap names the map it shows and how it is narrowed at first —
// the room, a label, a search text, the types and kinds of node it leaves
// out and the reach of the neighbourhood of the item it is on. Every
// reader's browser draws the map the server derives for them
// (starmap.js), so a text shows each reader only what they read; the
// reader may change the filters while reading, and nothing of that is
// kept. A click on an item opens it.
//
// The page shares what this module needs once (configureStarmaps): the
// symbols of the page tree (ADR-0250) and its dialog.
import { h } from "./dom.js";
import { t } from "./i18n.js";
import { get } from "./api.js";
import { BLOCK_ALL, BLOCK_KINDS, DEPTHS, starmapBlockOf } from "./starmap-graph.js";
import { embeddedStarmap } from "./starmap.js";
import { normalLabel } from "./timeline.js";

let shared = null;

// configureStarmaps takes what the page shares: symbols(), which answers
// the task types and finished states for the page tree's symbols, and
// formDialog(title, id, fields, submit, ok), the page's dialog.
export function configureStarmaps(helpers) { shared = helpers; }
export const starmapsReady = () => shared !== null;

// The names of the kinds a block may leave out, with their keys written
// out so the catalogue's tests find them (ADR-0017).
const kindName = (k) => ({ done: t("starmap.kindDone"), label: t("starmap.kindLabel"), outside: t("starmap.kindOutside"), picture: t("starmap.kindPicture") })[k];

// starmapBlockElement draws the starmap a block names for the item here
// ({ id, space }), as far as the reader reads it.
export async function starmapBlockElement(block, here) {
  const symbols = await Promise.resolve(shared?.symbols ? shared.symbols() : {}).catch(() => ({}));
  return embeddedStarmap(block, { here, symbols });
}

// starmapSummary says in a line what a block shows, beside its button.
export function starmapSummary(block) {
  const room = block.space === BLOCK_ALL ? t("starmap.allRooms") : block.space || t("starmap.blockOwnRoom");
  const parts = [room];
  if (block.depth) parts.push(t("starmap.blockAround", { n: block.depth }));
  if (block.label) parts.push(t("starmap.blockLabel", { label: block.label }));
  if (block.search) parts.push(t("starmap.blockSearch", { text: block.search }));
  const left = [...(block.hideTypes || []), ...(block.hideKinds || []).map(kindName)];
  if (left.length) parts.push(t("starmap.blockWithout", { names: left.join(", ") }));
  return parts.join(" · ");
}

// pickStarmap asks how a starmap in a text is narrowed at first, starting
// from initial (a block), and answers the block, or null when cancelled.
export async function pickStarmap(initial = {}, helpers = shared) {
  const [{ spaces }, { types }, labels] = await Promise.all([get("/api/spaces"), get("/api/types"),
    get("/api/labels").then((r) => r.labels || []).catch(() => [])]);
  const choice = (label, input) => h("label", { class: "field" }, h("span", { text: label }), input);
  const room = h("select", { id: "starmap-pick-space", name: "space" },
    h("option", { value: "", text: t("starmap.blockOwnRoom") }), h("option", { value: BLOCK_ALL, text: t("starmap.allRooms") }),
    spaces.map((r) => h("option", { value: r.key, text: r.name ? `${r.key} – ${r.name}` : r.key })));
  room.value = initial.space || "";
  const depth = h("select", { id: "starmap-pick-depth", name: "depth" },
    h("option", { value: "0", text: t("starmap.blockWhole") }),
    DEPTHS.map((d) => h("option", { value: String(d), text: t("starmap.blockAroundChoice", { n: d }) })));
  depth.value = String(initial.depth || 0);
  const label = h("input", { id: "starmap-pick-label", name: "label", type: "text", list: "starmap-pick-labels", value: initial.label || "", maxlength: 64, autocomplete: "off" });
  const labelList = h("datalist", { id: "starmap-pick-labels" }, labels.map((l) => h("option", { value: l.label })));
  const search = h("input", { id: "starmap-pick-search", name: "search", type: "search", value: initial.search || "", maxlength: 200 });
  const box = (key, text, on) => h("label", { class: "check", "data-hide": key }, h("input", { type: "checkbox", value: key, checked: on }), " ", text);
  const typeBoxes = h("fieldset", { class: "starmap-pick-types", id: "starmap-pick-types" }, h("legend", { text: t("starmap.blockHideTypes") }),
    [...new Set(types.map((ty) => ty.name))].sort().map((name) => box(name, name, (initial.hideTypes || []).includes(name))));
  const kindBoxes = h("fieldset", { class: "starmap-pick-kinds", id: "starmap-pick-kinds" }, h("legend", { text: t("starmap.blockHideKinds") }),
    BLOCK_KINDS.map((k) => box(k, kindName(k), (initial.hideKinds || []).includes(k))));
  const ticked = (set) => [...set.querySelectorAll("input:checked")].map((i) => i.value);
  return helpers.formDialog(t("starmap.blockInsert"), "starmap-dialog", [
    choice(t("starmap.space"), room),
    choice(t("starmap.blockReach"), depth),
    choice(t("starmap.blockLabelField"), label), labelList,
    choice(t("starmap.search"), search),
    typeBoxes, kindBoxes,
    h("p", { class: "muted", text: t("starmap.blockHint") }),
  ], async () => starmapBlockOf({ space: room.value, label: normalLabel(label.value), search: search.value, depth: Number(depth.value),
    hideTypes: ticked(typeBoxes), hideKinds: ticked(kindBoxes) }), t("starmap.blockOk"));
}
