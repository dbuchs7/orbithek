// Unit tests of the album's photo management that need no browser
// (photoparts.js, zipread.js, ADR-0177): the map's projection, tiles,
// clusters and zoom, the timeline's groups, choosing a range, what a
// picture dragged onto a folder takes with it, where a folder may be moved
// (ADR-0200), where pictures dropped
// among others go in the order of slides (ADR-0195), which files the album takes, the times set by hand (ADR-0198), and reading ZIP archives exported from photo apps.
// They run in Node.js without a browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// The album's views and imports themselves are in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import { deflateRawSync } from "node:zlib";
import {
  TILE, project, unproject, fit, tiles, cluster, pan, zoomAt, radius, hit, groups, yearsAgo, range, dragged, folderTargets, folderDrop, orderDrop, moveWithin, durationText, posterTime, isVideo, onShare, shareLink,
  sorts, typeOf, baseName, hidden, exposureText, placeText, OFFSETS, takenValue, shiftMinutes, shiftTaken,
} from "../static/assets/photoparts.js";
import { entries, read } from "../static/assets/zipread.js";

const near = (a, b, eps = 1e-6) => assert.ok(Math.abs(a - b) < eps, `${a} ≉ ${b}`);

test("a place projects onto the world and back", () => {
  assert.deepEqual(project(0, 0, 0), { x: TILE / 2, y: TILE / 2 });
  const p = project(46.5584, 7.8352, 12);
  const back = unproject(p.x, p.y, 12);
  near(back.lat, 46.5584);
  near(back.lon, 7.8352);
  assert.ok(project(89.9, 0, 1).y > -1e-6 && project(-89.9, 0, 1).y < 2 * TILE + 1e-6, "the poles are clamped");
});

test("the view fits the places, one place at a street's zoom", () => {
  assert.deepEqual(fit([], 800, 600), { lat: 20, lon: 0, zoom: 1 });
  assert.equal(fit([{ lat: 46, lon: 7 }], 800, 600).zoom, 12);
  const f = fit([{ lat: 46, lon: 6 }, { lat: 47.8, lon: 10.5 }], 800, 600);
  assert.ok(f.zoom >= 6 && f.zoom <= 8, String(f.zoom));
  near(f.lat, 46.9);
  assert.equal(fit([{ lat: -80, lon: -179 }, { lat: 80, lon: 179 }], 300, 200).zoom, 1);
});

test("the tiles of a view cover it, wrapping around the world", () => {
  const ts = tiles({ lat: 0, lon: 0 }, 1, 512, 512);
  assert.equal(ts.length, 4);
  assert.deepEqual(ts.map((t) => `${t.x},${t.y}`).sort(), ["0,0", "0,1", "1,0", "1,1"]);
  const wide = tiles({ lat: 0, lon: 179 }, 1, 1024, 256);
  assert.ok(wide.every((t) => t.x >= 0 && t.x < 2));
  assert.ok(tiles({ lat: 85, lon: 0 }, 2, 256, 1024).every((t) => t.y >= 0 && t.y < 4), "no rows beyond the poles");
});

test("near places cluster, far ones stay apart, places out of view are left out", () => {
  const places = [{ lat: 46.5, lon: 7.8 }, { lat: 46.5001, lon: 7.8001 }, { lat: -33.8, lon: 151.2 }];
  const at3 = cluster(places, { lat: 0, lon: 0 }, 1, 800, 600);
  assert.equal(at3.reduce((n, c) => n + c.count, 0), 3);
  assert.equal(at3.find((c) => c.count === 2).places.length, 2);
  assert.equal(cluster(places, { lat: 46.5, lon: 7.8 }, 12, 400, 300).length, 1, "Sydney is out of view");
});

test("a click lands on the point drawn there, the nearest of several", () => {
  assert.equal(radius(1), 7);
  assert.ok(radius(8) > radius(2) && radius(100000) === 26);
  const one = { x: 100, y: 100, count: 1, places: [{}] };
  const many = { x: 140, y: 100, count: 32, places: [{}, {}] };
  assert.equal(hit([one, many], 104, 103), one);
  assert.equal(hit([one, many], 160, 100), many, "within the larger point's own radius");
  const near = { x: 120, y: 100, count: 32, places: [{}] };
  assert.equal(hit([near, one], 108, 100), one, "where both overlap, the nearer");
  assert.equal(hit([one, many], 100, 140), null);
  assert.equal(hit([], 0, 0), null);
});

