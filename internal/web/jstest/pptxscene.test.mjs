// Unit tests of drawing a presentation's slides without a browser
// (pptxscene.js, pptxdraw.js, ADR-0186): colours through theme and colour
// map, fills and lines, transforms, text inherited through the styles of
// master, layout and shape, placeholders, preset and freeform shapes,
// tables, the scene of a whole slide, and the text layout with its
// wrapping, alignment, bullets, anchoring and autofit. They run in Node.js
// without a browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// Drawing onto a canvas is in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import { deflateRawSync } from "node:zlib";
import { parseXML, readDeck } from "../static/assets/pptx.js";
import {
  colorOf, css, readTheme, fillOf, lineOf, xfrmOf, boxMatrix, apply, textOf, autoNumber, geometry, findPlaceholder, slideScene, KNOWN, defaultBox, textRect,
} from "../static/assets/pptxscene.js";
import { layoutText, fitText, family, fontCSS } from "../static/assets/pptxdraw.js";

const NS = `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`;
const el = (xml) => parseXML(xml.replace(/<(\w+:\w+)(\s|>|\/)/, `<$1 ${NS}$2`)).children[0];
const near = (a, b, eps = 1e-3) => assert.ok(Math.abs(a - b) < eps, `${a} ≉ ${b}`);
const rgb = (c) => [c.r, c.g, c.b].map((v) => Math.round(v * 255));

const THEME = `<a:theme ${NS}><a:themeElements><a:clrScheme name="t">
  <a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1><a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1>
  <a:dk2><a:srgbClr val="1F497D"/></a:dk2><a:lt2><a:srgbClr val="EEECE1"/></a:lt2>
  <a:accent1><a:srgbClr val="4472C4"/></a:accent1><a:accent2><a:srgbClr val="ED7D31"/></a:accent2>
</a:clrScheme><a:fontScheme name="f"><a:majorFont><a:latin typeface="Calibri Light"/></a:majorFont><a:minorFont><a:latin typeface="Calibri"/></a:minorFont></a:fontScheme>
<a:fmtScheme name="s"><a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"><a:tint val="50000"/></a:schemeClr></a:solidFill></a:fillStyleLst>
<a:lnStyleLst><a:ln w="12700"><a:solidFill><a:schemeClr val="phClr"><a:shade val="50000"/></a:schemeClr></a:solidFill></a:ln></a:lnStyleLst>
<a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst></a:fmtScheme></a:themeElements></a:theme>`;
const theme = readTheme(parseXML(THEME));
const ctx = { theme, clrMap: { bg1: "lt1", tx1: "dk1", bg2: "lt2", tx2: "dk2" } };

test("colours come from their value, the theme through the colour map, and their transforms", () => {
  assert.deepEqual(rgb(colorOf(el(`<a:solidFill><a:srgbClr val="C00000"/></a:solidFill>`), ctx)), [192, 0, 0]);
  assert.deepEqual(rgb(colorOf(el(`<a:solidFill><a:schemeClr val="tx2"/></a:solidFill>`), ctx)), [31, 73, 125]);
  assert.deepEqual(rgb(colorOf(el(`<a:solidFill><a:schemeClr val="bg1"/></a:solidFill>`), { ...ctx, clrMap: { bg1: "dk1" } })), [0, 0, 0], "a dark map");
  assert.deepEqual(rgb(colorOf(el(`<a:solidFill><a:srgbClr val="808080"><a:shade val="50000"/></a:srgbClr></a:solidFill>`), ctx)), [64, 64, 64]);
  assert.deepEqual(rgb(colorOf(el(`<a:solidFill><a:srgbClr val="000000"><a:tint val="25000"/></a:srgbClr></a:solidFill>`), ctx)), [191, 191, 191]);
  assert.deepEqual(rgb(colorOf(el(`<a:solidFill><a:srgbClr val="FF0000"><a:lumMod val="50000"/></a:srgbClr></a:solidFill>`), ctx)), [128, 0, 0]);
  assert.deepEqual(rgb(colorOf(el(`<a:solidFill><a:srgbClr val="000000"><a:lumMod val="75000"/><a:lumOff val="25000"/></a:srgbClr></a:solidFill>`), ctx)), [64, 64, 64]);
  near(colorOf(el(`<a:solidFill><a:prstClr val="red"><a:alpha val="40000"/></a:prstClr></a:solidFill>`), ctx).a, 0.4);
  assert.deepEqual(rgb(colorOf(el(`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>`), { ...ctx, phClr: colorOf(el(`<a:x><a:srgbClr val="00FF00"/></a:x>`), ctx) })), [0, 255, 0]);
  assert.equal(colorOf(el(`<a:solidFill/>`), ctx), null);
  assert.equal(css({ r: 1, g: 0.5, b: 0, a: 0.25 }), "rgba(255,128,0,0.25)");
  assert.equal(theme.major, "Calibri Light");
  assert.equal(theme.minor, "Calibri");
});

