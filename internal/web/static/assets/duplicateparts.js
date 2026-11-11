// Which duplicates the album deletes at once (ADR-0196): of every group the
// server found, one picture stays and the others go, as far as the reader
// may delete them. This module runs without a browser.

const pixels = (p) => (p.attachment.photo?.width || 0) * (p.attachment.photo?.height || 0);
const created = (p) => Date.parse(p.attachment.created) || 0;
const deletable = (p) => p.editable && !p.used;

// rank orders the pictures of a group by which should stay: one a text
// shows or the reader may not delete first, as it stays anyway; then the
// largest picture, by pixels and then by bytes; then the oldest.
export function rank(a, b) {
  return Number(deletable(a)) - Number(deletable(b)) ||
    pixels(b) - pixels(a) ||
    (b.attachment.size || 0) - (a.attachment.size || 0) ||
    created(a) - created(b);
}

// keeper is the picture of a list that stays.
export function keeper(list) {
  return [...list].sort(rank)[0];
}

// cleanupPlan says what deleting the duplicates of groups does. With alike,
// one picture stays of every group; without, one stays of every set of the
// same file within a group, and pictures that only look the same all stay.
// remove holds the pictures to delete; kept counts those that would go but
// stay because the reader may not delete them or a text shows them; alike
// counts the pictures that alike adds to remove.
export function cleanupPlan(groups, alike = false) {
  const remove = [];
  let kept = 0;
  let extra = 0;
  for (const g of groups) {
    const pictures = g.pictures || [];
    if (pictures.length < 2) continue;
    const stays = keeper(pictures);
    const bySum = new Map();
    for (const p of pictures) {
      const sum = p.attachment.sha256 || "";
      if (!bySum.has(sum)) bySum.set(sum, []);
      bySum.get(sum).push(p);
    }
    const sameGo = new Set();
    for (const [sum, list] of bySum) {
      if (!sum || list.length < 2) continue;
      const own = list.includes(stays) ? stays : keeper(list);
      for (const p of list) if (p !== own) sameGo.add(p);
    }
    for (const p of pictures) {
      if (p === stays) continue;
      const goes = alike || sameGo.has(p);
      if (!sameGo.has(p) && deletable(p)) extra++;
      if (!goes) continue;
      if (deletable(p)) remove.push(p);
      else kept++;
    }
  }
  return { remove, kept, alike: extra };
}