test("panning and zooming keep the map where it is pointed at", () => {
  const c = { lat: 10, lon: 20 };
  const moved = pan(c, 5, 100, 0);
  assert.ok(moved.lon < 20);
  const back = pan(moved, 5, -100, 0);
  near(back.lon, 20, 1e-6);
  const z = zoomAt(c, 5, 6, 400, 300, 800, 600);
  near(z.lat, 10, 1e-6);
  near(z.lon, 20, 1e-6);
  assert.equal(zoomAt(c, 5, 99, 0, 0, 800, 600).zoom, 19);
  assert.equal(zoomAt(c, 5, -3, 0, 0, 800, 600).zoom, 1);
});

test("the timeline groups by month and day in the order given", () => {
  const p = (day) => ({ taken: { day }, attachment: {} });
  const g = groups([p("2024-05-02"), p("2024-05-02"), p("2024-05-01"), p("2023-12-24"), { attachment: { created: "2022-01-01T10:00:00Z" } }]);
  assert.deepEqual(g.map((m) => m.month), ["2024-05", "2023-12", "2022-01"]);
  assert.deepEqual(g[0].days.map((d) => [d.day, d.pictures.length]), [["2024-05-02", 2], ["2024-05-01", 1]]);
  assert.equal(yearsAgo("2023-11-10", "2026-11-10"), 3);
});

// A room A with folders Trips › 2024 › Rome and Prints, a room B with
// Archive, the way allFolders lists them; Prints may not be filled.
const roomsAB = [{ key: "A", name: "Alpha" }, { key: "B", name: "Beta" }];
const foldersAB = [
  { id: "t", space: "A", parent: "", path: "A › Trips" },
  { id: "y", space: "A", parent: "t", path: "A › Trips › 2024" },
  { id: "r", space: "A", parent: "y", path: "A › Trips › 2024 › Rome" },
  { id: "p", space: "A", parent: "", path: "A › Prints", editable: false },
  { id: "x", space: "B", parent: "", path: "B › Archive" },
];

test("a folder is offered every room's top and every folder it may go into, by room", () => {
  const out = folderTargets({ id: "y", space: "A", parent: "t" }, foldersAB, roomsAB, "top");
  assert.deepEqual(out.map((o) => [o.space, o.parent, o.text]), [
    ["A", "", "A Alpha › top"],
    ["B", "", "B Beta › top"],
    ["B", "x", "B › Archive"],
  ]);
});

test("a folder is not offered itself, a folder below it, where it is or a folder one may not fill", () => {
  const out = folderTargets({ id: "t", space: "A", parent: "" }, foldersAB, roomsAB, "top");
  assert.deepEqual(out.map((o) => o.parent || o.space), ["B", "x"]);
  const rome = folderTargets({ id: "r", space: "A", parent: "y" }, foldersAB, roomsAB, "top");
  assert.deepEqual(rome.map((o) => o.parent || o.space), ["A", "t", "B", "x"]);
});

test("a folder of a room the album does not list is still offered", () => {
  const out = folderTargets({ id: "x", space: "B", parent: "" }, [...foldersAB, { id: "c", space: "C", parent: "", path: "C › Club" }], roomsAB, "top");
  assert.deepEqual(out.map((o) => o.parent || o.space), ["A", "t", "y", "r", "c"]);
});

test("a folder dropped onto a folder goes into it, onto the top to its room's top, and nowhere it is", () => {
  const f = { id: "y", space: "A", parent: "t" };
  assert.deepEqual(folderDrop(f, { id: "x", space: "B" }), { space: "B", parent: "x" });
  assert.deepEqual(folderDrop(f, null), { space: "A", parent: "" });
  assert.equal(folderDrop(f, { id: "y", space: "A" }), null);
  assert.equal(folderDrop(f, { id: "t", space: "A" }), null);
  assert.equal(folderDrop({ id: "t", space: "A", parent: "" }, null), null);
});