test("fills and lines are read, with dashes and arrow heads", () => {
  assert.deepEqual(fillOf(el(`<p:spPr><a:noFill/></p:spPr>`), ctx), { type: "none" });
  assert.equal(fillOf(el(`<p:spPr><a:prstGeom prst="rect"/></p:spPr>`), ctx), undefined, "no fill set: inherited");
  const g = fillOf(el(`<p:spPr><a:gradFill><a:gsLst><a:gs pos="100000"><a:srgbClr val="FFFFFF"/></a:gs><a:gs pos="0"><a:srgbClr val="000000"/></a:gs></a:gsLst><a:lin ang="5400000"/></a:gradFill></p:spPr>`), ctx);
  assert.equal(g.type, "grad");
  assert.deepEqual(g.stops.map((s) => s.pos), [0, 1]);
  assert.equal(g.angle, 90);
  const b = fillOf(el(`<p:spPr><a:blipFill><a:blip r:embed="rId4"/><a:srcRect l="10000" b="20000"/></a:blipFill></p:spPr>`), ctx);
  assert.equal(b.embed, "rId4");
  assert.deepEqual(b.crop, { l: 0.1, t: 0, r: 0, b: 0.2 });
  const ln = lineOf(el(`<a:ln w="38100"><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill><a:prstDash val="dash"/><a:tailEnd type="triangle"/></a:ln>`), ctx);
  assert.equal(ln.width, 38100);
  assert.deepEqual(ln.dash, [4, 3]);
  assert.equal(ln.tailEnd, "triangle");
  assert.equal(lineOf(el(`<a:ln><a:noFill/></a:ln>`), ctx).color, null);
  assert.equal(lineOf(el(`<a:ln/>`), ctx), undefined);
});

test("a shape's transform places, rotates and flips its box", () => {
  const t = xfrmOf(el(`<a:xfrm rot="5400000" flipH="1"><a:off x="100" y="200"/><a:ext cx="40" cy="20"/></a:xfrm>`));
  assert.deepEqual([t.x, t.y, t.w, t.h, t.rot, t.flipH, t.flipV], [100, 200, 40, 20, 90, true, false]);
  const plain = boxMatrix({ ...t, rot: 0, flipH: false });
  assert.deepEqual(apply(plain, 0, 0), [100, 200]);
  assert.deepEqual(apply(plain, 40, 20), [140, 220]);
  const turned = boxMatrix({ ...t, flipH: false });
  const [x, y] = apply(turned, 0, 0);
  near(x, 130); near(y, 190);
  const flipped = boxMatrix({ ...t, rot: 0 });
  assert.deepEqual(apply(flipped, 0, 0), [140, 200], "flipped: the left edge is on the right");
  assert.deepEqual(apply(boxMatrix({ ...t, rot: 0 }, false), 0, 0), [100, 200], "text is not mirrored");
  assert.equal(xfrmOf(null), null);
});

const STYLES = el(`<p:txStyles><p:bodyStyle>
  <a:lvl1pPr marL="342900" indent="-342900"><a:buFont typeface="Arial"/><a:buChar char="•"/><a:defRPr sz="3200"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill><a:latin typeface="+mn-lt"/></a:defRPr></a:lvl1pPr>
  <a:lvl2pPr marL="742950" indent="-285750"><a:buChar char="–"/><a:defRPr sz="2800"/></a:lvl2pPr></p:bodyStyle></p:txStyles>`);

