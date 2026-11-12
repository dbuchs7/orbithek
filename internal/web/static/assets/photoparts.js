// The parts of the album's photo management that need no browser
// (ADR-0177): the map's projection, tiles and clusters, the grouping of
// the timeline, choosing a range of pictures, and which files the album
// takes. They are tested with node --test.

// TILE is the edge of a map tile in pixels; MAX_ZOOM the deepest zoom.
export const TILE = 256;
export const MAX_ZOOM = 19;
const MAX_LAT = 85.05112878;

const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));

// project turns a place into world pixels at a zoom (Web Mercator).
export function project(lat, lon, zoom) {
  const size = TILE * 2 ** zoom;
  const s = Math.sin((clamp(lat, -MAX_LAT, MAX_LAT) * Math.PI) / 180);
  return {
    x: ((lon + 180) / 360) * size,
    y: (0.5 - Math.log((1 + s) / (1 - s)) / (4 * Math.PI)) * size,
  };
}

// unproject turns world pixels at a zoom back into a place.
export function unproject(x, y, zoom) {
  const size = TILE * 2 ** zoom;
  const lon = (x / size) * 360 - 180;
  const n = Math.PI - (2 * Math.PI * y) / size;
  const lat = (180 / Math.PI) * Math.atan(0.5 * (Math.exp(n) - Math.exp(-n)));
  return { lat, lon };
}

// fit is the centre and the deepest zoom at which every place shows in a
// view of width × height pixels, with a margin; one place shows at zoom 12.
export function fit(places, width, height, margin = 40) {
  if (!places.length) return { lat: 20, lon: 0, zoom: 1 };
  let [s, n, w, e] = [90, -90, 180, -180];
  for (const p of places) {
    s = Math.min(s, p.lat); n = Math.max(n, p.lat);
    w = Math.min(w, p.lon); e = Math.max(e, p.lon);
  }
  const center = { lat: (s + n) / 2, lon: (w + e) / 2 };
  if (places.length === 1 || (n - s < 1e-6 && e - w < 1e-6)) return { ...center, zoom: 12 };
  for (let z = 16; z > 0; z--) {
    const a = project(n, w, z);
    const b = project(s, e, z);
    if (b.x - a.x <= width - 2 * margin && b.y - a.y <= height - 2 * margin) return { ...center, zoom: z };
  }
  return { ...center, zoom: 1 };
}

// tiles lists the tiles a view needs: its centre, zoom and size, each with
// the place it is drawn at in the view. x wraps around the world; rows
// beyond the poles are left out.
export function tiles(center, zoom, width, height) {
  const c = project(center.lat, center.lon, zoom);
  const left = c.x - width / 2;
  const top = c.y - height / 2;
  const n = 2 ** zoom;
  const out = [];
  for (let ty = Math.floor(top / TILE); ty * TILE < top + height; ty++) {
    if (ty < 0 || ty >= n) continue;
    for (let tx = Math.floor(left / TILE); tx * TILE < left + width; tx++) {
      out.push({ z: zoom, x: ((tx % n) + n) % n, y: ty, left: tx * TILE - left, top: ty * TILE - top });
    }
  }
  return out;
}

// cluster gathers places that lie within cell pixels of each other in a
// view into one point, placed at their mean, with their number; each
// cluster keeps its places.
export function cluster(places, center, zoom, width, height, cell = 48) {
  const c = project(center.lat, center.lon, zoom);
  const left = c.x - width / 2;
  const top = c.y - height / 2;
  const cells = new Map();
  for (const p of places) {
    const w = project(p.lat, p.lon, zoom);
    const x = w.x - left;
    const y = w.y - top;
    if (x < -cell || y < -cell || x > width + cell || y > height + cell) continue;
    const key = `${Math.floor(x / cell)}:${Math.floor(y / cell)}`;
    let k = cells.get(key);
    if (!k) cells.set(key, (k = { x: 0, y: 0, places: [] }));
    k.x += x;
    k.y += y;
    k.places.push(p);
  }
  return [...cells.values()].map((k) => ({ x: k.x / k.places.length, y: k.y / k.places.length, count: k.places.length, places: k.places }));
}

// radius is how large a point of the map is drawn: a single place small,
// a cluster growing with its number, up to a bound.
export const radius = (count) => (count === 1 ? 7 : Math.min(26, 10 + Math.log2(count) * 3));

