// The parts of the dialog that moves a page with its child pages
// (movepage.js, ADR-0067, ADR-draft-a-page-is-moved-into-another-room-from-its-head-and-its-tree):
// which rooms it offers, what it reads, and how it names the place chosen.
// Nothing here touches the page, so it is tested without a browser.

// moveRooms returns the rooms a page may go to: those one may create pages
// in, one's favourites first, each group in its order (ADR-0145). The
// server still decides; this only leaves out what it would refuse anyway.
export function moveRooms(spaces) {
  const editable = spaces.filter((s) => s.role === "editor" || s.role === "space-admin");
  return [...editable.filter((s) => s.favourite), ...editable.filter((s) => !s.favourite)];
}

// treeURL reads one level of a room's page tree (ADR-0041): the top, or the
// children of parent, from the page after after.
export function treeURL(space, parent, after) {
  const q = new URLSearchParams();
  if (parent) q.set("parent", parent);
  if (after) q.set("after", after);
  return `/api/spaces/${encodeURIComponent(space)}/tree` + (q.size ? "?" + q : "");
}

// findURL finds the items of a room whose key, title, type, state or labels
// hold every word typed.
export function findURL(space, words) {
  return "/api/items?" + new URLSearchParams({ space, q: words.trim().replace(/\s+/g, " "), limit: "20" });
}

// belowItself says whether a place lies on the page being moved or below
// it: trail is the target followed by the items above it, nearest first.
// The server refuses such a move; the dialog says so before it is sent.
export function belowItself(id, trail) {
  return trail.includes(id);
}

// placeTrail names a place from the room down to the target; titles are
// the target's and those above it, nearest first.
export function placeTrail(room, titles) {
  return [room, ...[...titles].reverse()].join(" › ");
}

// crossesRoom says whether a move takes the page into another room, whose
// access then applies.
export function crossesRoom(item, space) {
  return item.space !== space;
}