test("text inherits through the styles by level, and a shape's own properties win", () => {
  const body = el(`<p:txBody><a:bodyPr anchor="ctr"><a:normAutofit fontScale="85000" lnSpcReduction="10000"/></a:bodyPr><a:lstStyle/>
    <a:p><a:r><a:rPr lang="de-CH"/><a:t>Erste</a:t></a:r></a:p>
    <a:p><a:pPr lvl="1"/><a:r><a:rPr b="1"><a:solidFill><a:srgbClr val="C00000"/></a:solidFill></a:rPr><a:t>Zweite</a:t></a:r></a:p>
    <a:p><a:pPr><a:buNone/></a:pPr><a:r><a:rPr sz="1200" i="1" u="sng" cap="all"/><a:t>ohne Punkt</a:t></a:r><a:br/><a:r><a:t>neu</a:t></a:r></a:p>
    <a:p><a:endParaRPr sz="4000"/></a:p></p:txBody>`);
  const t = textOf(body, [null, STYLES.children[0]], [], ctx);
  assert.equal(t.body.anchor, "ctr");
  assert.deepEqual(t.body.autofit, { kind: "norm", fontScale: 0.85, lnSpcReduction: 0.1 });
  const [a, b, c, d] = t.paragraphs;
  assert.equal(a.marL, 342900);
  assert.equal(a.bullet.text, "•");
  assert.equal(a.bullet.font, "Arial");
  assert.equal(a.runs[0].size, 32);
  assert.equal(a.runs[0].font, "Calibri");
  assert.deepEqual(rgb(a.runs[0].color), [0, 0, 0]);
  assert.equal(b.level, 1);
  assert.equal(b.bullet.text, "–");
  assert.equal(b.runs[0].size, 28);
  assert.equal(b.runs[0].bold, true);
  assert.deepEqual(rgb(b.runs[0].color), [192, 0, 0]);
  assert.equal(c.bullet, null);
  assert.deepEqual([c.runs[0].text, c.runs[0].italic, c.runs[0].underline], ["OHNE PUNKT", true, true]);
  assert.equal(c.runs[1].br, true);
  assert.equal(d.bullet, null, "an empty paragraph shows no bullet");
  assert.equal(d.endSize, 40);
});

test("a shape's style colour sets text over the inherited styles, but not over its own", () => {
  const white = colorOf(el(`<a:x><a:schemeClr val="bg1"/></a:x>`), ctx);
  const t = textOf(el(`<p:txBody><a:bodyPr/><a:p><a:r><a:t>weiss</a:t></a:r></a:p><a:p><a:r><a:rPr><a:solidFill><a:srgbClr val="00FF00"/></a:solidFill></a:rPr><a:t>grün</a:t></a:r></a:p></p:txBody>`),
    [null, STYLES.children[0]], [], ctx, { color: white });
  assert.deepEqual(rgb(t.paragraphs[0].runs[0].color), [255, 255, 255]);
  assert.deepEqual(rgb(t.paragraphs[1].runs[0].color), [0, 255, 0]);
  assert.equal(textOf(null, [], [], ctx), null);
});

test("numbered paragraphs count per level, in every numbering scheme", () => {
  const body = el(`<p:txBody><a:bodyPr/>
    <a:p><a:pPr><a:buAutoNum type="arabicPeriod"/></a:pPr><a:r><a:t>a</a:t></a:r></a:p>
    <a:p><a:pPr lvl="1"><a:buAutoNum type="alphaLcParenR"/></a:pPr><a:r><a:t>b</a:t></a:r></a:p>
    <a:p><a:pPr lvl="1"><a:buAutoNum type="alphaLcParenR"/></a:pPr><a:r><a:t>c</a:t></a:r></a:p>
    <a:p><a:pPr><a:buAutoNum type="arabicPeriod"/></a:pPr><a:r><a:t>d</a:t></a:r></a:p>
    <a:p><a:pPr lvl="1"><a:buAutoNum type="alphaLcParenR"/></a:pPr><a:r><a:t>e</a:t></a:r></a:p></p:txBody>`);
  assert.deepEqual(textOf(body, [], [], ctx).paragraphs.map((p) => p.bullet.text), ["1.", "a)", "b)", "2.", "a)"]);
  assert.equal(autoNumber("romanUcPeriod", 14), "XIV.");
  assert.equal(autoNumber("romanLcParenBoth", 4), "(iv)");
  assert.equal(autoNumber("alphaUcPlain", 28), "AB");
  assert.equal(autoNumber("arabicMinus", 3), "- 3 -");
  assert.equal(autoNumber("circleNumDbPlain", 2), "2.");
});