test("a dragged picture takes the chosen ones with it only when it is chosen", () => {
  const key = (p) => p.id;
  const a = { id: "a" }, b = { id: "b" }, c = { id: "c" };
  const chosen = new Map([["a", a], ["b", b]]);
  assert.deepEqual(dragged(a, chosen, key), [a, b]);
  assert.deepEqual(dragged(c, chosen, key), [c]);
  assert.deepEqual(dragged(c, new Map(), key), [c]);
});

test("pictures dropped onto another go before it, or after it onto its latter half", () => {
  const firsts = ["a", "b", "c", "d"];
  assert.equal(orderDrop(firsts, ["d"], 1, false, true), "b");
  assert.equal(orderDrop(firsts, ["d"], 1, true, true), "c");
  assert.equal(orderDrop(firsts, ["a"], 3, true, true), "", "after the last one is the end");
  assert.equal(orderDrop(firsts, ["a"], 3, true, false), null, "after the last one shown while more follow is not known");
  assert.equal(orderDrop(firsts, ["b", "c"], 0, true, true), "d", "the moved ones are passed over");
  assert.equal(orderDrop(firsts, ["c"], 2, false, true), "d", "onto itself: before the next one");
});

test("pictures move within a list as the server moves them", () => {
  const id = (x) => x;
  assert.deepEqual(moveWithin(["a", "b", "c", "d"], id, ["d"], "b"), ["a", "d", "b", "c"]);
  assert.deepEqual(moveWithin(["a", "b", "c", "d"], id, ["c", "a"], ""), ["b", "d", "c", "a"]);
  assert.deepEqual(moveWithin(["a", "b"], id, ["x"], "a"), ["a", "b"], "one not in the list is left out");
});

test("a range is chosen in the order shown, either way", () => {
  const ids = ["a", "b", "c", "d"];
  assert.deepEqual(range(ids, "b", "d"), ["b", "c", "d"]);
  assert.deepEqual(range(ids, "d", "b"), ["b", "c", "d"]);
  assert.deepEqual(range(ids, "x", "c"), ["c"]);
});

test("the album takes JPEG, PNG, GIF, WebP and videos, and tells HEIC apart", () => {
  assert.equal(sorts("IMG_1.JPG"), "picture");
  assert.equal(sorts("x", "image/webp"), "picture");
  assert.equal(sorts("IMG_2.HEIC"), "heic");
  assert.equal(sorts("x", "image/heif"), "heic");
  assert.equal(sorts("clip.mov", "video/quicktime"), "video");
  assert.equal(sorts("IMG_0001.MOV"), "video");
  assert.equal(sorts("clip.webm"), "video");
  assert.equal(sorts("x", "video/mp4"), "video");
  assert.equal(sorts("song.mp3", "audio/mpeg"), "skip");
  assert.equal(typeOf("a/clip.m4v"), "video/mp4");
  assert.equal(sorts("photo.json"), "skip");
  assert.equal(typeOf("a/b/c.jpeg"), "image/jpeg");
  assert.equal(typeOf("c.tif"), "");
  assert.equal(baseName("Export/2024/IMG_1.jpg"), "IMG_1.jpg");
  assert.ok(hidden("__MACOSX/._IMG_1.jpg") && hidden("Export/.DS_Store") && hidden("Export/"));
  assert.ok(!hidden("Export/IMG_1.jpg"));
});

test("a video's length is written as a player writes it, and its poster is taken past the first frame", () => {
  assert.equal(durationText(12.3), "0:12");
  assert.equal(durationText(245), "4:05");
  assert.equal(durationText(3723), "1:02:03");
  assert.equal(durationText(0), "");
  assert.equal(durationText(undefined), "");
  assert.equal(posterTime(60), 1);
  assert.equal(posterTime(5), 0.5);
  assert.equal(posterTime(NaN), 0);
  assert.equal(posterTime(Infinity), 0);
  assert.ok(isVideo({ video: true }) && isVideo({ attachment: { type: "video/quicktime" } }) && !isVideo({ attachment: { type: "image/png" } }));
});

