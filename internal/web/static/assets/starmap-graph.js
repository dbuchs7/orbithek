// The starmap's graph without a browser (ADR-0157, ADR-0158, ADR-0159):
// which kind and group a node and an edge belong to, what the filters
// keep, a node's neighbourhood at a depth, the drill path, which captions
// a zoom shows, the frame that fits a set of points, the seeded layout,
// and an impact walk's answer as a map, a list and an address. The seeded
// layout is a force simulation with a fixed seed and a fixed number of
// steps, as atlas's panorama has it: the same map starts in the same place
// on every load. The live simulation (starmap-sim.js) takes it from there.
//
// Plain functions over plain data, so node --test runs them
// (internal/web/jstest/starmap.test.mjs).

// GROUPS are the edge groups the filters switch, each with its kinds.
export const GROUPS = {
  tree: ["parent"],
  relations: ["relation", "mentions", "embeds"],
  pictures: ["picture", "copy"],
  labels: ["label"],
};

// groupOf names the group of an edge kind.
export function groupOf(kind) {
  return Object.keys(GROUPS).find((g) => GROUPS[g].includes(kind)) || "relations";
}

// kindOf is the filter a node falls under: an item by its type (types are
// the users' own, never named here), a picture or a label. Outside nodes
// have a switch of their own besides (outsideKind).
export function kindOf(node) {
  if (node.kind === "item" || node.kind === "group") return "type:" + node.type;
  return node.kind;
}

// groupRadius is how big a group of the starmap of all rooms is drawn
// (ADR-0191): its area grows with the number of its items, from an
// item's size up to a bound.
// CLOUD_BOTTOM is where a group's cloud stands, as a share of its
// radius below its middle (ADR-0230).
export const CLOUD_BOTTOM = 0.5;

// cloudLobes are the end points of a cloud's arcs in a circle of radius
// one, from the bottom left round its top to the bottom right, each with
// the radius of the lobe that reaches it and whether that lobe is the
// larger arc. The bottom is flat.
const cloudLobes = [
  { x: -0.85, y: -0.05, r: 0.34, large: 1 },
  { x: -0.3, y: -0.55, r: 0.4, large: 0 },
  { x: 0.4, y: -0.6, r: 0.42, large: 0 },
  { x: 0.85, y: 0, r: 0.36, large: 0 },
  { x: 0.7, y: CLOUD_BOTTOM, r: 0.32, large: 1 },
];

// cloudPath is the SVG path of a group's cloud of radius r: lobes along
// its top and a flat bottom, within the circle of radius r, so the map
// keeps the room a bubble took.
export function cloudPath(r) {
  const n = (v) => String(Math.round(v * r * 10) / 10);
  return `M${n(-0.7)} ${n(CLOUD_BOTTOM)}` + cloudLobes.map((l) => `A${n(l.r)} ${n(l.r)} 0 ${l.large} 1 ${n(l.x)} ${n(l.y)}`).join("") + "Z";
}

// mergeLabelCounts joins lists of {label, count}, the labels of items and
// of pictures (ADR-0090), into one list counting each label once, the most
// carried first, then by name.
export function mergeLabelCounts(...lists) {
  const count = new Map();
  for (const list of lists) for (const { label, count: n } of list) count.set(label, (count.get(label) || 0) + n);
  return [...count].map(([label, n]) => ({ label, count: n })).sort((a, b) => b.count - a.count || (a.label < b.label ? -1 : a.label > b.label ? 1 : 0));
}

export function groupRadius(count) {
  return Math.min(64, 11 + 4 * Math.sqrt(Math.max(0, Number(count) || 0)));
}

export const outsideKind = "outside";

// kinds lists the node filters the map needs: every item type on it in
// the order of the alphabet, then pictures, labels and outside nodes when
// it has them.
export function kinds(nodes) {
  const types = [...new Set(nodes.filter((n) => n.kind === "item" || n.kind === "group").map((n) => n.type))].sort();
  const out = types.map((type) => ({ key: "type:" + type, type }));
  for (const k of ["picture", "label"]) if (nodes.some((n) => n.kind === k)) out.push({ key: k });
  if (nodes.some((n) => n.outside)) out.push({ key: outsideKind });
  return out;
}