test("a slide's placeholder finds its layout's by index, its master's by type", () => {
  const list = [{ ph: { type: "title" } }, { ph: { type: "body", idx: "1" } }, { ph: { type: "pic", idx: "13" } }, { ph: { type: "dt", idx: "10" } }];
  assert.equal(findPlaceholder(list, { type: "obj", idx: "13" }), list[2]);
  assert.equal(findPlaceholder(list, { type: "ctrTitle" }), list[0]);
  assert.equal(findPlaceholder(list, { type: "obj", idx: "99" }), list[1], "a content placeholder falls back to the body");
  assert.equal(findPlaceholder(list, { type: "subTitle", idx: "1" }, true), list[1], "the master's body for a subtitle");
  assert.equal(findPlaceholder(list, { type: "dt" }, true), list[3]);
  assert.equal(findPlaceholder(null, { type: "dt" }), null);
  const title = defaultBox({ type: "title" }, 1000, 500);
  assert.deepEqual([title.x, title.y, title.w], [70, 20, 860], "a placeholder placed nowhere stands where a default layout puts it");
  assert.ok(defaultBox({ type: "obj", idx: "1" }, 1000, 500).y > title.y + title.h);
});

test("preset shapes trace their outlines; unknown ones are rectangles", () => {
  const rect = geometry({ preset: "rect", adj: {} }, 10, 5).paths[0].cmds;
  assert.deepEqual(rect.filter((c) => c.op !== "Z").map((c) => [c.x, c.y]), [[0, 0], [10, 0], [10, 5], [0, 5]]);
  assert.ok(geometry({ preset: "roundRect", adj: {} }, 10, 5).paths[0].cmds.some((c) => c.op === "E"));
  assert.deepEqual(geometry({ preset: "triangle", adj: { adj: 0 } }, 10, 5).paths[0].cmds[0], { op: "M", x: 0, y: 0 });
  const line = geometry({ preset: "line", adj: {} }, 10, 5).paths[0];
  assert.equal(line.fill, false);
  assert.deepEqual(geometry({ preset: "heart", adj: {} }, 10, 5), geometry({ preset: "rect", adj: {} }, 10, 5));
  assert.ok(!KNOWN.has("heart"));
  for (const p of KNOWN) assert.ok(geometry({ preset: p, adj: {} }, 100, 50).paths.length >= 1, p);
  const custom = geometry({ custom: true, paths: [{ w: 100, h: 100, fill: true, stroke: true, cmds: [
    { op: "moveTo", pts: [["0", "0"]] }, { op: "lnTo", pts: [["r", "b"]] },
    { op: "cubicBezTo", pts: [["50", "0"], ["50", "100"], ["0", "100"]] }, { op: "arcTo", pts: [], wR: "50", hR: "50", stAng: "0", swAng: "5400000" },
    { op: "quadBezTo", pts: [["10", "10"], ["20", "20"]] }, { op: "close", pts: [] }] }] }, 200, 50).paths[0].cmds;
  assert.deepEqual([custom[1].x, custom[1].y], [200, 50], "freeform points scale to the shape");
  assert.deepEqual(custom.map((c) => c.op), ["M", "L", "C", "E", "Q", "Z"]);
});

test("text stands in a shape's text rectangle: an ellipse's inscribed one, a triangle's lower half", () => {
  assert.deepEqual(textRect({ preset: "rect", adj: {} }, 100, 50), { x: 0, y: 0, w: 100, h: 50 });
  const e = textRect({ preset: "ellipse", adj: {} }, 100, 100);
  near(e.x, 14.64); near(e.w, 70.72);
  assert.deepEqual(textRect({ preset: "triangle", adj: {} }, 100, 100), { x: 25, y: 50, w: 50, h: 50 });
  assert.deepEqual(textRect({ preset: "chevron", adj: {} }, 200, 100), { x: 50, y: 0, w: 150, h: 100 });
  for (const p of KNOWN) { const r = textRect({ preset: p, adj: {} }, 100, 60); assert.ok(r.w > 0 && r.h > 0 && r.x >= 0 && r.x + r.w <= 100.001, p); }
  assert.deepEqual(textRect({ custom: true, paths: [] }, 10, 20), { x: 0, y: 0, w: 10, h: 20 });
});

// ---- a whole slide ----