test("exposure and place are written plainly", () => {
  assert.equal(exposureText({ exposure: "1/250", fNumber: 8, iso: 100, focal: 35, focal35: 52 }), "1/250 s · f/8 · ISO 100 · 35 mm (52 mm)");
  assert.equal(exposureText({ focal: 50, focal35: 50 }), "50 mm");
  assert.equal(exposureText(null), "");
  assert.equal(placeText({ lat: 46.5584, lon: -7.8352 }), "46.55840, -7.83520");
  assert.equal(placeText({}), "");
});

// zipOf packs files into a ZIP archive: stored, or deflated.
function zipOf(files) {
  const enc = new TextEncoder();
  const parts = [];
  const central = [];
  let offset = 0;
  for (const f of files) {
    const name = enc.encode(f.name);
    const data = f.deflate ? deflateRawSync(f.data) : f.data;
    const local = Buffer.alloc(30);
    local.writeUInt32LE(0x04034b50, 0);
    local.writeUInt16LE(0x800, 6);
    local.writeUInt16LE(f.deflate ? 8 : 0, 8);
    local.writeUInt32LE(data.length, 18);
    local.writeUInt32LE(f.data.length, 22);
    local.writeUInt16LE(name.length, 26);
    const head = Buffer.alloc(46);
    head.writeUInt32LE(0x02014b50, 0);
    head.writeUInt16LE(0x800, 8);
    head.writeUInt16LE(f.deflate ? 8 : 0, 10);
    head.writeUInt32LE(data.length, 20);
    head.writeUInt32LE(f.data.length, 24);
    head.writeUInt16LE(name.length, 28);
    head.writeUInt32LE(offset, 42);
    parts.push(local, name, data);
    central.push(head, name);
    offset += 30 + name.length + data.length;
  }
  const dir = Buffer.concat(central);
  const end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50, 0);
  end.writeUInt16LE(files.length, 8);
  end.writeUInt16LE(files.length, 10);
  end.writeUInt32LE(dir.length, 12);
  end.writeUInt32LE(offset, 16);
  return new Blob([Buffer.concat([...parts, dir, end])]);
}

test("a ZIP archive's pictures are read, stored and deflated", async () => {
  const jpeg = Buffer.from([0xff, 0xd8, 1, 2, 3, 0xff, 0xd9]);
  const text = Buffer.from("x".repeat(500));
  const zip = zipOf([{ name: "Export/IMG_1.jpg", data: jpeg }, { name: "Export/Fotos – Sommer/notes.txt", data: text, deflate: true }]);
  const list = await entries(zip);
  assert.deepEqual(list.map((e) => [e.name, e.method, e.size]), [["Export/IMG_1.jpg", 0, 7], ["Export/Fotos – Sommer/notes.txt", 8, 500]]);
  const a = await read(zip, list[0], "image/jpeg");
  assert.equal(a.type, "image/jpeg");
  assert.deepEqual(Buffer.from(await a.arrayBuffer()), jpeg);
  const b = await read(zip, list[1]);
  assert.equal(Buffer.from(await b.arrayBuffer()).toString(), text.toString());
});

test("what is no ZIP archive, or a damaged one, is refused", async () => {
  await assert.rejects(entries(new Blob([Buffer.from("not a zip archive at all, just text")])), /not a ZIP/);
  const zip = zipOf([{ name: "a.jpg", data: Buffer.from([1, 2, 3]) }]);
  const list = await entries(zip);
  await assert.rejects(read(zip, { ...list[0], encrypted: true }), /encrypted/);
  await assert.rejects(read(zip, { ...list[0], method: 12 }), /compression/);
  await assert.rejects(read(zip, { ...list[0], local: 5 }), /damaged/);
});

test("the offsets a time set by hand may name run every quarter hour from -12:00 to +14:00", () => {
  assert.equal(OFFSETS.length, 105);
  assert.deepEqual([OFFSETS[0], OFFSETS[1], OFFSETS[48], OFFSETS[57], OFFSETS.at(-1)], ["-12:00", "-11:45", "+00:00", "+02:15", "+14:00"]);
  assert.ok(OFFSETS.every((o) => /^[+-](0\d|1[0-4]):[0-5]\d$/.test(o)), "every offset is one the server takes");
});

