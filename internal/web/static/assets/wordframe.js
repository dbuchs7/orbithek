// The parts of a page framed as its Word template's document (ADR-0167)
// that need no browser: which header and footer a sheet carries, the
// formatting of a run as checked style values, the parts of a line a tab
// separates, and the number a page field shows. The server sends text and
// values only; these functions admit what they recognise and drop the rest,
// so nothing the template holds becomes markup or a stylesheet.

const colour = /^#[0-9A-Fa-f]{6}$/;
const fontName = /^[\p{L}\p{N} ._-]{1,64}$/u;

// chromeOf is the header and footer of sheet i (from 0): the first sheet's
// own when the template gives the first page one.
export function chromeOf(frame, i) {
  if (i === 0 && frame.titlePage) return { header: frame.firstHeader || [], footer: frame.firstFooter || [] };
  return { header: frame.header || [], footer: frame.footer || [] };
}

// runStyle is a run's formatting as CSS properties of the style object,
// each value checked.
export function runStyle(run) {
  const out = {};
  if (run.b === true) out.fontWeight = "700";
  if (run.b === false) out.fontWeight = "400";
  if (run.i === true) out.fontStyle = "italic";
  if (run.i === false) out.fontStyle = "normal";
  const deco = [];
  if (run.u === true) deco.push("underline");
  if (run.s === true) deco.push("line-through");
  if (deco.length) out.textDecoration = deco.join(" ");
  else if (run.u === false || run.s === false) out.textDecoration = "none";
  if (run.caps === true) out.textTransform = "uppercase";
  if (run.caps === false) out.textTransform = "none";
  if (typeof run.size === "number" && run.size > 0 && run.size < 400) out.fontSize = `${run.size}pt`;
  if (colour.test(run.color || "")) out.color = run.color;
  if (colour.test(run.fill || "")) out.backgroundColor = run.fill;
  if (fontName.test(run.font || "")) out.fontFamily = `"${run.font}", system-ui, sans-serif`;
  return out;
}

// tabSegments splits a paragraph's runs at its tabs: a header's title at
// the left and its page number at the right are two segments.
export function tabSegments(runs) {
  const out = [[]];
  for (const r of runs || []) {
    if (r.tab) out.push([]);
    else out[out.length - 1].push(r);
  }
  return out;
}

// fieldText is what a page field shows on sheet n of all.
export function fieldText(run, n, all) {
  if (run.field === "page") return String(n);
  if (run.field === "pages") return String(all);
  return "";
}

// knownClass admits a style's class of the template's stylesheet.
export function knownClass(c) {
  return /^s\d{1,4}$/.test(c || "") ? c : "";
}

// pictureSource admits a picture's address among the template's own.
export function pictureSource(src) {
  return /^\/api\/word-templates\/[a-z][a-z0-9-]{0,39}\/versions\/\d+\/media\/[A-Za-z0-9._-]{1,100}$/.test(src || "") ? src : "";
}

// points admits a length in points for a picture or a cell.
export function points(v) {
  return typeof v === "number" && v > 0 && v <= 2000 ? `${v}pt` : "";
}