function zipOf(files) {
  const enc = new TextEncoder();
  const parts = [], central = [];
  let offset = 0;
  for (const f of files) {
    const name = enc.encode(f.name);
    const raw = Buffer.from(f.data);
    const data = deflateRawSync(raw);
    const local = Buffer.alloc(30);
    local.writeUInt32LE(0x04034b50, 0); local.writeUInt16LE(0x800, 6); local.writeUInt16LE(8, 8);
    local.writeUInt32LE(data.length, 18); local.writeUInt32LE(raw.length, 22); local.writeUInt16LE(name.length, 26);
    const head = Buffer.alloc(46);
    head.writeUInt32LE(0x02014b50, 0); head.writeUInt16LE(0x800, 8); head.writeUInt16LE(8, 10);
    head.writeUInt32LE(data.length, 20); head.writeUInt32LE(raw.length, 24); head.writeUInt16LE(name.length, 28); head.writeUInt32LE(offset, 42);
    parts.push(local, name, data);
    central.push(head, name);
    offset += 30 + name.length + data.length;
  }
  const dir = Buffer.concat(central);
  const end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50, 0); end.writeUInt16LE(files.length, 8); end.writeUInt16LE(files.length, 10);
  end.writeUInt32LE(dir.length, 12); end.writeUInt32LE(offset, 16);
  return new Blob([Buffer.concat([...parts, dir, end])]);
}
const REL = "http://schemas.openxmlformats.org/officeDocument/2006/relationships";
const rels = (list) => `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">${list.map(([id, type, target]) =>
  `<Relationship Id="${id}" Type="${type.startsWith("http") ? type : REL + "/" + type}" Target="${target}"/>`).join("")}</Relationships>`;
const xfrm = (x, y, w, h, more = "") => `<a:xfrm${more}><a:off x="${x}" y="${y}"/><a:ext cx="${w}" cy="${h}"/></a:xfrm>`;

