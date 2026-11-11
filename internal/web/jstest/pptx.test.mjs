// Unit tests of the album's slide import that need no browser (pptx.js,
// ADR-0186): reading a presentation's slides in their order, which are
// hidden, their titles and speaker's notes, and pairing the slides with
// the pictures they were exported as. They run in Node.js without a
// browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
//
// The import from the album's Upload is in e2e/smoke.cjs.
import { test } from "node:test";
import assert from "node:assert/strict";
import { deflateRawSync } from "node:zlib";
import {
  isDeck, parseXML, titleOf, notesOf, resolve, readDeck, numberOf, pair, slideName, widthOf, folderName,
} from "../static/assets/pptx.js";

// zipOf packs files into a ZIP archive, deflated.
function zipOf(files) {
  const enc = new TextEncoder();
  const parts = [];
  const central = [];
  let offset = 0;
  for (const f of files) {
    const name = enc.encode(f.name);
    const raw = Buffer.from(f.data);
    const data = deflateRawSync(raw);
    const local = Buffer.alloc(30);
    local.writeUInt32LE(0x04034b50, 0);
    local.writeUInt16LE(0x800, 6);
    local.writeUInt16LE(8, 8);
    local.writeUInt32LE(data.length, 18);
    local.writeUInt32LE(raw.length, 22);
    local.writeUInt16LE(name.length, 26);
    const head = Buffer.alloc(46);
    head.writeUInt32LE(0x02014b50, 0);
    head.writeUInt16LE(0x800, 8);
    head.writeUInt16LE(8, 10);
    head.writeUInt32LE(data.length, 20);
    head.writeUInt32LE(raw.length, 24);
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

const NS = `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`;
const REL = "http://schemas.openxmlformats.org/officeDocument/2006/relationships";
const rels = (list) => `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">${list.map(([id, type, target]) =>
  `<Relationship Id="${id}" Type="${REL}/${type}" Target="${target}"/>`).join("")}</Relationships>`;
const para = (p) => `<a:p>${p}</a:p>`;
const run = (text) => `<a:r><a:rPr lang="de-CH"/><a:t>${text}</a:t></a:r>`;
const shape = (ph, paragraphs) => `<p:sp><p:nvSpPr><p:cNvPr id="2" name="x"/><p:cNvSpPr/><p:nvPr>${ph}</p:nvPr></p:nvSpPr>` +
  (paragraphs ? `<p:txBody><a:bodyPr/>${paragraphs.map(para).join("")}</p:txBody>` : "") + "</p:sp>";
const slide = (title, { hidden = false, more = "" } = {}) => `<?xml version="1.0" encoding="UTF-8"?>
<p:sld ${NS}${hidden ? ` show="0"` : ""}><p:cSld><p:spTree>${title === null ? "" : shape(`<p:ph type="title"/>`, [run(title)])}${more}</p:spTree></p:cSld></p:sld>`;
const notes = (paragraphs) => `<?xml version="1.0" encoding="UTF-8"?>
<p:notes ${NS}><p:cSld><p:spTree>${shape(`<p:ph type="sldImg"/>`)}${shape(`<p:ph type="body" idx="1"/>`, paragraphs)}${shape(`<p:ph type="sldNum" idx="5"/>`, [run("3")])}</p:spTree></p:cSld></p:notes>`;

// deck builds a presentation of four slide files shown in the order
// 1, 3, 2, 4, the third hidden: slide files are not numbered by their
// place in the presentation once slides have been moved.
function deck() {
  return zipOf([
    { name: "[Content_Types].xml", data: "<Types/>" },
    { name: "_rels/.rels", data: rels([["rId1", "officeDocument", "ppt/presentation.xml"]]) },
    { name: "ppt/presentation.xml", data: `<?xml version="1.0"?><p:presentation ${NS}><p:sldMasterIdLst/><p:sldIdLst>` +
      `<p:sldId id="256" r:id="rId2"/><p:sldId id="258" r:id="rId4"/><p:sldId id="257" r:id="rId3"/><p:sldId id="259" r:id="rId5"/>` +
      `</p:sldIdLst><p:sldSz cx="12192000" cy="6858000"/></p:presentation>` },
    { name: "ppt/_rels/presentation.xml.rels", data: rels([["rId1", "slideMaster", "slideMasters/slideMaster1.xml"],
      ["rId2", "slide", "slides/slide1.xml"], ["rId3", "slide", "slides/slide2.xml"], ["rId4", "slide", "/ppt/slides/slide3.xml"],
      ["rId5", "slide", "slides/slide4.xml"]]) },
    { name: "ppt/slides/slide1.xml", data: slide("Willkommen &amp; Ziele") },
    { name: "ppt/slides/_rels/slide1.xml.rels", data: rels([["rId1", "slideLayout", "../slideLayouts/slideLayout1.xml"], ["rId2", "notesSlide", "../notesSlides/notesSlide1.xml"]]) },
    { name: "ppt/notesSlides/notesSlide1.xml", data: notes([run("Begrüssung"), run("Erstens") + "<a:br/>" + run("zweitens"), "", run("Danke &lt;alle&gt;")]) },
    { name: "ppt/slides/slide2.xml", data: slide("Versteckt", { hidden: true }) },
    { name: "ppt/slides/_rels/slide2.xml.rels", data: rels([["rId2", "notesSlide", "../notesSlides/notesSlide2.xml"]]) },
    { name: "ppt/notesSlides/notesSlide2.xml", data: notes([run("geheim")]) },
    { name: "ppt/slides/slide3.xml", data: slide("Agenda:", { more: "" }).replace("<a:t>Agenda:</a:t>", "<a:t>Agenda:</a:t></a:r><a:br/><a:r><a:t>Q4/2026</a:t>") },
    { name: "ppt/slides/slide4.xml", data: slide(null, { more: shape("", [run("Bild")]) }) },
  ]);
}

test("presentations are told by their extension", () => {
  assert.ok(isDeck("Quarterly review.pptx"));
  assert.ok(isDeck("Show.PPSX"));
  assert.ok(!isDeck("old.ppt"));
  assert.ok(!isDeck("slides.zip"));
  assert.ok(!isDeck(undefined));
});

test("XML is read into elements without prefixes, attributes by their qualified names", () => {
  const doc = parseXML(`<?xml version="1.0"?><!-- c --><p:x a="1" r:id='r&amp;2'><p:y/>te&#x41;&#66;xt<![CDATA[<raw>]]></p:x>`);
  const x = doc.children[0];
  assert.equal(x.name, "x");
  assert.deepEqual(x.attrs, { a: "1", "r:id": "r&2" });
  assert.equal(x.children[0].name, "y");
  assert.deepEqual(x.children.slice(1), ["teABxt", "<raw>"]);
  assert.throws(() => parseXML("<a><b></a></b>"), /nest/);
  assert.throws(() => parseXML("<a><b>"), /unclosed/);
  assert.throws(() => parseXML("<a"), /unterminated/);
});

test("a slide's title is its title placeholder's text on one line", () => {
  const doc = (body) => parseXML(`<p:sld ${NS}><p:cSld><p:spTree>${body}</p:spTree></p:cSld></p:sld>`);
  assert.equal(titleOf(doc(shape(`<p:ph type="ctrTitle"/>`, [run("Erste "), run("Zeile") + "<a:br/>" + run("zwei")]))), "Erste Zeile zwei");
  assert.equal(titleOf(doc(shape(`<p:ph idx="1"/>`, [run("Inhalt")]))), "", "a body placeholder is no title");
  assert.equal(titleOf(doc(`<p:grpSp>${shape(`<p:ph type="title"/>`, [run("  In  einer Gruppe ")])}</p:grpSp>`)), "In einer Gruppe");
});

test("a notes page's notes leave out the slide's picture, number, header, footer and date", () => {
  const doc = parseXML(notes([run("Eins  "), run("Zwei") + "<a:br/>" + run("Drei"), "", "", "", run("Vier")]));
  assert.equal(notesOf(doc), "Eins\nZwei\nDrei\n\nVier");
  const footer = parseXML(`<p:notes ${NS}><p:cSld><p:spTree>${shape(`<p:ph type="ftr"/>`, [run("Fusszeile")])}${shape(`<p:ph type="dt"/>`, [run("1.1.2026")])}</p:spTree></p:cSld></p:notes>`);
  assert.equal(notesOf(footer), "");
});

test("relationships resolve relative to their part, or from the root", () => {
  assert.equal(resolve("ppt/presentation.xml", "slides/slide1.xml"), "ppt/slides/slide1.xml");
  assert.equal(resolve("ppt/slides/slide1.xml", "../notesSlides/notesSlide1.xml"), "ppt/notesSlides/notesSlide1.xml");
  assert.equal(resolve("ppt/slides/slide1.xml", "/ppt/media/image1.png"), "ppt/media/image1.png");
  assert.equal(resolve("", "ppt/presentation.xml"), "ppt/presentation.xml");
});

test("a presentation's slides are read in the presentation's order, with hidden ones, titles and notes", async () => {
  const { slides, size } = await readDeck(deck());
  assert.deepEqual(size, { cx: 12192000, cy: 6858000 });
  assert.deepEqual(slides, [
    { number: 1, hidden: false, title: "Willkommen & Ziele", notes: "Begrüssung\nErstens\nzweitens\n\nDanke <alle>", part: "ppt/slides/slide1.xml" },
    { number: 2, hidden: false, title: "Agenda: Q4/2026", notes: "", part: "ppt/slides/slide3.xml" },
    { number: 3, hidden: true, title: "Versteckt", notes: "geheim", part: "ppt/slides/slide2.xml" },
    { number: 4, hidden: false, title: "", notes: "", part: "ppt/slides/slide4.xml" },
  ]);
});

test("files that are no presentation are refused", async () => {
  await assert.rejects(readDeck(new Blob(["not a zip at all"])), /not a presentation/);
  await assert.rejects(readDeck(zipOf([{ name: "word/document.xml", data: "<w:document/>" }])), /missing/);
  const empty = zipOf([
    { name: "ppt/presentation.xml", data: `<p:presentation ${NS}><p:sldIdLst/></p:presentation>` },
    { name: "ppt/_rels/presentation.xml.rels", data: rels([]) },
  ]);
  await assert.rejects(readDeck(empty), /no slides/);
  const other = zipOf([{ name: "ppt/presentation.xml", data: `<w:document/>` }]);
  await assert.rejects(readDeck(other), /not a presentation/);
});

test("a picture's slide number is the last number in its name", () => {
  assert.equal(numberOf("Slide3.PNG"), 3);
  assert.equal(numberOf("Präsentation 2/Folie12.png"), 12);
  assert.equal(numberOf("2026 Review - 07.jpg"), 7);
  assert.equal(numberOf("cover.png"), null);
});

const slidesOf = (hidden = []) => [1, 2, 3, 4].map((n) => ({ number: n, hidden: hidden.includes(n), title: `T${n}`, notes: "" }));
const pics = (...names) => names.map((name) => ({ name }));
const shownPairs = (r) => r.pairs.map((p) => `${p.slide.number}:${p.picture.name}`);

test("pictures of every slide pair by their numbers, whatever order they come in", () => {
  const r = pair(slidesOf(), pics("Slide2.PNG", "Slide4.PNG", "Slide1.PNG", "Slide3.PNG"));
  assert.deepEqual(shownPairs(r), ["1:Slide1.PNG", "2:Slide2.PNG", "3:Slide3.PNG", "4:Slide4.PNG"]);
  assert.equal(r.hidden, 0);
});

test("hidden slides are left out, whether the export has their pictures or not", () => {
  const all = pair(slidesOf([3]), pics("Folie1.png", "Folie2.png", "Folie3.png", "Folie4.png"));
  assert.deepEqual(shownPairs(all), ["1:Folie1.png", "2:Folie2.png", "4:Folie4.png"]);
  assert.equal(all.hidden, 1);
  const gaps = pair(slidesOf([3]), pics("Folie1.png", "Folie2.png", "Folie4.png"));
  assert.deepEqual(shownPairs(gaps), ["1:Folie1.png", "2:Folie2.png", "4:Folie4.png"]);
  const consecutive = pair(slidesOf([1]), pics("Folie1.png", "Folie2.png", "Folie3.png"));
  assert.deepEqual(shownPairs(consecutive), ["2:Folie1.png", "3:Folie2.png", "4:Folie3.png"]);
});

test("pictures numbered from zero pair in their order", () => {
  assert.deepEqual(shownPairs(pair(slidesOf(), pics("s0.png", "s1.png", "s2.png", "s3.png"))), ["1:s0.png", "2:s1.png", "3:s2.png", "4:s3.png"]);
});

test("pictures that fit the slides neither way are refused, saying how many of each", () => {
  const mismatch = (slides, pictures) => {
    try { pair(slides, pictures); } catch (err) { return err; }
    assert.fail("paired");
  };
  let err = mismatch(slidesOf(), pics("Slide1.png", "Slide2.png"));
  assert.equal(err.code, "mismatch");
  assert.equal(err.slides, 4);
  assert.equal(err.pictures, 2);
  assert.equal(mismatch(slidesOf(), pics("a.png", "b.png", "c.png", "d.png")).code, "mismatch", "names without numbers");
  assert.equal(mismatch(slidesOf(), pics("1.png", "1.jpg", "2.png", "3.png")).code, "mismatch", "a number twice");
  assert.equal(mismatch(slidesOf(), pics("1.png", "2.png", "5.png", "6.png")).code, "mismatch", "numbers beyond the slides");
  err = mismatch(slidesOf([1, 2, 3, 4]), pics("1.png"));
  assert.equal(err.slides, 0);
  assert.equal(mismatch(slidesOf(), []).code, "mismatch");
});

test("a single slide takes a single picture, whatever its name", () => {
  const one = [{ number: 1, hidden: false, title: "", notes: "" }];
  assert.equal(pair(one, pics("cover.png")).pairs[0].picture.name, "cover.png");
  assert.equal(pair(one, pics("Folie7.png")).pairs[0].picture.name, "Folie7.png");
});

test("a slide's picture is named by its place and title, keeping the picture's extension", () => {
  assert.equal(slideName(3, 2, "Our goals", "Slide3.PNG"), "03 Our goals.png");
  assert.equal(slideName(12, widthOf(120), "Agenda: Q4/2026 <draft>?", "x.jpeg"), "012 Agenda Q4 2026 draft.jpg");
  assert.equal(slideName(1, 2, "", "Folie1.png"), "01.png");
  assert.equal(slideName(1, 2, "Ende...", "f"), "01 Ende.png");
  assert.equal(slideName(5, 2, "x".repeat(300), "a.png").length, "05 ".length + 80 + ".png".length);
  assert.equal(widthOf(9), 2);
  assert.equal(widthOf(100), 3);
});

test("the folder is named after the presentation's file", () => {
  assert.equal(folderName("Quartal 3/2026 Review.pptx", "Slides"), "2026 Review");
  assert.equal(folderName("Kick-off: Projekt.pptx", "Slides"), "Kick-off Projekt");
  assert.equal(folderName(".pptx", "Folien"), "Folien");
});