// toneOf gives each item type of the map a tone, 0 to 5, by its place
// among the types: the colour that tells types apart besides the legend.
export function toneOf(nodes) {
  const types = kinds(nodes).filter((k) => k.type !== undefined).map((k) => k.type);
  return (type) => Math.max(0, types.indexOf(type)) % 6;
}

// roomToneOf gives each room with clouds on the map a tone, 0 to 5, by
// the place of its key among those rooms, so the clouds of one room
// share a colour; beyond six rooms the tones repeat (ADR-0232).
export function roomToneOf(nodes) {
  const rooms = [...new Set(nodes.filter((n) => n.kind === "group").map((n) => n.space))].sort();
  return (space) => Math.max(0, rooms.indexOf(space)) % 6;
}

// roomLinks ties the clouds of each room together, the room's first
// cloud to each of the others, as edges of the kind room whose rest is
// the two clouds side by side. They are not drawn: the layout and the
// simulation pull a room's clouds together so its outline encloses them
// alone (ADR-draft-the-clouds-of-a-room-are-outlined).
export function roomLinks(nodes) {
  const first = new Map();
  const links = [];
  for (const n of nodes) {
    if (n.kind !== "group") continue;
    if (!first.has(n.space)) first.set(n.space, n);
    else {
      const f = first.get(n.space);
      links.push({ from: f.id, to: n.id, kind: "room", rest: Math.round(groupRadius(f.count) + groupRadius(n.count) + 8) });
    }
  }
  return links;
}

// roomStart is where the clouds of the starmap of all rooms start the
// layout: each room around a centre of its own, the centres on a spiral
// round mid in the order of the rooms' keys, so a room's clouds start
// together and its outline does not cross another's.
export function roomStart(nodes, mid) {
  const rooms = new Map();
  for (const n of nodes) if (n.kind === "group") (rooms.get(n.space) || rooms.set(n.space, []).get(n.space)).push(n.id);
  const start = new Map();
  [...rooms.keys()].sort().forEach((space, k) => {
    const angle = k * 2.399963, r = k ? 160 * Math.sqrt(k) : 0;
    const cx = mid + r * Math.cos(angle), cy = mid + r * Math.sin(angle);
    const ids = rooms.get(space);
    ids.forEach((id, j) => {
      const a = (j * 2 * Math.PI) / ids.length, rr = ids.length > 1 ? 45 : 0;
      start.set(id, { x: cx + rr * Math.cos(a), y: cy + rr * Math.sin(a) });
    });
  });
  return start;
}

// roomHull is the SVG path of a room's outline: the convex hull of its
// clouds' circles, each widened by pad, sampled at sixteen points; empty
// when the room shows no cloud.
export function roomHull(circles, pad = 14) {
  if (!circles.length) return "";
  const pts = [];
  for (const c of circles) for (let k = 0; k < 16; k++) {
    const a = (k * Math.PI) / 8;
    pts.push([c.x + (c.r + pad) * Math.cos(a), c.y + (c.r + pad) * Math.sin(a)]);
  }
  pts.sort((p, q) => p[0] - q[0] || p[1] - q[1]);
  const cross = (o, a, b) => (a[0] - o[0]) * (b[1] - o[1]) - (a[1] - o[1]) * (b[0] - o[0]);
  const half = (list) => {
    const h = [];
    for (const p of list) {
      while (h.length >= 2 && cross(h[h.length - 2], h[h.length - 1], p) <= 0) h.pop();
      h.push(p);
    }
    h.pop();
    return h;
  };
  const hull = [...half(pts), ...half([...pts].reverse())];
  const r1 = (v) => Math.round(v * 10) / 10;
  return "M" + hull.map(([x, y]) => `${r1(x)} ${r1(y)}`).join("L") + "Z";
}

const fold = (s) => String(s || "").normalize("NFKD").replace(/\p{M}/gu, "").toLowerCase();

// matches reports whether a node holds every word of the query in its
// title, key, labels or name.
export function matches(node, query) {
  const words = fold(query).split(/\s+/).filter(Boolean);
  if (!words.length) return false;
  const text = fold([node.title, node.key, node.name, ...(node.labels || [])].join(" "));
  return words.every((w) => text.includes(w));
}