function deck() {
  const master = `<p:sldMaster ${NS}><p:cSld><p:bg><p:bgRef idx="1001"><a:schemeClr val="bg1"/></p:bgRef></p:bg><p:spTree>
    <p:sp><p:nvSpPr><p:cNvPr id="2" name="t"/><p:cNvSpPr/><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr><p:spPr>${xfrm(100, 100, 8000, 1000)}</p:spPr><p:txBody><a:bodyPr anchor="b"/><a:p/></p:txBody></p:sp>
    <p:sp><p:nvSpPr><p:cNvPr id="3" name="b"/><p:cNvSpPr/><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr><p:spPr>${xfrm(100, 1200, 8000, 4000)}</p:spPr><p:txBody><a:bodyPr/><a:p/></p:txBody></p:sp>
    <p:sp><p:nvSpPr><p:cNvPr id="4" name="logo"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr>${xfrm(0, 0, 500, 50)}<a:prstGeom prst="rect"/><a:solidFill><a:schemeClr val="accent2"/></a:solidFill></p:spPr></p:sp>
    </p:spTree></p:cSld><p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2"/>
    <p:txStyles><p:titleStyle><a:lvl1pPr algn="ctr"><a:defRPr sz="4400"><a:solidFill><a:schemeClr val="tx2"/></a:solidFill><a:latin typeface="+mj-lt"/></a:defRPr></a:lvl1pPr></p:titleStyle>${STYLES.children[0] ? "<p:bodyStyle><a:lvl1pPr><a:buChar char=\"•\"/><a:defRPr sz=\"2800\"/></a:lvl1pPr></p:bodyStyle>" : ""}<p:otherStyle><a:lvl1pPr><a:defRPr sz="1800"/></a:lvl1pPr></p:otherStyle></p:txStyles></p:sldMaster>`;
  const layout = `<p:sldLayout ${NS}><p:cSld><p:spTree>
    <p:sp><p:nvSpPr><p:cNvPr id="2" name="t"/><p:cNvSpPr/><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr><p:spPr>${xfrm(200, 300, 7000, 900)}</p:spPr></p:sp>
    </p:spTree></p:cSld></p:sldLayout>`;
  const slide = `<p:sld ${NS}><p:cSld><p:spTree>
    <p:sp><p:nvSpPr><p:cNvPr id="2" name="Titel"/><p:cNvSpPr/><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:p><a:r><a:t>Ziele</a:t></a:r></a:p></p:txBody></p:sp>
    <p:sp><p:nvSpPr><p:cNvPr id="3" name="Inhalt"/><p:cNvSpPr/><p:nvPr><p:ph idx="1"/></p:nvPr></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:p><a:r><a:t>Punkt</a:t></a:r></a:p></p:txBody></p:sp>
    <p:sp><p:nvSpPr><p:cNvPr id="4" name="Form"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr>${xfrm(1000, 1000, 2000, 1000, ' rot="5400000"')}<a:prstGeom prst="ellipse"/></p:spPr>
      <p:style><a:lnRef idx="1"><a:schemeClr val="accent1"/></a:lnRef><a:fillRef idx="2"><a:schemeClr val="accent1"/></a:fillRef><a:effectRef idx="0"><a:schemeClr val="accent1"/></a:effectRef><a:fontRef idx="minor"><a:schemeClr val="lt1"/></a:fontRef></p:style>
      <p:txBody><a:bodyPr/><a:p><a:r><a:t>Kreis</a:t></a:r></a:p></p:txBody></p:sp>
    <p:sp><p:nvSpPr><p:cNvPr id="9" name="versteckt" hidden="1"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr>${xfrm(0, 0, 10, 10)}</p:spPr></p:sp>
    <p:grpSp><p:nvGrpSpPr><p:cNvPr id="5" name="G"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="5000" y="5000"/><a:ext cx="200" cy="200"/><a:chOff x="0" y="0"/><a:chExt cx="100" cy="100"/></a:xfrm></p:grpSpPr>
      <p:pic><p:nvPicPr><p:cNvPr id="6" name="Bild"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr><p:blipFill><a:blip r:embed="rId2"/><a:srcRect t="10000"/></p:blipFill><p:spPr>${xfrm(50, 50, 50, 50)}</p:spPr></p:pic>
      <p:pic><p:nvPicPr><p:cNvPr id="7" name="EMF"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr><p:blipFill><a:blip r:embed="rId3"/></p:blipFill><p:spPr>${xfrm(0, 0, 10, 10)}</p:spPr></p:pic>
    </p:grpSp>
    <p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="8" name="Tab"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr><p:xfrm><a:off x="100" y="6000"/><a:ext cx="2000" cy="600"/></p:xfrm>
      <a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table"><a:tbl><a:tblPr firstRow="1" bandRow="1"/><a:tblGrid><a:gridCol w="1000"/><a:gridCol w="1000"/></a:tblGrid>
      <a:tr h="300"><a:tc><a:txBody><a:bodyPr/><a:p><a:r><a:t>Kopf</a:t></a:r></a:p></a:txBody><a:tcPr/></a:tc><a:tc><a:txBody><a:bodyPr/><a:p><a:r><a:t>Wert</a:t></a:r></a:p></a:txBody><a:tcPr><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></a:tcPr></a:tc></a:tr>
      <a:tr h="300"><a:tc gridSpan="2"><a:txBody><a:bodyPr/><a:p><a:r><a:t>breit</a:t></a:r></a:p></a:txBody><a:tcPr/></a:tc><a:tc hMerge="1"><a:txBody><a:bodyPr/><a:p/></a:txBody><a:tcPr/></a:tc></a:tr>
      </a:tbl></a:graphicData></a:graphic></p:graphicFrame>
    <p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="10" name="Chart"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr><p:xfrm><a:off x="3000" y="6000"/><a:ext cx="1000" cy="600"/></p:xfrm>
      <a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" r:id="rId9"/></a:graphicData></a:graphic></p:graphicFrame>
    </p:spTree></p:cSld></p:sld>`;
  return zipOf([
    { name: "_rels/.rels", data: rels([["rId1", "officeDocument", "ppt/presentation.xml"]]) },
    { name: "ppt/presentation.xml", data: `<p:presentation ${NS}><p:sldIdLst><p:sldId id="256" r:id="rId1"/></p:sldIdLst><p:sldSz cx="9144000" cy="6858000"/><p:defaultTextStyle><a:lvl1pPr><a:defRPr sz="1800"/></a:lvl1pPr></p:defaultTextStyle></p:presentation>` },
    { name: "ppt/_rels/presentation.xml.rels", data: rels([["rId1", "slide", "slides/slide1.xml"]]) },
    { name: "ppt/slides/slide1.xml", data: slide },
    { name: "ppt/slides/_rels/slide1.xml.rels", data: rels([["rId1", "slideLayout", "../slideLayouts/slideLayout1.xml"], ["rId2", "image", "../media/image1.png"], ["rId3", "image", "../media/image2.emf"]]) },
    { name: "ppt/slideLayouts/slideLayout1.xml", data: layout },
    { name: "ppt/slideLayouts/_rels/slideLayout1.xml.rels", data: rels([["rId1", "slideMaster", "../slideMasters/slideMaster1.xml"]]) },
    { name: "ppt/slideMasters/slideMaster1.xml", data: master },
    { name: "ppt/slideMasters/_rels/slideMaster1.xml.rels", data: rels([["rId1", "theme", "../theme/theme1.xml"]]) },
    { name: "ppt/theme/theme1.xml", data: THEME },
    { name: "ppt/media/image1.png", data: Buffer.from([0x89, 0x50, 0x4e, 0x47]) },
  ]);
}