test("a field's date and time becomes the time set by hand, or nothing", () => {
  assert.equal(takenValue("2024-05-01T13:22"), "2024-05-01T13:22:00");
  assert.equal(takenValue("2024-05-01T13:22:10"), "2024-05-01T13:22:10");
  assert.equal(takenValue("2024-05-01T13:22:10.5"), "2024-05-01T13:22:10");
  assert.equal(takenValue("1800-01-01T00:00"), "1800-01-01T00:00:00");
  assert.equal(takenValue("2024-02-29T12:00"), "2024-02-29T12:00:00");
  for (const bad of ["", "2024-05-01", "2023-02-29T12:00", "2024-13-01T00:00", "2024-05-01T24:00", "1799-12-31T23:59", "2201-01-01T00:00", "x"]) {
    assert.equal(takenValue(bad), "", bad);
  }
});

test("a time is shifted by days, hours and minutes, later or earlier, within the years a time may lie in", () => {
  assert.equal(shiftMinutes({ days: 1, hours: 2, minutes: 3 }), 1563);
  assert.equal(shiftMinutes({ days: "0", hours: "1", minutes: "", later: false }), -60);
  assert.equal(shiftMinutes({}), 0);
  assert.equal(shiftTaken("2024-12-31T23:30:00", 60), "2025-01-01T00:30:00");
  assert.equal(shiftTaken("2024-03-01T00:10:05", -20), "2024-02-29T23:50:05");
  assert.equal(shiftTaken("2024-05-01T13:22:10", 365 * 1440), "2025-05-01T13:22:10");
  assert.equal(shiftTaken("1800-01-01T00:00:00", -1), "");
  assert.equal(shiftTaken("2200-12-31T23:59:00", 1), "");
  assert.equal(shiftTaken("", 10), "");
  assert.equal(shiftTaken("2024-05-01T13:22:10", NaN), "");
});

test("an entry of a file share is told apart and opens only by https: or smb:", () => {
  const entry = (link) => ({ attachment: { share: { share: "nas", path: "a.jpg", link } } });
  assert.equal(onShare(entry("")), true);
  assert.equal(onShare({ attachment: {} }), false);
  assert.equal(onShare(null), false);
  assert.equal(shareLink(entry("smb://nas/photos/a.jpg")), "smb://nas/photos/a.jpg");
  assert.equal(shareLink(entry("https://nas.example/p?a.jpg")), "https://nas.example/p?a.jpg");
  assert.equal(shareLink(entry("javascript:alert(1)")), "");
  assert.equal(shareLink(entry("file:///mnt/a.jpg")), "");
  assert.equal(shareLink(entry(undefined)), "");
  assert.equal(shareLink({ attachment: {} }), "");
});

test("the link to an original takes https: and smb: alone", async () => {
  const { shareHref } = await import("../static/assets/dom.js");
  assert.equal(shareHref("smb://nas/photos/2024/lake.png"), "smb://nas/photos/2024/lake.png");
  assert.equal(shareHref("https://nas.example/p?f=a.jpg"), "https://nas.example/p?f=a.jpg");
  for (const bad of ["javascript:alert(1)", "data:text/html,x", "file:///etc/passwd", "http://nas/a.jpg", "/relative", "not a url"]) {
    assert.equal(shareHref(bad), "#", bad);
  }
});

test("an S3 share that signs links opens its original through the wiki", async () => {
  const { shareHref } = await import("../static/assets/dom.js");
  const route = "/api/items/abc123/attachments/0123456789abcdef0123456789abcdef/original";
  assert.equal(shareLink({ attachment: { share: { link: route } } }), route);
  assert.equal(shareHref(route), route);
  for (const bad of ["/api/items/abc/attachments/x/original", "/api/items/abc123/attachments/0123456789abcdef0123456789abcdef", "//evil.example/api/items/a/attachments/0123456789abcdef0123456789abcdef/original"]) {
    assert.equal(shareLink({ attachment: { share: { link: bad } } }), "", bad);
    assert.equal(shareHref(bad), "#", bad);
  }
});