// filter applies the filters: hidden is a set of node kinds and edge
// groups switched off. A node shows when its kind is on (and, outside, the
// outside switch); an edge when its group is on and both ends show. With
// a query, only the matches show and what one shown edge joins to them.
// It returns the ids of the nodes shown, the indexes of the edges shown
// and the ids of the matches.
export function filter(graph, { query = "", hidden = new Set() } = {}) {
  const on = new Set(graph.nodes.filter((n) => !hidden.has(kindOf(n)) && !(n.outside && hidden.has(outsideKind))).map((n) => n.id));
  const edgeOn = (e) => !hidden.has(groupOf(e.kind)) && on.has(e.from) && on.has(e.to);
  const found = new Set();
  let shown = on;
  if (fold(query).trim()) {
    for (const n of graph.nodes) if (on.has(n.id) && matches(n, query)) found.add(n.id);
    shown = new Set(found);
    for (const e of graph.edges) {
      if (!edgeOn(e)) continue;
      if (found.has(e.from)) shown.add(e.to);
      if (found.has(e.to)) shown.add(e.from);
    }
  }
  const edges = [];
  graph.edges.forEach((e, i) => { if (edgeOn(e) && shown.has(e.from) && shown.has(e.to)) edges.push(i); });
  return { nodes: shown, edges, matches: found };
}

// neighbourhood is a node and every node one shown edge joins to it.
export function neighbourhood(graph, id, edges = graph.edges.map((_, i) => i)) {
  return around(graph, id, 1, edges);
}

// around is a node and every node at most depth shown edges away from it,
// in either direction.
export function around(graph, id, depth = 1, edges = graph.edges.map((_, i) => i)) {
  const next = new Map();
  const link = (a, b) => { if (next.has(a)) next.get(a).push(b); else next.set(a, [b]); };
  for (const i of edges) {
    const e = graph.edges[i];
    link(e.from, e.to);
    link(e.to, e.from);
  }
  const out = new Set([id]);
  let frontier = [id];
  for (let d = 0; d < depth && frontier.length; d++) {
    const found = [];
    for (const n of frontier) for (const m of next.get(n) || []) if (!out.has(m)) { out.add(m); found.push(m); }
    frontier = found;
  }
  return out;
}

// DEPTHS are the reaches a separated neighbourhood may have.
export const DEPTHS = [1, 2, 3];

// drillInto is the drill path after separating id: appended, or cut back
// to id when id is on the path already, so the path never holds a node
// twice (as atlas's drilldown has it).
export function drillInto(trail, id) {
  const seen = trail.indexOf(id);
  return seen >= 0 ? trail.slice(0, seen + 1) : [...trail, id];
}

// startRoom is the room the starmap opens on: the one the address names
// when one reads it, else every room ("*", ADR-0191, ADR-0228).
export function startRoom(space, rooms) {
  return space === "*" || rooms.some((r) => r.key === space) ? space : "*";
}

// roomAddress is where a double-click on a node leads instead of
// separating it: a bubble of the starmap of every room opens the map of
// its room, as its panel's button does; any other node has none (null).
export function roomAddress(node) {
  return node.kind === "group" && node.space ? "#/starmap/" + encodeURIComponent(node.space) : null;
}

// drillBack is the drill path one step back, or back to the station at
// index to (-1 for the whole room).
export function drillBack(trail, to = trail.length - 2) {
  return to < 0 ? [] : trail.slice(0, to + 1);
}

// visible is what the map shows: the filters' choice (filter), narrowed,
// when the drill path is not empty, to the neighbourhood of its last
// station at the given depth. The neighbourhood follows the edges the
// switches leave on, the query aside, so a search inside a separated
// neighbourhood narrows it rather than reaching out of it.
export function visible(graph, { query = "", hidden = new Set(), trail = [], depth = 1 } = {}) {
  const shown = filter(graph, { query, hidden });
  const at = trail[trail.length - 1];
  if (at === undefined) return shown;
  const base = filter(graph, { hidden });
  const near = base.nodes.has(at) ? around(graph, at, depth, base.edges) : new Set();
  const nodes = new Set([...shown.nodes].filter((id) => near.has(id)));
  const edges = shown.edges.filter((i) => nodes.has(graph.edges[i].from) && nodes.has(graph.edges[i].to));
  const matches = new Set([...shown.matches].filter((id) => nodes.has(id)));
  return { nodes, edges, matches };
}

// IMPACT_DIRECTIONS are the ways an impact walk goes (ADR-0159):
// affected, to what depends on the start, and dependson, to what it
// depends on. IMPACT_DEPTHS are the depths one may choose.
export const IMPACT_DIRECTIONS = ["affected", "dependson"];
export const IMPACT_DEPTHS = [1, 2, 3, 4, 5, "all"];