test("a slide's scene draws its master's shapes, inherits its placeholders and names what it cannot draw", async () => {
  const d = await readDeck(deck());
  assert.deepEqual(d.size, { cx: 9144000, cy: 6858000 });
  const scene = await slideScene(d.pkg, d, d.slides[0].part, 1);
  assert.deepEqual(rgb(scene.background.color), [255, 255, 255]);
  const [logo, title, body, circle, pic, emf, table, chart] = scene.items;
  assert.equal(scene.items.length, 8, scene.items.map((i) => i.kind).join());
  assert.deepEqual([logo.kind, rgb(logo.fill.color)], ["shape", [237, 125, 49]], "the master's own shape, its placeholders not");
  assert.deepEqual([title.m[4], title.m[5], title.w], [200, 300, 7000], "the title stands where its layout puts it");
  assert.equal(title.text.body.anchor, "b", "the master's body properties");
  assert.equal(title.text.paragraphs[0].align, "ctr");
  assert.equal(title.text.paragraphs[0].runs[0].font, "Calibri Light");
  assert.equal(title.text.paragraphs[0].runs[0].size, 44);
  assert.deepEqual(rgb(title.text.paragraphs[0].runs[0].color), [31, 73, 125]);
  assert.deepEqual([body.m[4], body.m[5]], [100, 1200], "a content placeholder without a layout counterpart takes the master's body");
  assert.equal(body.text.paragraphs[0].bullet.text, "•");
  assert.equal(body.fill.type, "none");
  assert.equal(circle.geom.preset, "ellipse");
  assert.deepEqual(rgb(circle.fill.color), [162, 185, 226], "the theme's second fill, a tint of accent 1");
  assert.equal(circle.line.width, 12700);
  assert.deepEqual(rgb(circle.text.paragraphs[0].runs[0].color), [255, 255, 255]);
  const [cx, cy] = apply(circle.m, 0, 0);
  near(cx, 2500); near(cy, 500);
  assert.equal(pic.kind, "picture");
  assert.equal(pic.media, "ppt/media/image1.png");
  assert.deepEqual(apply(pic.m, 0, 0), [5100, 5100], "a group scales its children's frame");
  assert.equal(pic.w, 50);
  assert.equal(emf.kind, "missing");
  assert.equal(table.kind, "table");
  assert.equal(table.rows[0].cells[0].text.paragraphs[0].runs[0].bold, true, "the header row of the default style is bold");
  assert.deepEqual(rgb(table.rows[0].cells[1].fill.color), [255, 0, 0], "a cell's own fill wins");
  assert.equal(table.rows[1].cells[0].span, 2);
  assert.equal(table.rows[1].cells[1].merged, true);
  assert.equal(chart.what, "chart");
  assert.deepEqual([...scene.missing].sort(), ["chart", "picture"]);
  assert.deepEqual([...scene.media], ["ppt/media/image1.png"]);
});

// ---- text layout ----

// Every letter is half as wide as the font is high.
const measure = (font, text) => Number(/([\d.]+)px/.exec(font)[1]) * 0.5 * [...text].length;
const body = (paragraphs, more = {}) => ({ body: { lIns: 0, tIns: 0, rIns: 0, bIns: 0, anchor: "t", wrap: true, autofit: null, ...more }, paragraphs });
const para = (text, more = {}) => ({
  level: 0, align: "l", marL: 0, indent: 0, lnSpc: { pct: 1 }, spcBef: { pts: 0 }, spcAft: { pts: 0 }, bullet: null, endSize: 10,
  runs: text.split("|").map((t) => (t === "\n" ? { text: "\n", br: true, size: 10 } : { text: t, size: 10, font: "Calibri", color: null })), ...more,
});
// At k = 1/12700 a point is a pixel: size 10 is 10 px, a letter 5 px.
const K = 1 / 12700;