// hit is the point of the map a click at (x, y) lands on: within its
// drawn radius and a little more for a finger, the nearest if several;
// null when none.
export function hit(points, x, y, slack = 4) {
  let best = null;
  let bestD = Infinity;
  for (const p of points) {
    const d = Math.hypot(p.x - x, p.y - y);
    if (d <= radius(p.count) + slack && d < bestD) {
      best = p;
      bestD = d;
    }
  }
  return best;
}

// pan moves a centre by pixels at a zoom.
export function pan(center, zoom, dx, dy) {
  const c = project(center.lat, center.lon, zoom);
  const size = TILE * 2 ** zoom;
  const p = unproject(((c.x - dx) % size + size) % size, clamp(c.y - dy, 0, size), zoom);
  return { lat: p.lat, lon: p.lon };
}

// zoomAt changes the zoom keeping the place under a point of the view
// (px, py) where it is.
export function zoomAt(center, zoom, next, px, py, width, height) {
  next = clamp(next, 1, MAX_ZOOM);
  const c = project(center.lat, center.lon, zoom);
  const under = unproject(c.x - width / 2 + px, c.y - height / 2 + py, zoom);
  const u = project(under.lat, under.lon, next);
  const p = unproject(u.x - px + width / 2, u.y - py + height / 2, next);
  return { lat: p.lat, lon: p.lon, zoom: next };
}

// groups splits pictures, in the order given, into months and days by
// the day each was taken (taken.day, "YYYY-MM-DD"); pictures without it
// are grouped by the day they were added.
export function groups(pictures) {
  const out = [];
  for (const p of pictures) {
    const day = p.taken?.day || String(p.attachment?.created || "").slice(0, 10);
    const month = day.slice(0, 7);
    let m = out[out.length - 1];
    if (!m || m.month !== month) out.push((m = { month, days: [] }));
    let d = m.days[m.days.length - 1];
    if (!d || d.day !== day) m.days.push((d = { day, pictures: [] }));
    d.pictures.push(p);
  }
  return out;
}

// yearsAgo is how many years before today a day lies, on the same
// calendar day; 0 for this year.
export function yearsAgo(day, today) {
  return Number(today.slice(0, 4)) - Number(day.slice(0, 4));
}

// range chooses the pictures between anchor and target in the order shown,
// both included; the IDs of the others stay as they are.
export function range(ids, anchor, target) {
  const a = ids.indexOf(anchor);
  const b = ids.indexOf(target);
  if (a < 0 || b < 0) return [target];
  return ids.slice(Math.min(a, b), Math.max(a, b) + 1);
}

// dragged is what a picture dragged onto a folder takes with it: every
// chosen picture when it is one of them, else itself alone. chosen maps
// keys to pictures; key names a picture.
export function dragged(p, chosen, key) {
  return chosen.has(key(p)) ? [...chosen.values()] : [p];
}

// folderTargets lists where a folder of the album may be moved (ADR-0200):
// the top of every room and every folder one may fill, each room's after
// its top, but neither the folder itself, a folder below it nor where it
// is now. moving is the folder, with id, space and parent; folders are as
// allFolders lists them, with id, space, parent, path and editable; rooms
// as the album lists them, with key and name; top names a room's top.
// Folders of a room not listed follow at the end. Each target names its
// room, its parent ("" for the top) and its text. The server decides
// whether the move is allowed.
export function folderTargets(moving, folders, rooms, top) {
  const parentOf = new Map(folders.map((f) => [f.id, f.parent || ""]));
  const below = (id) => {
    for (let at = id, n = 0; at && n < 64; at = parentOf.get(at), n++) {
      if (at === moving.id) return true;
    }
    return false;
  };
  const into = (f) => f.editable !== false && f.id !== (moving.parent || "") && !below(f.id);
  const out = [];
  const keys = new Set(rooms.map((r) => r.key));
  for (const r of rooms) {
    if (r.key !== moving.space || moving.parent) out.push({ space: r.key, parent: "", text: `${r.key} ${r.name} › ${top}` });
    for (const f of folders) if (f.space === r.key && into(f)) out.push({ space: f.space, parent: f.id, text: f.path });
  }
  for (const f of folders) if (!keys.has(f.space) && into(f)) out.push({ space: f.space, parent: f.id, text: f.path });
  return out;
}