// impactParams reads an impact view from the address's parameters (a
// URLSearchParams): its direction and depth, or null when the address
// asks for none. A depth that is not one of IMPACT_DEPTHS is 1.
export function impactParams(params) {
  const direction = params.get("impact");
  if (!IMPACT_DIRECTIONS.includes(direction)) return null;
  const raw = params.get("depth");
  const depth = raw === "all" ? "all" : IMPACT_DEPTHS.includes(Number(raw)) ? Number(raw) : 1;
  return { direction, depth };
}

// impactAddress is the address of an impact view: the room, the stations
// of the drill path it was opened from, the start last, its direction
// and depth.
export function impactAddress(space, stations, { direction, depth }) {
  return "#/starmap/" + [space, ...stations].map(encodeURIComponent).join("/") + "?impact=" + direction + "&depth=" + depth;
}

// impactPath is the API path of a walk from a station: an item's key or
// ID, or a picture's node id ("picture:<item>/<attachment>").
export function impactPath(station, { direction, depth }) {
  const q = new URLSearchParams({ direction, depth: String(depth) });
  let ref = station;
  const pic = /^picture:([^/]+)\/(.+)$/.exec(station);
  if (pic) { ref = pic[1]; q.set("picture", pic[2]); }
  return `/api/items/${encodeURIComponent(ref)}/impact?${q}`;
}

const edgeKey = (e) => [e.from, e.to, e.kind, e.type || ""].join("\u0000");

// impactGraph turns a walk's answer into a graph the map draws: its
// nodes as they are, each with its distance, and its edges, those the
// walk followed marked walk.
export function impactGraph(result) {
  const walked = new Set();
  for (const n of result.nodes) for (const e of n.via || []) walked.add(edgeKey(e));
  return {
    nodes: result.nodes,
    edges: result.edges.map((e) => ({ ...e, walk: walked.has(edgeKey(e)) })),
  };
}

// distanceClass is the class that grades a node by its distance from the
// start: dist-0 for the start up to dist-5, dist-far beyond.
export function distanceClass(distance) {
  return distance > 5 ? "dist-far" : "dist-" + Math.max(0, distance);
}

// impactList is what the walk reached, the start left out, nearest
// first; nodes of one distance keep the server's order.
export function impactList(result) {
  return result.nodes.filter((n) => n.distance > 0).map((n, i) => [n, i])
    .sort(([a, i], [b, j]) => a.distance - b.distance || i - j).map(([n]) => n);
}

// impactSummary counts what the walk reached, the start left out: its
// items and its pictures, the farthest distance, and whether the walk
// ended before its depth (nothing further reachable within it).
export function impactSummary(result) {
  const reached = impactList(result);
  const farthest = reached.reduce((m, n) => Math.max(m, n.distance), 0);
  return {
    items: reached.filter((n) => n.kind === "item").length,
    pictures: reached.filter((n) => n.kind === "picture").length,
    farthest,
    endedEarly: Boolean(result.complete) && result.depth !== "all" && farthest < result.depth,
  };
}

// zoomBand names how close the reader looks: far, where only the
// captions that matter now show (the node under the pointer or focus, its
// neighbours, the chosen node, the matches), mid, where the items' captions
// join them, and near, where every caption shows.
export function zoomBand(zoom) {
  if (zoom < 0.6) return "far";
  if (zoom < 1.2) return "mid";
  return "near";
}

// captionShown says whether a caption shows at a band; lit is a node that
// matters now (see zoomBand). The style sheet holds the same rule.
export function captionShown(band, { kind = "item", lit = false } = {}) {
  if (lit || band === "near") return true;
  return band === "mid" && kind === "item";
}

// fitView is the zoom and pan that fit the points into a canvas of width
// w and height h with a margin pad on every side, centred; the zoom stays
// within [min, max].
export function fitView(points, w, h, { pad = 48, min = 0.1, max = 2.5 } = {}) {
  if (!points.length) return { zoom: 1, panX: w / 2, panY: h / 2 };
  let x0 = Infinity, y0 = Infinity, x1 = -Infinity, y1 = -Infinity;
  for (const p of points) {
    if (p.x < x0) x0 = p.x;
    if (p.y < y0) y0 = p.y;
    if (p.x > x1) x1 = p.x;
    if (p.y > y1) y1 = p.y;
  }
  const zoom = Math.min(max, Math.max(min, Math.min((w - 2 * pad) / Math.max(x1 - x0, 1), (h - 2 * pad) / Math.max(y1 - y0, 1))));
  return { zoom, panX: w / 2 - zoom * (x0 + x1) / 2, panY: h / 2 - zoom * (y0 + y1) / 2 };
}