test("text wraps at spaces, breaks long words and honours line breaks", () => {
  const out = layoutText(body([para("aaaa bbbb cccc")]), 40, 100, K, measure);
  assert.deepEqual(out.lines.map((l) => l.items.map((i) => i.text).join("")), ["aaaa", "bbbb", "cccc"], "spaces at a line's end are dropped");
  assert.deepEqual(layoutText(body([para("aaaa bbbb cccc")]), 50, 100, K, measure).lines.map((l) => l.items.map((i) => i.text).join("")), ["aaaa bbbb", "cccc"]);
  const long = layoutText(body([para("abcdefghijklmnop")]), 40, 100, K, measure);
  assert.deepEqual(long.lines.map((l) => l.items.map((i) => i.text).join("")), ["abcdefgh", "ijklmnop"]);
  const br = layoutText(body([para("eins|\n|zwei")]), 500, 100, K, measure);
  assert.equal(br.lines.length, 2);
  const nowrap = layoutText(body([para("aaaa bbbb cccc")], { wrap: false }), 20, 100, K, measure);
  assert.equal(nowrap.lines.length, 1);
  assert.equal(out.lines[1].baseline - out.lines[0].baseline, 12, "single spacing is 1.2 times the font size");
});

test("text aligns, indents its bullets and anchors in its box", () => {
  const ctr = layoutText(body([para("ab", { align: "ctr" })]), 100, 100, K, measure);
  assert.equal(ctr.lines[0].items[0].x, 45);
  const r = layoutText(body([para("ab", { align: "r" })]), 100, 100, K, measure);
  assert.equal(r.lines[0].items[0].x, 90);
  const bullet = layoutText(body([para("aaaa bbbb", { marL: 20 * 12700, indent: -20 * 12700, bullet: { text: "•", size: 10, color: null, font: "Arial" } })]), 60, 100, K, measure);
  assert.equal(bullet.lines[0].bullet.x, 0);
  assert.equal(bullet.lines[0].items[0].x, 20, "text hangs after its bullet");
  assert.equal(bullet.lines[1].items[0].x, 20);
  assert.equal(bullet.lines[1].bullet, null);
  const bottom = layoutText(body([para("a")], { anchor: "b" }), 100, 100, K, measure);
  near(bottom.lines[0].baseline, 100 - 12 + 12 - 2.5);
  const middle = layoutText(body([para("a")], { anchor: "ctr" }), 100, 100, K, measure);
  near(middle.lines[0].baseline, 44 + 12 - 2.5);
  const spaced = layoutText(body([para("a"), para("b", { spcBef: { pts: 6 }, lnSpc: { pct: 1.5 } })]), 100, 100, K, measure);
  assert.equal(spaced.height, 12 + 6 + 18);
});

test("autofit shrinks the text by the stored scale, or until it fits", () => {
  const many = [para("aaaa"), para("bbbb"), para("cccc"), para("dddd")];
  assert.equal(fitText(body(many), 100, 30, K, measure).overflow, true, "without autofit text overflows");
  const stored = fitText(body(many, { autofit: { kind: "norm", fontScale: 0.5, lnSpcReduction: 0 } }), 100, 30, K, measure);
  assert.equal(stored.lines[0].items[0].px, 5);
  const shrunk = fitText(body(many, { autofit: { kind: "norm", fontScale: null, lnSpcReduction: 0 } }), 100, 30, K, measure);
  assert.equal(shrunk.overflow, false);
  assert.ok(shrunk.lines[0].items[0].px < 10);
});

test("fonts are named with the fonts that stand in for them", () => {
  assert.equal(family("Calibri"), '"Calibri", "Carlito", "Liberation Sans", Arial, Helvetica, sans-serif');
  assert.match(family("Times New Roman"), /Liberation Serif/);
  assert.match(family("Consolas"), /monospace$/);
  assert.equal(fontCSS({ bold: true, italic: true, font: "Arial" }, 12), 'italic bold 12.00px "Arial", "Liberation Sans", Arial, Helvetica, sans-serif');
});