// folderDrop says where a folder dropped onto another goes (ADR-0200):
// into the folder it is dropped onto, or, dropped onto the album's top
// (onto null), to the top of its own room; null when that is where it is
// or the folder itself.
export function folderDrop(moving, onto) {
  const to = onto ? { space: onto.space, parent: onto.id } : { space: moving.space, parent: "" };
  if (to.parent === moving.id || (to.space === moving.space && to.parent === (moving.parent || ""))) return null;
  return to;
}

// orderDrop says where pictures dropped onto a picture of a list go in
// the order of slides set by hand (ADR-0195). firsts are the first
// versions of the pictures listed, in their order; moved those of the
// pictures dropped; target the index of the picture dropped onto and
// after whether onto its latter half; complete whether the list holds
// every picture, so that after its last one is the end. It answers the
// first version to place them before, "" for the end, or null when the
// place is not known: after the last picture shown while more follow.
export function orderDrop(firsts, moved, target, after, complete) {
  const skip = new Set(moved);
  for (let i = after ? target + 1 : target; i < firsts.length; i++) {
    if (!skip.has(firsts[i])) return firsts[i];
  }
  return complete ? "" : null;
}

// moveWithin moves items of a list as the server moves pictures in the
// order of slides (ADR-0195): those moved, in the order given, right
// before the item before names, or to the end when before is "". keyOf
// names an item.
export function moveWithin(list, keyOf, moved, before) {
  const skip = new Set(moved);
  const rest = list.filter((x) => !skip.has(keyOf(x)));
  const taken = moved.map((k) => list.find((x) => keyOf(x) === k)).filter(Boolean);
  const at = before ? rest.findIndex((x) => keyOf(x) === before) : -1;
  const i = at < 0 ? rest.length : at;
  return [...rest.slice(0, i), ...taken, ...rest.slice(i)];
}

// ALBUM_TYPES are the formats the album takes (JPEG, PNG, GIF, WebP), and
// VIDEO_TYPES its videos (MP4, WebM, QuickTime; ADR-0189).
export const ALBUM_TYPES = ["image/jpeg", "image/png", "image/gif", "image/webp"];
export const VIDEO_TYPES = ["video/mp4", "video/webm", "video/quicktime"];
const EXT = { jpg: "image/jpeg", jpeg: "image/jpeg", png: "image/png", gif: "image/gif", webp: "image/webp",
  mp4: "video/mp4", m4v: "video/mp4", webm: "video/webm", mov: "video/quicktime", qt: "video/quicktime" };

// sorts says what the album does with a file: "picture" it takes, "video"
// it takes as a video, "heic" an iPhone photo it does not take, "skip"
// anything else (a sidecar file, a folder).
export function sorts(name, type = "") {
  const ext = (name.match(/\.([^./\\]+)$/)?.[1] || "").toLowerCase();
  if (/^(heic|heif)$/.test(ext) || /^image\/hei[cf]/.test(type)) return "heic";
  if (VIDEO_TYPES.includes(type) || VIDEO_TYPES.includes(EXT[ext])) return "video";
  if (ALBUM_TYPES.includes(type) || EXT[ext]) return "picture";
  return "skip";
}

// isVideo reports whether a picture of the album is a video.
export const isVideo = (p) => Boolean(p?.video) || VIDEO_TYPES.includes(p?.attachment?.type);

// onShare says a picture is an entry of a file share (ADR-0197): the share
// is the master, so it is not renamed, moved or deleted here.
export const onShare = (p) => Boolean(p?.attachment?.share);

// shareLink is the address that opens an entry's original where it lies,
// "" when its share names none: an https: or smb: address of the share's
// link template, or the wiki's own route that signs a link to an S3
// share's original when it is followed.
export const shareLink = (p) => {
  const link = p?.attachment?.share?.link || "";
  return /^(https|smb):\/\//i.test(link) || /^\/api\/items\/[^/]+\/attachments\/[0-9a-f]{32}\/original$/.test(link) ? link : "";
};

// durationText writes how long a video plays: "0:12", "4:05", "1:02:03".
export function durationText(seconds) {
  if (!(seconds > 0)) return "";
  const s = Math.round(seconds);
  const pad = (n) => String(n).padStart(2, "0");
  const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), r = s % 60;
  return h ? `${h}:${pad(m)}:${pad(r)}` : `${m}:${pad(r)}`;
}

// posterTime is where in a video its poster is taken: a second in, or a
// tenth of a shorter video, so it is not the black first frame.
export const posterTime = (duration) => (duration > 0 && Number.isFinite(duration) ? Math.min(1, duration / 10) : 0);