// mulberry32 is a small seeded random number generator.
function mulberry32(seed) {
  return () => {
    seed |= 0; seed = (seed + 0x6D2B79F5) | 0;
    let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

// size is the side of the square the layout fills: room for every node.
export function size(n) {
  return Math.round(Math.max(800, Math.sqrt(Math.max(n, 1)) * 110));
}

// layout places the nodes: they repel each other, edges pull their ends
// together, and a weak pull holds everything near the middle. The start is
// a spiral in the map's order with a seeded jitter, and the steps are
// fixed, so the result depends on the map alone. It returns a Map of id to
// {x, y}, rounded to tenths.
export function layout(graph, { iterations = 300, seed = 0x5EED, start = null } = {}) {
  const n = graph.nodes.length;
  const side = size(n);
  const mid = side / 2;
  const random = mulberry32(seed);
  const index = new Map(graph.nodes.map((node, i) => [node.id, i]));
  const x = new Float64Array(n), y = new Float64Array(n);
  for (let i = 0; i < n; i++) {
    const angle = i * 2.399963; // the golden angle
    const r = 18 * Math.sqrt(i + 1);
    x[i] = mid + r * Math.cos(angle) + (random() - 0.5) * 4;
    y[i] = mid + r * Math.sin(angle) + (random() - 0.5) * 4;
    // A given start (the clouds of a room, roomStart) takes the place of
    // the spiral's.
    const at = start?.get(graph.nodes[i].id);
    if (at) { x[i] = at.x; y[i] = at.y; }
  }
  const links = [];
  for (const e of graph.edges) {
    const a = index.get(e.from), b = index.get(e.to);
    if (a !== undefined && b !== undefined && a !== b) links.push([a, b, e.rest]);
  }
  const rest = 90;
  const repulsion = 4200;
  const steps = n > 400 ? Math.min(iterations, 160) : iterations;
  const dx = new Float64Array(n), dy = new Float64Array(n);
  for (let step = 0; step < steps; step++) {
    const heat = 1 - step / steps; // cooling: big moves first, small ones last
    dx.fill(0); dy.fill(0);
    for (let i = 0; i < n; i++) {
      for (let j = i + 1; j < n; j++) {
        let ux = x[i] - x[j], uy = y[i] - y[j];
        let d2 = ux * ux + uy * uy;
        if (d2 < 0.01) { ux = (i % 2 ? 1 : -1) * 0.1; uy = 0.1; d2 = 0.02; }
        if (d2 > 250000) continue; // beyond 500, too far to matter
        const f = repulsion / d2;
        const d = Math.sqrt(d2);
        dx[i] += (ux / d) * f; dy[i] += (uy / d) * f;
        dx[j] -= (ux / d) * f; dy[j] -= (uy / d) * f;
      }
    }
    for (const [a, b, own] of links) {
      const ux = x[b] - x[a], uy = y[b] - y[a];
      const d = Math.sqrt(ux * ux + uy * uy) || 0.1;
      const f = (d - (own ?? rest)) * 0.04;
      dx[a] += (ux / d) * f; dy[a] += (uy / d) * f;
      dx[b] -= (ux / d) * f; dy[b] -= (uy / d) * f;
    }
    const limit = 2 + 28 * heat;
    for (let i = 0; i < n; i++) {
      dx[i] += (mid - x[i]) * 0.012; dy[i] += (mid - y[i]) * 0.012;
      const m = Math.sqrt(dx[i] * dx[i] + dy[i] * dy[i]);
      const k = m > limit ? limit / m : 1;
      x[i] = Math.min(side - 20, Math.max(20, x[i] + dx[i] * k));
      y[i] = Math.min(side - 20, Math.max(20, y[i] + dy[i] * k));
    }
  }
  const out = new Map();
  graph.nodes.forEach((node, i) => out.set(node.id, { x: Math.round(x[i] * 10) / 10, y: Math.round(y[i] * 10) / 10 }));
  return out;
}
