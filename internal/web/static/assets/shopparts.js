// The parts of the shop (ADR-draft-a-shop-and-catalogues-from-an-engine)
// that work without a browser: which text of a catalogue or product to
// show, which products a release offers on their own, how a basket becomes
// the engine's orders, and how answers of a form become the strings an
// order carries.

// textOf is the text of a map by language in the language wanted, else in
// the first language the map holds, else empty.
export function textOf(texts, lang) {
  if (!texts) return "";
  if (texts[lang]) return texts[lang];
  const first = Object.keys(texts).sort().find((k) => String(texts[k] || "").trim());
  return first ? texts[first] : "";
}

// offered are the products of a release a person picks on their own: an
// active product that is no integral part of another — a part comes with
// its whole.
export function offered(release) {
  if (!release?.items) return [];
  const parts = new Set(Object.values(release.includes || {}).flat());
  return release.items.filter((it) => (!it.state || it.state === "active") && !parts.has(it.id));
}

// partsOf are the integral parts and the options of a product in a
// release, as products.
export function partsOf(release, id) {
  const byID = new Map((release?.items || []).map((it) => [it.id, it]));
  const pick = (ids) => (ids || []).map((x) => byID.get(x)).filter(Boolean);
  return { included: pick(release?.includes?.[id]), options: pick(release?.options?.[id]) };
}

// answers turns what a form-js form submitted into the strings an order
// carries: a string as it is, a list and an object as JSON, nothing for
// an empty answer.
export function answers(data) {
  const out = {};
  for (const [k, v] of Object.entries(data || {})) {
    if (v === null || v === undefined || v === "") continue;
    out[k] = typeof v === "string" ? v : typeof v === "number" || typeof v === "boolean" ? String(v) : JSON.stringify(v);
  }
  return out;
}

// orders are the engine's orders of a basket: one per release, with the
// products chosen, their options, the shape chosen of each and the
// answers of their forms, in the order they were put in the basket.
export function orders(basket) {
  const byRelease = new Map();
  for (const line of basket || []) {
    if (!line?.releaseId || !line.itemId) continue;
    if (!byRelease.has(line.releaseId)) byRelease.set(line.releaseId, { releaseId: line.releaseId, items: [] });
    const o = byRelease.get(line.releaseId);
    for (const id of [line.itemId, ...(line.options || [])]) if (!o.items.includes(id)) o.items.push(id);
    if (line.variantId) {
      o.variants = o.variants || {};
      (o.variants[line.itemId] = o.variants[line.itemId] || []).push(line.variantId);
    }
    if (line.config && Object.keys(line.config).length) {
      o.config = o.config || {};
      o.config[line.itemId] = line.config;
    }
  }
  return [...byRelease.values()];
}

// The statuses of an order's position the shop names; another is shown as
// the engine names it.
export const statuses = ["pending", "blocked", "running", "done", "skipped", "failed", "rejected", "cancelled", "returning", "returned", "abandoned"];

// settled reports whether a position is over: nothing will happen to it
// any more unless it is given back.
export const settled = (status) => ["done", "skipped", "rejected", "cancelled", "returned", "abandoned"].includes(status);

// listOf reads a list the engine answered either as it is or as a page.
export function listOf(answer) {
  if (Array.isArray(answer)) return answer;
  if (answer && Array.isArray(answer.items)) return answer.items;
  return [];
}
