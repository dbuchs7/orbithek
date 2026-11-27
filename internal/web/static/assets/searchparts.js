// The parts of a search field in a text (ADR-0264)
// that need no browser: the scopes it offers, the block a choice makes,
// the search address it asks and how the reader's choice of scope maps
// onto the page or room it searches, narrowed by the block's label and
// type, which the reader does not change. searchblock.js draws them.

// SCOPES are what a search field searches, in the order they are offered:
// "" the room, "page" the page alone, "tree" the page and every page
// below it, "below" the pages below it only. The keys equal the server's
// (item.SearchScopes).
export const SCOPES = ["", "page", "tree", "below"];

// searchBlockOf makes the block a choice describes: a room's search keeps
// its room (none for the room of the item), a page's search its scope and
// page (none for the item itself), and either the label and the type its
// hits carry (none for every one). What does not belong to the scope is
// dropped, so the block says it one way, as the server asks.
export function searchBlockOf(spec = {}, self = "") {
  const scope = SCOPES.includes(spec.scope) ? spec.scope : "";
  const b = { kind: "search" };
  if (!scope) {
    if (spec.space) b.space = spec.space;
  } else {
    b.scope = scope;
    if (spec.ref && spec.ref !== self) b.ref = spec.ref;
  }
  if (spec.label) b.label = spec.label;
  if (spec.itemType) b.itemType = spec.itemType;
  return b;
}

// narrowed adds the block's label and type to a target, as far as it
// names them.
const narrowed = (target, block) => ({ ...target, ...(block.label ? { label: block.label } : {}), ...(block.itemType ? { itemType: block.itemType } : {}) });

// searchTarget answers what a search field searches when the reader
// chose scope, for the item here ({ id, space }): the room's key for a
// room, or the page's ID for a page. A block of a room searches the item
// itself when the reader chooses a page, and a block of a page searches
// the item's own room when the reader chooses the room. The block's label
// and type narrow every scope.
export function searchTarget(block, scope, here = {}) {
  if (!scope) return narrowed({ scope: "", space: block.scope ? here.space || "" : block.space || here.space || "" }, block);
  return narrowed({ scope, within: (block.scope && block.ref) || here.id || "" }, block);
}

// searchUrl is the address of the full-text search (ADR-0033) a search
// field asks for text, narrowed to target (searchTarget) and its label
// and type, a page of limit hits after the cursor after.
export function searchUrl(text, target, { limit = 10, after = "" } = {}) {
  const p = new URLSearchParams({ q: text.trim(), limit: String(limit) });
  if (target.scope && target.within) {
    p.set("within", target.within);
    p.set("scope", target.scope);
  } else if (target.space) {
    p.set("space", target.space);
  }
  if (target.label) p.set("label", target.label);
  if (target.itemType) p.set("type", target.itemType);
  if (after) p.set("after", after);
  return "/api/search?" + p;
}

// searchable reports whether text holds something to search for: a
// letter or a digit, as the server asks.
export const searchable = (text) => /[\p{L}\p{N}]/u.test(text || "");