// typeOf is the format of a file by its name, for files unpacked from an
// archive, which carry no type.
export const typeOf = (name) => EXT[(name.match(/\.([^.]+)$/)?.[1] || "").toLowerCase()] || "";

// baseName is the last part of a path inside an archive.
export const baseName = (path) => path.split(/[\\/]/).pop();

// hidden reports whether an archive entry is a folder or a file the
// system that packed it added (macOS resource forks, .DS_Store).
export const hidden = (path) => path.endsWith("/") || /(^|\/)(__MACOSX|\.[^/]*)(\/|$)/.test(path);

// exposureText writes a photo's exposure: "1/250 s · f/8 · ISO 100 · 35 mm".
export function exposureText(p) {
  if (!p) return "";
  return [p.exposure && `${p.exposure} s`, p.fNumber && `f/${p.fNumber}`, p.iso && `ISO ${p.iso}`,
    p.focal && `${p.focal} mm${p.focal35 && p.focal35 !== Math.round(p.focal) ? ` (${p.focal35} mm)` : ""}`].filter(Boolean).join(" · ");
}

// placeText writes a place in decimal degrees, south and west negative:
// "46.55840, 7.83520".
export function placeText(p) {
  if (!p || p.lat == null || p.lon == null) return "";
  return `${p.lat.toFixed(5)}, ${p.lon.toFixed(5)}`;
}

// OFFSETS are the offsets from UTC a time set by hand may name
// (ADR-0198): every quarter of an hour from -12:00 to +14:00.
export const OFFSETS = Array.from({ length: (26 * 60) / 15 + 1 }, (_, i) => {
  const m = -12 * 60 + i * 15;
  const a = Math.abs(m);
  return `${m < 0 ? "-" : "+"}${String(Math.floor(a / 60)).padStart(2, "0")}:${String(a % 60).padStart(2, "0")}`;
});

// The years a time set by hand may lie in, as the server takes them.
const MIN_YEAR = 1800;
const MAX_YEAR = 2200;

// wallTime writes the parts of a wall-clock time as the server takes it,
// "2024-05-01T13:22:10", or "" when they are no time of those years.
function wallTime(y, mo, d, hr, mi, s) {
  const t = new Date(Date.UTC(2000, mo - 1, d, hr, mi, s));
  t.setUTCFullYear(y, mo - 1, d);
  if (y < MIN_YEAR || y > MAX_YEAR || t.getUTCMonth() !== mo - 1 || t.getUTCDate() !== d || hr > 23 || mi > 59 || s > 59) return "";
  const two = (n) => String(n).padStart(2, "0");
  return `${String(y).padStart(4, "0")}-${two(mo)}-${two(d)}T${two(hr)}:${two(mi)}:${two(s)}`;
}

// takenValue turns the value of a field for a date and time,
// "2024-05-01T13:22" or with seconds, into the time a photo was taken as
// it is set by hand, "2024-05-01T13:22:00"; "" when it is none (ADR-0198).
export function takenValue(value) {
  const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2})(?:\.\d+)?)?$/.exec(value || "");
  return m ? wallTime(+m[1], +m[2], +m[3], +m[4], +m[5], +(m[6] || 0)) : "";
}

// shiftMinutes is how far a shift moves a time, in minutes: days, hours
// and minutes, later or earlier.
export const shiftMinutes = ({ days = 0, hours = 0, minutes = 0, later = true }) =>
  (later ? 1 : -1) * (Math.trunc(+days || 0) * 1440 + Math.trunc(+hours || 0) * 60 + Math.trunc(+minutes || 0));

// shiftTaken moves the wall-clock time a photo was taken by minutes, as a
// camera whose clock was wrong is put right; "" when the time is none or
// leaves the years a time may lie in (ADR-0198).
export function shiftTaken(taken, minutes) {
  const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})$/.exec(taken || "");
  if (!m || !Number.isFinite(minutes)) return "";
  const t = new Date(Date.UTC(2000, +m[2] - 1, +m[3], +m[4], +m[5], +m[6]));
  t.setUTCFullYear(+m[1], +m[2] - 1, +m[3]);
  t.setTime(t.getTime() + minutes * 60000);
  return wallTime(t.getUTCFullYear(), t.getUTCMonth() + 1, t.getUTCDate(), t.getUTCHours(), t.getUTCMinutes(), t.getUTCSeconds());
}
