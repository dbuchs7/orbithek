// pptxscene reads what a slide of a presentation shows (ADR-0186): its
// background and, in drawing order, the shapes, pictures, texts and tables
// of its master, its layout and itself, with every inherited property
// resolved — theme colours and fonts, placeholder positions, text styles
// by level, fills and lines from the theme's style references. The result
// is plain data in EMU that assets/pptxdraw.js draws onto a canvas. What a
// browser cannot draw (charts, embedded objects, EMF pictures) is named in
// the scene's missing list, so the import can say so. It needs no browser,
// so node --test runs it.
import { all, attrOf, child, kids, rootOf, isType } from "./pptx.js";
import { readChart } from "./pptxchart.js";

const num = (v, d = 0) => (v === undefined || v === null || v === "" || Number.isNaN(Number(v)) ? d : Number(v));
const on = (v) => v === "1" || v === "true" || v === "on";

// ---- colours ----

const PRESET = {
  black: "000000", white: "FFFFFF", red: "FF0000", green: "008000", lime: "00FF00", blue: "0000FF", yellow: "FFFF00",
  gray: "808080", grey: "808080", silver: "C0C0C0", navy: "000080", teal: "008080", maroon: "800000", purple: "800080",
  orange: "FFA500", olive: "808000", aqua: "00FFFF", fuchsia: "FF00FF", darkBlue: "00008B", darkRed: "8B0000",
  darkGreen: "006400", lightGray: "D3D3D3", darkGray: "A9A9A9",
};
const SYS = { window: "FFFFFF", windowText: "000000", btnFace: "F0F0F0", btnText: "000000", highlight: "0078D7", highlightText: "FFFFFF" };
const COLOR_TAGS = new Set(["srgbClr", "schemeClr", "sysClr", "prstClr", "scrgbClr", "hslClr"]);

const hex = (h) => {
  const v = String(h || "000000").replace(/[^0-9a-fA-F]/g, "").padStart(6, "0").slice(-6);
  return { r: parseInt(v.slice(0, 2), 16) / 255, g: parseInt(v.slice(2, 4), 16) / 255, b: parseInt(v.slice(4, 6), 16) / 255, a: 1 };
};
const clamp01 = (v) => Math.min(1, Math.max(0, v));

function toHSL({ r, g, b }) {
  const max = Math.max(r, g, b), min = Math.min(r, g, b);
  const l = (max + min) / 2;
  if (max === min) return { h: 0, s: 0, l };
  const d = max - min;
  const s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
  let h;
  if (max === r) h = (g - b) / d + (g < b ? 6 : 0);
  else if (max === g) h = (b - r) / d + 2;
  else h = (r - g) / d + 4;
  return { h: h / 6, s, l };
}
function fromHSL({ h, s, l }) {
  if (s === 0) return { r: l, g: l, b: l };
  const q = l < 0.5 ? l * (1 + s) : l + s - l * s;
  const p = 2 * l - q;
  const f = (t) => {
    t = (t + 1) % 1;
    if (t < 1 / 6) return p + (q - p) * 6 * t;
    if (t < 1 / 2) return q;
    if (t < 2 / 3) return p + (q - p) * (2 / 3 - t) * 6;
    return p;
  };
  return { r: f(h + 1 / 3), g: f(h), b: f(h - 1 / 3) };
}

// colorOf reads the colour a node holds, or is: srgbClr, schemeClr (through
// the colour map and theme), sysClr, prstClr, scrgbClr or hslClr, with its
// transforms (alpha, lumMod, lumOff, satMod, tint, shade). null when none.
export function colorOf(node, ctx) {
  if (!node) return null;
  const el = COLOR_TAGS.has(node.name) ? node : kids(node).find((k) => COLOR_TAGS.has(k.name));
  if (!el) return null;
  const val = el.attrs.val;
  let c;
  switch (el.name) {
    case "srgbClr": c = hex(val); break;
    case "sysClr": c = hex(el.attrs.lastClr || SYS[val] || "000000"); break;
    case "prstClr": c = hex(PRESET[val] || "000000"); break;
    case "scrgbClr": c = { r: num(el.attrs.r) / 100000, g: num(el.attrs.g) / 100000, b: num(el.attrs.b) / 100000, a: 1 }; break;
    case "hslClr": c = { ...fromHSL({ h: num(el.attrs.hue) / 21600000, s: num(el.attrs.sat) / 100000, l: num(el.attrs.lum) / 100000 }), a: 1 }; break;
    default: {
      if (val === "phClr") c = ctx?.phClr ? { ...ctx.phClr } : hex("000000");
      else {
        const mapped = ctx?.clrMap?.[val] || val;
        c = ctx?.theme?.colors?.[mapped] ? { ...ctx.theme.colors[mapped] } : hex("000000");
      }
    }
  }
  for (const t of kids(el)) {
    const v = num(t.attrs.val, 100000) / 100000;
    switch (t.name) {
      case "alpha": c.a = v; break;
      case "alphaMod": c.a *= v; break;
      case "alphaOff": c.a = clamp01(c.a + v); break;
      case "tint": c = { ...mixWith(c, 1, v), a: c.a }; break;
      case "shade": c = { r: c.r * v, g: c.g * v, b: c.b * v, a: c.a }; break;
      case "lumMod": case "lumOff": case "satMod": case "satOff": {
        const hsl = toHSL(c);
        if (t.name === "lumMod") hsl.l = clamp01(hsl.l * v);
        if (t.name === "lumOff") hsl.l = clamp01(hsl.l + v);
        if (t.name === "satMod") hsl.s = clamp01(hsl.s * v);
        if (t.name === "satOff") hsl.s = clamp01(hsl.s + v);
        c = { ...fromHSL(hsl), a: c.a };
        break;
      }
      case "gray": { const y = 0.3 * c.r + 0.59 * c.g + 0.11 * c.b; c = { r: y, g: y, b: y, a: c.a }; break; }
      case "inv": c = { r: 1 - c.r, g: 1 - c.g, b: 1 - c.b, a: c.a }; break;
      default:
    }
  }
  return c;
}
// mixWith keeps share v of a colour and blends the rest with grey level w:
// a tint of 40 % is 40 % of the colour on white.
const mixWith = (c, w, v) => ({ r: c.r * v + w * (1 - v), g: c.g * v + w * (1 - v), b: c.b * v + w * (1 - v) });

// css writes a colour for the canvas.
export const css = (c) => (c ? `rgba(${Math.round(c.r * 255)},${Math.round(c.g * 255)},${Math.round(c.b * 255)},${+c.a.toFixed(3)})` : "transparent");

// ---- theme ----

// readTheme reads a theme's colours, its heading and body fonts and the
// fill, line and background styles shapes refer to.
export function readTheme(doc) {
  const root = doc ? rootOf(doc) : null;
  const el = root ? all(root, "themeElements")[0] : null;
  const colors = {};
  const scheme = el ? all(el, "clrScheme")[0] : null;
  for (const c of scheme ? kids(scheme) : []) {
    const v = colorOf(c, null);
    if (v) colors[c.name] = v;
  }
  const fonts = all(el || { children: [] }, "fontScheme")[0];
  const face = (which) => {
    const f = fonts ? child(fonts, which) : null;
    return (f && child(f, "latin")?.attrs.typeface) || "Calibri";
  };
  const fmt = el ? all(el, "fmtScheme")[0] : null;
  const list = (name) => (fmt && child(fmt, name) ? kids(child(fmt, name)) : []);
  return {
    colors: { dk1: hex("000000"), lt1: hex("FFFFFF"), ...colors },
    major: face("majorFont"), minor: face("minorFont"),
    fills: list("fillStyleLst"), lines: list("lnStyleLst"), bgFills: list("bgFillStyleLst"),
  };
}

const DEFAULT_MAP = { bg1: "lt1", tx1: "dk1", bg2: "lt2", tx2: "dk2" };
// clrMapOf reads a colour map (p:clrMap, or a:overrideClrMapping).
export function clrMapOf(node) {
  if (!node) return null;
  const out = {};
  for (const [k, v] of Object.entries(node.attrs)) if (!k.includes(":")) out[k] = v;
  return Object.keys(out).length ? out : null;
}

// ---- fills and lines ----

// fillOf reads the fill a properties element sets: none, solid, gradient,
// picture or, for a pattern, its foreground; undefined when it sets none.
export function fillOf(props, ctx) {
  if (!props) return undefined;
  for (const k of kids(props)) {
    switch (k.name) {
      case "noFill": return { type: "none" };
      case "solidFill": return { type: "solid", color: colorOf(k, ctx) || hex("000000") };
      case "gradFill": {
        const stops = all(k, "gs").map((g) => ({ pos: num(g.attrs.pos) / 100000, color: colorOf(g, ctx) || hex("000000") }))
          .sort((a, b) => a.pos - b.pos);
        const lin = child(k, "lin");
        const path = child(k, "path");
        return { type: "grad", stops, angle: lin ? num(lin.attrs.ang) / 60000 : 90, radial: Boolean(path) };
      }
      case "blipFill": return { type: "blip", ...blipOf(k) };
      case "pattFill": return { type: "solid", color: colorOf(child(k, "fgClr"), ctx) || hex("000000") };
      case "grpFill": return { type: "group" };
      default:
    }
  }
  return undefined;
}

// blipOf reads a picture fill: the relationship of its picture and the part
// of it cropped away on each side.
function blipOf(blipFill) {
  const blip = child(blipFill, "blip");
  const src = child(blipFill, "srcRect");
  const alpha = blip ? all(blip, "alphaModFix")[0] : null;
  return {
    embed: blip ? attrOf(blip, "embed", true) : undefined,
    crop: { l: num(src?.attrs.l) / 100000, t: num(src?.attrs.t) / 100000, r: num(src?.attrs.r) / 100000, b: num(src?.attrs.b) / 100000 },
    alpha: alpha ? num(alpha.attrs.amt, 100000) / 100000 : 1,
  };
}

const DASH = { dash: [4, 3], sysDash: [3, 1], dot: [1, 1], sysDot: [1, 1], dashDot: [4, 3, 1, 3], lgDash: [8, 3], lgDashDot: [8, 3, 1, 3], sysDashDot: [3, 1, 1, 1] };

// lineOf reads a line (a:ln): width in EMU, colour, dash pattern in line
// widths, and the arrow heads at its ends; undefined when it sets nothing.
export function lineOf(ln, ctx) {
  if (!ln) return undefined;
  const fill = fillOf(ln, ctx);
  const out = {};
  if (ln.attrs.w !== undefined) out.width = num(ln.attrs.w);
  if (fill) out.color = fill.type === "none" ? null : fill.type === "grad" ? fill.stops[0]?.color : fill.color || null;
  const dash = child(ln, "prstDash")?.attrs.val;
  if (dash) out.dash = DASH[dash] || null;
  for (const end of ["headEnd", "tailEnd"]) {
    const e = child(ln, end);
    if (e && e.attrs.type && e.attrs.type !== "none") out[end] = e.attrs.type;
  }
  return Object.keys(out).length ? out : undefined;
}

// styleRefs reads a shape's style (p:style): the theme's fill, line and font
// it refers to, each with the colour that stands in for phClr.
function styleRefs(style, ctx) {
  if (!style) return {};
  const ref = (name) => {
    const r = child(style, name);
    return r ? { idx: num(r.attrs.idx), color: colorOf(r, ctx), font: r.attrs.idx } : null;
  };
  return { fill: ref("fillRef"), line: ref("lnRef"), font: ref("fontRef") };
}
function themeFill(ref, ctx) {
  if (!ref || ref.idx === 0) return ref ? { type: "none" } : undefined;
  const list = ref.idx >= 1000 ? ctx.theme.bgFills : ctx.theme.fills;
  const el = list[(ref.idx >= 1000 ? ref.idx - 1001 : ref.idx - 1)];
  if (!el) return undefined;
  return fillOf({ children: [el] }, { ...ctx, phClr: ref.color });
}
function themeLine(ref, ctx) {
  if (!ref || ref.idx === 0) return undefined;
  return lineOf(ctx.theme.lines[ref.idx - 1], { ...ctx, phClr: ref.color });
}

// ---- transforms ----

// A transform maps a shape's own box, from (0, 0) to (w, h), onto the
// slide: [a, b, c, d, e, f] takes (x, y) to (a·x + c·y + e, b·x + d·y + f).
export const IDENTITY = [1, 0, 0, 1, 0, 0];
export const mul = (m, n) => [
  m[0] * n[0] + m[2] * n[1], m[1] * n[0] + m[3] * n[1],
  m[0] * n[2] + m[2] * n[3], m[1] * n[2] + m[3] * n[3],
  m[0] * n[4] + m[2] * n[5] + m[4], m[1] * n[4] + m[3] * n[5] + m[5],
];
const translate = (x, y) => [1, 0, 0, 1, x, y];
const scale = (x, y) => [x, 0, 0, y, 0, 0];
const rotate = (deg) => {
  const r = (deg * Math.PI) / 180;
  return [Math.cos(r), Math.sin(r), -Math.sin(r), Math.cos(r), 0, 0];
};
export const apply = (m, x, y) => [m[0] * x + m[2] * y + m[4], m[1] * x + m[3] * y + m[5]];

// xfrmOf reads a transform element (a:xfrm, p:xfrm): offset, size,
// rotation in degrees and flips, and for a group its children's frame.
export function xfrmOf(x) {
  if (!x) return null;
  const off = child(x, "off"), ext = child(x, "ext"), choff = child(x, "chOff"), chext = child(x, "chExt");
  return {
    x: num(off?.attrs.x), y: num(off?.attrs.y), w: num(ext?.attrs.cx), h: num(ext?.attrs.cy),
    rot: num(x.attrs.rot) / 60000, flipH: on(x.attrs.flipH), flipV: on(x.attrs.flipV),
    chX: num(choff?.attrs.x), chY: num(choff?.attrs.y), chW: num(chext?.attrs.cx), chH: num(chext?.attrs.cy),
  };
}
// boxMatrix places a box of the given transform in its parent; withFlips
// false leaves the flips out, as text is not mirrored.
export function boxMatrix(t, withFlips = true) {
  const cx = t.w / 2, cy = t.h / 2;
  let m = mul(translate(t.x + cx, t.y + cy), rotate(t.rot));
  if (withFlips) m = mul(m, scale(t.flipH ? -1 : 1, t.flipV ? -1 : 1));
  return mul(m, translate(-cx, -cy));
}

// ---- text ----

const LEVELS = ["lvl1pPr", "lvl2pPr", "lvl3pPr", "lvl4pPr", "lvl5pPr", "lvl6pPr", "lvl7pPr", "lvl8pPr", "lvl9pPr"];

// mergePara lays the paragraph properties of one element (a level of a
// list style, or a:pPr) over those gathered so far.
function mergePara(base, el, ctx) {
  if (!el) return base;
  const p = { ...base, run: { ...base.run } };
  const a = el.attrs;
  if (a.algn) p.align = a.algn;
  if (a.marL !== undefined) p.marL = num(a.marL);
  if (a.indent !== undefined) p.indent = num(a.indent);
  for (const k of kids(el)) {
    switch (k.name) {
      case "lnSpc": case "spcBef": case "spcAft": {
        const pct = child(k, "spcPct"), pts = child(k, "spcPts");
        p[k.name] = pct ? { pct: num(pct.attrs.val) / 100000 } : pts ? { pts: num(pts.attrs.val) / 100 } : p[k.name];
        break;
      }
      case "buNone": p.bullet = null; break;
      case "buChar": p.bullet = { kind: "char", char: k.attrs.char || "•" }; break;
      case "buAutoNum": p.bullet = { kind: "num", scheme: k.attrs.type || "arabicPeriod", startAt: num(k.attrs.startAt, 1) }; break;
      case "buBlip": p.bullet = { kind: "char", char: "•" }; break;
      case "buFont": p.buFont = k.attrs.typeface; break;
      case "buFontTx": p.buFont = undefined; break;
      case "buClr": p.buColor = colorOf(k, ctx); break;
      case "buClrTx": p.buColor = undefined; break;
      case "buSzPct": p.buSize = num(k.attrs.val, 100000) / 100000; break;
      case "buSzTx": p.buSize = undefined; break;
      case "defRPr": p.run = mergeRun(p.run, k, ctx); break;
      default:
    }
  }
  return p;
}

// mergeRun lays run properties (a:rPr, a:defRPr, a:endParaRPr) over those
// gathered so far.
function mergeRun(base, el, ctx) {
  if (!el) return base;
  const r = { ...base };
  const a = el.attrs;
  if (a.sz !== undefined) r.size = num(a.sz) / 100;
  if (a.b !== undefined) r.bold = on(a.b);
  if (a.i !== undefined) r.italic = on(a.i);
  if (a.u !== undefined) r.underline = a.u !== "none";
  if (a.strike !== undefined) r.strike = a.strike !== "noStrike";
  if (a.baseline !== undefined) r.baseline = num(a.baseline) / 100000;
  if (a.cap !== undefined) r.caps = a.cap === "all";
  for (const k of kids(el)) {
    if (k.name === "solidFill") r.color = colorOf(k, ctx);
    else if (k.name === "gradFill") r.color = colorOf(all(k, "gs")[0], ctx);
    else if (k.name === "noFill") r.color = { r: 0, g: 0, b: 0, a: 0 };
    else if (k.name === "latin" && k.attrs.typeface) r.font = k.attrs.typeface;
  }
  return r;
}

const fontName = (face, ctx) => {
  if (!face || face === "+mn-lt" || face === "+mn-ea" || face === "+mn-cs") return ctx.theme.minor;
  if (face === "+mj-lt" || face === "+mj-ea" || face === "+mj-cs") return ctx.theme.major;
  return face;
};

// textOf resolves a text body: its body properties and its paragraphs, each
// with its properties and runs inherited through the styles listed from
// the lowest priority up (each style a list style element, or null).
export function textOf(txBody, styles, bodyPrs, ctx, extra = {}) {
  if (!txBody) return null;
  const body = { lIns: 91440, tIns: 45720, rIns: 91440, bIns: 45720, anchor: "t", wrap: true, autofit: null, vert: false, anchorCtr: false };
  for (const bp of [...bodyPrs, child(txBody, "bodyPr")]) {
    if (!bp) continue;
    const a = bp.attrs;
    for (const k of ["lIns", "tIns", "rIns", "bIns"]) if (a[k] !== undefined) body[k] = num(a[k]);
    if (a.anchor) body.anchor = a.anchor;
    if (a.anchorCtr !== undefined) body.anchorCtr = on(a.anchorCtr);
    if (a.wrap) body.wrap = a.wrap !== "none";
    if (a.vert) body.vert = a.vert !== "horz";
    if (a.rot !== undefined) body.rot = num(a.rot) / 60000;
    const norm = child(bp, "normAutofit");
    if (norm) body.autofit = { kind: "norm", fontScale: norm.attrs.fontScale ? num(norm.attrs.fontScale) / 100000 : null, lnSpcReduction: num(norm.attrs.lnSpcReduction) / 100000 };
    else if (child(bp, "spAutoFit")) body.autofit = { kind: "shape" };
    else if (child(bp, "noAutofit")) body.autofit = null;
  }
  const own = child(txBody, "lstStyle");
  const base = { align: "l", marL: 0, indent: 0, bullet: null, run: { size: 18, bold: false, italic: false, underline: false, strike: false, baseline: 0, caps: false, color: null, font: undefined } };
  const counters = [];
  const paragraphs = [];
  for (const p of kids(txBody, "p")) {
    const pPr = child(p, "pPr");
    const level = Math.min(8, num(pPr?.attrs.lvl));
    let props = base;
    for (const s of styles) props = mergePara(props, s ? child(s, LEVELS[level]) : null, ctx);
    // A shape's style (fontRef) or a table style sets the text's colour
    // and font over the inherited styles, below the shape's own.
    if (extra.color || extra.font) props = { ...props, run: { ...props.run, ...(extra.color ? { color: extra.color } : {}), ...(extra.font ? { font: extra.font } : {}) } };
    props = mergePara(props, own ? child(own, LEVELS[level]) : null, ctx);
    props = mergePara(props, pPr, ctx);
    const runs = [];
    for (const r of kids(p)) {
      if (r.name === "r" || r.name === "fld") {
        let text = child(r, "t")?.children.filter((c) => typeof c === "string").join("") || "";
        if (r.name === "fld" && r.attrs.type === "slidenum" && extra.slideNumber) text = String(extra.slideNumber);
        const rp = mergeRun(props.run, child(r, "rPr"), ctx);
        runs.push(runOf(text, rp, ctx));
      } else if (r.name === "br") {
        runs.push({ ...runOf("\n", mergeRun(props.run, child(r, "rPr"), ctx), ctx), br: true });
      }
    }
    const end = mergeRun(props.run, child(p, "endParaRPr"), ctx);
    const hasText = runs.some((r) => r.text.trim());
    let bullet = null;
    if (props.bullet && hasText) {
      const first = runs.find((r) => r.text.trim()) || runOf("", props.run, ctx);
      const sizePt = first.size * (props.buSize || 1);
      const color = props.buColor || first.color;
      if (props.bullet.kind === "num") {
        counters.length = level + 1;
        counters[level] = (counters[level] || 0) + 1;
        bullet = { text: autoNumber(props.bullet.scheme, props.bullet.startAt - 1 + counters[level]), size: sizePt, color, font: first.font };
      } else {
        bullet = { text: props.bullet.char, size: sizePt, color, font: props.buFont ? fontName(props.buFont, ctx) : first.font };
      }
    } else if (hasText) counters.length = level;
    paragraphs.push({
      level, align: props.align, marL: props.marL, indent: props.indent,
      lnSpc: props.lnSpc || { pct: 1 }, spcBef: props.spcBef || { pts: 0 }, spcAft: props.spcAft || { pts: 0 },
      bullet, runs, endSize: end.size,
    });
  }
  return { body, paragraphs };
}

function runOf(text, rp, ctx) {
  return {
    text: rp.caps ? text.toUpperCase() : text, size: rp.size, bold: rp.bold, italic: rp.italic, underline: rp.underline,
    strike: rp.strike, baseline: rp.baseline, color: rp.color || ctx.theme.colors[ctx.clrMap?.tx1 || "dk1"] || hex("000000"),
    font: fontName(rp.font, ctx),
  };
}

const ROMAN = [[1000, "m"], [900, "cm"], [500, "d"], [400, "cd"], [100, "c"], [90, "xc"], [50, "l"], [40, "xl"], [10, "x"], [9, "ix"], [5, "v"], [4, "iv"], [1, "i"]];
const roman = (n) => ROMAN.reduce((s, [v, r]) => { while (n >= v) { s += r; n -= v; } return s; }, "");
const alpha = (n) => { let s = ""; while (n > 0) { n--; s = String.fromCharCode(97 + (n % 26)) + s; n = Math.floor(n / 26); } return s; };

// autoNumber writes the number of an automatically numbered paragraph in
// its scheme: arabicPeriod 3 is "3.", romanUcParenR 4 is "IV)".
export function autoNumber(scheme, n) {
  const m = scheme.match(/^(arabic|alphaLc|alphaUc|romanLc|romanUc)(.*)$/);
  if (!m) return `${n}.`;
  const [, kind, form] = m;
  const v = kind === "arabic" ? String(n) : kind === "alphaLc" ? alpha(n) : kind === "alphaUc" ? alpha(n).toUpperCase()
    : kind === "romanLc" ? roman(n) : roman(n).toUpperCase();
  if (form.startsWith("ParenBoth")) return `(${v})`;
  if (form.startsWith("ParenR")) return `${v})`;
  if (form.startsWith("Plain")) return v;
  if (form.startsWith("Minus")) return `- ${v} -`;
  return `${v}.`;
}

// ---- placeholders ----

const phOf = (sp) => {
  const nv = kids(sp).find((k) => k.name.startsWith("nv"));
  const ph = nv ? all(nv, "ph")[0] : null;
  return ph ? { type: ph.attrs.type || "obj", idx: ph.attrs.idx !== undefined ? String(ph.attrs.idx) : undefined } : null;
};
const MASTER_TYPE = { title: "title", ctrTitle: "title", dt: "dt", ftr: "ftr", sldNum: "sldNum", hdr: "hdr" };
// findPlaceholder finds a slide placeholder's counterpart among a layout's
// or master's: by index, then by type.
export function findPlaceholder(list, ph, master = false) {
  if (!ph || !list) return null;
  if (!master && ph.idx !== undefined) {
    const byIdx = list.find((p) => p.ph.idx === ph.idx);
    if (byIdx) return byIdx;
  }
  const want = master ? MASTER_TYPE[ph.type] || "body" : ph.type;
  return list.find((p) => p.ph.type === want)
    || (!master ? list.find((p) => (MASTER_TYPE[p.ph.type] || "body") === (MASTER_TYPE[ph.type] || "body")) : null) || null;
}
const placeholdersOf = (root) => {
  const tree = root ? all(root, "spTree")[0] : null;
  return tree ? all(tree, "sp").map((sp) => ({ sp, ph: phOf(sp) })).filter((p) => p.ph) : [];
};
const styleKind = (ph) => (!ph ? "otherStyle" : ph.type === "title" || ph.type === "ctrTitle" ? "titleStyle"
  : ["dt", "ftr", "sldNum", "hdr"].includes(ph.type) ? "otherStyle" : "bodyStyle");

// ---- the scene ----

const DRAWABLE = /\.(png|jpe?g|gif|bmp|webp|ico)$/i;

// slideScene builds the scene of one slide: its size, background and
// items in drawing order, the pictures they show (parts of the package),
// and what could not be drawn.
export async function slideScene(pkg, deck, slidePart, slideNumber = 1) {
  const slideDoc = rootOf(await pkg.xml(slidePart));
  const slideRels = await pkg.rels(slidePart);
  const layoutPart = [...slideRels.values()].find((r) => isType(r, "slideLayout"))?.part;
  const layoutDoc = layoutPart ? rootOf(await pkg.xml(layoutPart, false)) : null;
  const layoutRels = layoutPart ? await pkg.rels(layoutPart) : new Map();
  const masterPart = [...layoutRels.values()].find((r) => isType(r, "slideMaster"))?.part;
  const masterDoc = masterPart ? rootOf(await pkg.xml(masterPart, false)) : null;
  const masterRels = masterPart ? await pkg.rels(masterPart) : new Map();
  const themePart = [...masterRels.values()].find((r) => isType(r, "theme"))?.part;
  const theme = readTheme(themePart ? await pkg.xml(themePart, false) : null);
  const clrMap = {
    ...DEFAULT_MAP,
    ...(clrMapOf(masterDoc ? child(masterDoc, "clrMap") : null) || {}),
    ...(clrMapOf(layoutDoc ? all(child(layoutDoc, "clrMapOvr") || { children: [] }, "overrideClrMapping")[0] : null) || {}),
    ...(clrMapOf(all(child(slideDoc, "clrMapOvr") || { children: [] }, "overrideClrMapping")[0]) || {}),
  };
  const ctx = { theme, clrMap };
  const presentation = deck.main ? rootOf(await pkg.xml(deck.main, false)) : null;
  const defaultStyle = presentation ? child(presentation, "defaultTextStyle") : null;
  const txStyles = masterDoc ? child(masterDoc, "txStyles") : null;
  const masterPh = placeholdersOf(masterDoc);
  const layoutPh = placeholdersOf(layoutDoc);
  const tableStyles = await readTableStyles(pkg, ctx);
  const missing = new Set();
  const items = [];

  const scene = { width: deck.size.cx, height: deck.size.cy, background: null, items, missing, media: new Set() };
  scene.background = backgroundOf(slideDoc, ctx) || backgroundOf(layoutDoc, ctx) || backgroundOf(masterDoc, ctx) || { type: "solid", color: theme.colors[clrMap.bg1] || hex("FFFFFF") };
  if (scene.background.type === "blip") {
    const owner = slideDoc && child(child(slideDoc, "cSld"), "bg") ? slideRels : layoutDoc && child(child(layoutDoc, "cSld") || { children: [] }, "bg") ? layoutRels : masterRels;
    scene.background.media = owner.get(scene.background.embed)?.part;
    if (scene.background.media) scene.media.add(scene.background.media);
  }

  const showMaster = (doc) => !doc || doc.attrs.showMasterSp === undefined || on(doc.attrs.showMasterSp);
  const layers = [];
  if (masterDoc && showMaster(slideDoc) && showMaster(layoutDoc)) layers.push({ doc: masterDoc, rels: masterRels, level: "master" });
  if (layoutDoc && showMaster(slideDoc)) layers.push({ doc: layoutDoc, rels: layoutRels, level: "layout" });
  layers.push({ doc: slideDoc, rels: slideRels, level: "slide" });

  for (const layer of layers) {
    const tree = all(layer.doc, "spTree")[0];
    if (!tree) continue;
    const env = { pkg, rels: layer.rels, level: layer.level, ctx, defaultStyle, txStyles, masterPh, layoutPh, tableStyles, missing, scene, slideNumber };
    await walk(tree, env, IDENTITY, items);
  }
  return scene;
}

function backgroundOf(doc, ctx) {
  const bg = doc ? child(child(doc, "cSld") || { children: [] }, "bg") : null;
  if (!bg) return null;
  const pr = child(bg, "bgPr");
  if (pr) return fillOf(pr, ctx) || null;
  const ref = child(bg, "bgRef");
  if (ref) return themeFill({ idx: num(ref.attrs.idx), color: colorOf(ref, ctx) }, ctx) || null;
  return null;
}

// walk adds the items of a shape tree, or a group, under transform m.
async function walk(tree, env, m, out, groupFill = null) {
  for (let node of kids(tree)) {
    if (node.name === "AlternateContent") {
      const pick = child(node, "Fallback") || child(node, "Choice");
      if (pick) await walk(pick, env, m, out, groupFill);
      continue;
    }
    const nv = kids(node).find((k) => k.name.startsWith("nv"));
    const cNvPr = nv ? child(nv, "cNvPr") : null;
    if (cNvPr && on(cNvPr.attrs.hidden)) continue;
    switch (node.name) {
      case "sp": case "cxnSp": await shapeItem(node, env, m, out, groupFill); break;
      case "pic": await pictureItem(node, env, m, out); break;
      case "grpSp": {
        const pr = child(node, "grpSpPr");
        const t = xfrmOf(child(pr, "xfrm"));
        if (!t) { await walk(node, env, m, out, groupFill); break; }
        const sx = t.chW ? t.w / t.chW : 1, sy = t.chH ? t.h / t.chH : 1;
        const gm = mul(mul(m, boxMatrix(t)), mul(scale(sx, sy), translate(-t.chX, -t.chY)));
        const gf = fillOf(pr, env.ctx);
        await walk(node, env, gm, out, gf && gf.type !== "group" ? gf : groupFill);
        break;
      }
      case "graphicFrame": await frameItem(node, env, m, out); break;
      case "contentPart": env.missing.add("ink"); break;
      default:
    }
  }
}

async function shapeItem(sp, env, m, out, groupFill) {
  const ph = phOf(sp);
  // Placeholders of a master or layout are templates, not drawn on slides.
  if (ph && env.level !== "slide") return;
  const layoutMatch = ph ? findPlaceholder(env.layoutPh, ph) : null;
  const masterMatch = ph ? findPlaceholder(env.masterPh, layoutMatch?.ph || ph, true) : null;
  const chain = [masterMatch?.sp, layoutMatch?.sp, sp].filter(Boolean);
  const prOf = (s) => child(s, "spPr");
  let t = null;
  for (const s of chain) t = xfrmOf(child(prOf(s) || { children: [] }, "xfrm")) || t;
  // A placeholder placed nowhere, as files without master or layout have
  // them, stands where a default layout puts its kind.
  if (!t && ph) t = defaultBox(ph, env.scene.width, env.scene.height);
  if (!t) return;
  const ctx = env.ctx;
  let style = {};
  for (const s of chain) if (child(s, "style")) style = styleRefs(child(s, "style"), ctx);
  let fill, line, geom;
  for (const s of chain) {
    const pr = prOf(s);
    fill = fillOf(pr, ctx) ?? fill;
    line = mergeLine(line, lineOf(pr ? child(pr, "ln") : null, ctx));
    geom = geometryOf(pr) || geom;
  }
  if (fill === undefined) fill = themeFill(style.fill, ctx) || { type: "none" };
  if (fill.type === "group") fill = groupFill || { type: "none" };
  if (fill.type === "blip") fill = mediaFill(fill, env);
  line = mergeLine(themeLine(style.line, ctx), line);
  const kind = styleKind(ph);
  const styles = [env.defaultStyle, env.txStyles ? child(env.txStyles, kind) : null,
    ...chain.slice(0, -1).map((s) => child(child(s, "txBody") || { children: [] }, "lstStyle"))];
  const bodyPrs = chain.slice(0, -1).map((s) => child(child(s, "txBody") || { children: [] }, "bodyPr"));
  const fontColor = style.font?.color || null;
  const fontFace = style.font ? (style.font.font === "major" ? ctx.theme.major : style.font.font === "minor" ? ctx.theme.minor : undefined) : undefined;
  const text = textOf(child(sp, "txBody"), styles, bodyPrs, ctx, { color: fontColor, font: fontFace, slideNumber: env.slideNumber });
  const txX = xfrmOf(child(sp, "txXfrm"));
  const g = geom || { preset: "rect", adj: {} };
  if (!g.custom && !KNOWN.has(g.preset)) env.missing.add("shape");
  out.push({
    kind: "shape", m: mul(m, boxMatrix(t)), tm: mul(m, boxMatrix(t, false)), w: t.w, h: t.h, geom: g, fill, line,
    text: text && text.paragraphs.some((p) => p.runs.some((r) => r.text.trim())) ? text : null,
    textBox: txX ? { x: txX.x - t.x, y: txX.y - t.y, w: txX.w, h: txX.h } : textRect(g, t.w, t.h),
  });
}

// defaultBox is where a title, subtitle, footer or body placeholder stands
// on a slide of w × h without a layout to say so.
export function defaultBox(ph, w, h) {
  const box = (x, y, bw, bh) => ({ x: x * w, y: y * h, w: bw * w, h: bh * h, rot: 0, flipH: false, flipV: false });
  switch (ph.type) {
    case "title": return box(0.07, 0.04, 0.86, 0.17);
    case "ctrTitle": return box(0.07, 0.31, 0.86, 0.21);
    case "subTitle": return box(0.15, 0.56, 0.7, 0.25);
    case "dt": return box(0.07, 0.93, 0.21, 0.05);
    case "ftr": return box(0.34, 0.93, 0.32, 0.05);
    case "sldNum": return box(0.72, 0.93, 0.21, 0.05);
    default: return box(0.07, 0.25, 0.86, 0.65);
  }
}

function mergeLine(a, b) {
  if (!a) return b;
  if (!b) return a;
  return { ...a, ...b };
}

function mediaFill(fill, env) {
  const part = env.rels.get(fill.embed)?.part;
  if (!part || !DRAWABLE.test(part)) { env.missing.add("picture"); return { type: "none" }; }
  env.scene.media.add(part);
  return { ...fill, media: part };
}

async function pictureItem(pic, env, m, out) {
  const pr = child(pic, "spPr");
  const t = xfrmOf(child(pr || { children: [] }, "xfrm"));
  if (!t) return;
  const bf = child(pic, "blipFill");
  const blip = bf ? blipOf(bf) : null;
  const part = blip?.embed ? env.rels.get(blip.embed)?.part : null;
  if (!part || !DRAWABLE.test(part)) {
    env.missing.add("picture");
    out.push({ kind: "missing", what: "picture", m: mul(m, boxMatrix(t)), w: t.w, h: t.h });
    return;
  }
  env.scene.media.add(part);
  out.push({
    kind: "picture", m: mul(m, boxMatrix(t)), w: t.w, h: t.h, media: part, crop: blip.crop, alpha: blip.alpha,
    geom: geometryOf(pr) || { preset: "rect", adj: {} }, line: lineOf(pr ? child(pr, "ln") : null, env.ctx),
  });
}

async function frameItem(frame, env, m, out) {
  const t = xfrmOf(child(frame, "xfrm"));
  if (!t) return;
  const data = all(frame, "graphicData")[0];
  const uri = data?.attrs.uri || "";
  const fm = mul(m, boxMatrix(t));
  if (uri.endsWith("/table")) {
    const tbl = child(data, "tbl");
    if (tbl) out.push(tableItem(tbl, env, fm, t));
    return;
  }
  if (uri.endsWith("/diagram")) {
    const ids = child(data, "relIds");
    const dataPart = ids ? env.rels.get(attrOf(ids, "dm", true))?.part : null;
    const dataDoc = dataPart ? await env.pkg.xml(dataPart, false) : null;
    const ext = dataDoc ? all(rootOf(dataDoc), "dataModelExt")[0] : null;
    const drawingPart = ext ? env.rels.get(ext.attrs.relId)?.part : null;
    const drawing = drawingPart ? await env.pkg.xml(drawingPart, false) : null;
    const tree = drawing ? all(rootOf(drawing), "spTree")[0] : null;
    if (!tree) { env.missing.add("diagram"); out.push({ kind: "missing", what: "diagram", m: fm, w: t.w, h: t.h }); return; }
    // The drawing's shapes stand in the frame's coordinates, its
    // pictures in the drawing's own relationships.
    const denv = { ...env, rels: await env.pkg.rels(drawingPart), level: "slide", layoutPh: [], masterPh: [] };
    await walk(tree, denv, mul(m, translate(t.x, t.y)), out);
    return;
  }
  if (uri.endsWith("/chart")) {
    const ref = child(data, "chart");
    const part = ref ? env.rels.get(attrOf(ref, "id", true))?.part : null;
    const doc = part ? await env.pkg.xml(part, false) : null;
    const chart = doc ? readChart(doc, env.ctx, { colorOf, fillOf, lineOf }) : null;
    if (chart) { out.push({ kind: "chart", m: fm, w: t.w, h: t.h, chart }); return; }
  }
  const fallback = all(data || { children: [] }, "pic")[0];
  if (fallback) {
    const inner = xfrmOf(child(child(fallback, "spPr") || { children: [] }, "xfrm"));
    const pic = inner && inner.w ? fallback : { ...fallback, children: fallback.children.map((c) => (c.name === "spPr"
      ? { ...c, children: [{ name: "xfrm", attrs: {}, children: [{ name: "off", attrs: { x: t.x, y: t.y }, children: [] }, { name: "ext", attrs: { cx: t.w, cy: t.h }, children: [] }] }] } : c)) };
    await pictureItem(pic, env, m, out);
    return;
  }
  const what = uri.includes("chart") ? "chart" : "object";
  env.missing.add(what);
  out.push({ kind: "missing", what, m: fm, w: t.w, h: t.h });
}

// ---- tables ----

const MEDIUM_2 = "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}";

// readTableStyles reads the table styles a presentation keeps, by ID, each
// part as {fill, border, bold, color}; the default style of PowerPoint
// stands in when the presentation does not keep it.
async function readTableStyles(pkg, ctx) {
  const out = new Map();
  const doc = await pkg.xml("ppt/tableStyles.xml", false);
  for (const s of doc ? kids(rootOf(doc), "tblStyle") : []) {
    const parts = {};
    for (const part of kids(s)) {
      const tc = child(part, "tcStyle");
      const tx = child(part, "tcTxStyle");
      const fillEl = tc ? child(tc, "fill") : null;
      const fillRef = tc ? child(tc, "fillRef") : null;
      const borders = {};
      const bdr = tc ? child(tc, "tcBdr") : null;
      for (const b of bdr ? kids(bdr) : []) {
        const ln = child(b, "ln");
        const lnRef = child(b, "lnRef");
        borders[b.name] = ln ? lineOf(ln, ctx) : lnRef ? themeLine({ idx: num(lnRef.attrs.idx), color: colorOf(lnRef, ctx) }, ctx) : undefined;
      }
      parts[part.name] = {
        fill: fillEl ? fillOf(fillEl, ctx) : fillRef ? themeFill({ idx: num(fillRef.attrs.idx), color: colorOf(fillRef, ctx) }, ctx) : undefined,
        borders, bold: tx ? on(tx.attrs.b) : undefined, color: tx ? colorOf(tx, ctx) || colorOf(child(tx, "fontRef"), ctx) : undefined,
      };
    }
    out.set(s.attrs.styleId, parts);
  }
  if (!out.has(MEDIUM_2)) {
    const accent = ctx.theme.colors.accent1 || hex("4472C4");
    const lt1 = ctx.theme.colors[ctx.clrMap.bg1] || hex("FFFFFF");
    const white = { width: 12700, color: lt1 };
    const tintOf = (v) => ({ ...mixWith(accent, 1, v), a: 1 });
    out.set(MEDIUM_2, {
      wholeTbl: { fill: { type: "solid", color: tintOf(0.2) }, borders: { left: white, right: white, top: white, bottom: white, insideH: white, insideV: white } },
      band1H: { fill: { type: "solid", color: tintOf(0.4) }, borders: {} },
      firstRow: { fill: { type: "solid", color: accent }, borders: { bottom: { width: 38100, color: lt1 } }, bold: true, color: lt1 },
      lastRow: { fill: { type: "solid", color: accent }, borders: { top: { width: 38100, color: lt1 } }, bold: true, color: lt1 },
      firstCol: { fill: { type: "solid", color: accent }, borders: {}, bold: true, color: lt1 },
      lastCol: { fill: { type: "solid", color: accent }, borders: {}, bold: true, color: lt1 },
    });
  }
  return out;
}

function tableItem(tbl, env, m, t) {
  const ctx = env.ctx;
  const pr = child(tbl, "tblPr");
  const flags = pr ? pr.attrs : {};
  const styleId = pr ? child(pr, "tableStyleId")?.children.join("").trim() : undefined;
  const style = env.tableStyles.get(styleId || (pr && pr.attrs ? MEDIUM_2 : undefined)) || {};
  const cols = kids(child(tbl, "tblGrid") || { children: [] }, "gridCol").map((c) => num(c.attrs.w));
  const rows = kids(tbl, "tr");
  const styles = [env.defaultStyle, env.txStyles ? child(env.txStyles, "otherStyle") : null];
  const outRows = rows.map((tr, ri) => {
    const cells = kids(tr, "tc").map((tc, ci) => {
      const parts = ["wholeTbl"];
      const dataRow = ri - (on(flags.firstRow) ? 1 : 0);
      if (on(flags.bandRow) && dataRow >= 0) parts.push(dataRow % 2 === 0 ? "band1H" : "band2H");
      if (on(flags.bandCol)) parts.push(ci % 2 === 0 ? "band1V" : "band2V");
      if (on(flags.firstCol) && ci === 0) parts.push("firstCol");
      if (on(flags.lastCol) && ci === cols.length - 1) parts.push("lastCol");
      if (on(flags.lastRow) && ri === rows.length - 1) parts.push("lastRow");
      if (on(flags.firstRow) && ri === 0) parts.push("firstRow");
      let fill, bold, color;
      const borders = {};
      for (const p of parts) {
        const s = style[p];
        if (!s) continue;
        fill = s.fill ?? fill;
        if (s.bold !== undefined) bold = s.bold;
        if (s.color) color = s.color;
        Object.assign(borders, Object.fromEntries(Object.entries(s.borders || {}).filter(([, v]) => v)));
      }
      const tcPr = child(tc, "tcPr");
      const own = fillOf(tcPr, ctx);
      if (own) fill = own;
      const side = (name, outer, inner, first, last) => {
        const ln = tcPr ? lineOf(child(tcPr, name), ctx) : undefined;
        return ln || (first || last ? borders[outer] : borders[inner]) || borders[outer];
      };
      const text = textOf(child(tc, "txBody"), styles, [], ctx, { color });
      if (bold && text) for (const p of text.paragraphs) for (const r of p.runs) r.bold = true;
      return {
        span: num(tc.attrs.gridSpan, 1), rowSpan: num(tc.attrs.rowSpan, 1), merged: on(tc.attrs.hMerge) || on(tc.attrs.vMerge),
        fill: fill || { type: "none" }, text,
        margins: { l: num(tcPr?.attrs.marL, 91440), r: num(tcPr?.attrs.marR, 91440), t: num(tcPr?.attrs.marT, 45720), b: num(tcPr?.attrs.marB, 45720) },
        anchor: tcPr?.attrs.anchor || "t",
        borders: {
          l: side("lnL", "left", "insideV", ci === 0, false), r: side("lnR", "right", "insideV", false, ci === cols.length - 1),
          t: side("lnT", "top", "insideH", ri === 0, false), b: side("lnB", "bottom", "insideH", false, ri === rows.length - 1),
        },
      };
    });
    return { h: num(tr.attrs.h), cells };
  });
  return { kind: "table", m, w: t.w, h: t.h, cols, rows: outRows };
}

// ---- geometry ----

function geometryOf(pr) {
  if (!pr) return null;
  const prst = child(pr, "prstGeom");
  if (prst) {
    const adj = {};
    for (const gd of all(child(prst, "avLst") || { children: [] }, "gd")) {
      const m = /val\s+(-?\d+)/.exec(gd.attrs.fmla || "");
      if (m) adj[gd.attrs.name] = Number(m[1]);
    }
    return { preset: prst.attrs.prst || "rect", adj };
  }
  const cust = child(pr, "custGeom");
  if (cust) {
    const paths = all(child(cust, "pathLst") || { children: [] }, "path").map((p) => ({
      w: num(p.attrs.w), h: num(p.attrs.h), fill: p.attrs.fill !== "none", stroke: p.attrs.stroke !== "0" && p.attrs.stroke !== "false",
      cmds: kids(p).map((c) => ({ op: c.name, pts: kids(c, "pt").map((pt) => [pt.attrs.x, pt.attrs.y]), ...c.attrs })),
    }));
    return { custom: true, paths };
  }
  return null;
}

// KNOWN are the preset shapes geometry() draws as themselves; others are
// drawn as rectangles and counted as approximated.
export const KNOWN = new Set(["rect", "roundRect", "ellipse", "triangle", "rtTriangle", "diamond", "parallelogram", "trapezoid",
  "pentagon", "hexagon", "octagon", "chevron", "homePlate", "rightArrow", "leftArrow", "upArrow", "downArrow", "line",
  "straightConnector1", "bentConnector2", "bentConnector3", "curvedConnector3", "snip1Rect", "round1Rect", "round2SameRect",
  "plus", "star5", "flowChartProcess", "flowChartAlternateProcess", "flowChartDecision", "flowChartTerminator", "donut",
  "flowChartConnector", "teardrop", "frame", "halfFrame", "cube", "can", "leftRightArrow", "notchedRightArrow", "wedgeRectCallout",
  "wedgeRoundRectCallout", "rect"]);

const P = (x, y) => ({ op: "M", x, y });
const L = (x, y) => ({ op: "L", x, y });
const Z = { op: "Z" };
const poly = (pts) => [P(...pts[0]), ...pts.slice(1).map((p) => L(...p)), Z];

// geometry gives the outline of a shape of size w × h as path commands —
// M, L, C (cubic), E (ellipse arc), Z — and whether it is only a line.
export function geometry(g, w, h) {
  if (g.custom) return { paths: g.paths.map((p) => customPath(p, w, h)) };
  const a = (name, d) => (g.adj[name] ?? d) / 100000;
  const ss = Math.min(w, h);
  const one = (cmds, line = false) => ({ paths: [{ cmds, fill: !line, stroke: true }] });
  switch (g.preset) {
    case "line": case "straightConnector1": return one([P(0, 0), L(w, h)], true);
    case "bentConnector2": return one([P(0, 0), L(w, 0), L(w, h)], true);
    case "bentConnector3": { const x = w * a("adj1", 50000); return one([P(0, 0), L(x, 0), L(x, h), L(w, h)], true); }
    case "curvedConnector3": { const x = w * a("adj1", 50000); return one([P(0, 0), { op: "C", x1: x, y1: 0, x2: x, y2: h, x, y: h / 2 }, { op: "C", x1: x, y1: h, x2: x, y2: h, x: w, y: h }], true); }
    case "ellipse": case "flowChartConnector": return one([{ op: "E", cx: w / 2, cy: h / 2, rx: w / 2, ry: h / 2, a0: 0, a1: Math.PI * 2 }, Z]);
    case "donut": {
      const d = ss * a("adj", 25000);
      return { paths: [{ cmds: [{ op: "E", cx: w / 2, cy: h / 2, rx: w / 2, ry: h / 2, a0: 0, a1: Math.PI * 2 }, Z,
        { op: "E", cx: w / 2, cy: h / 2, rx: Math.max(0, w / 2 - d), ry: Math.max(0, h / 2 - d), a0: Math.PI * 2, a1: 0, ccw: true }, Z], fill: true, stroke: true, evenodd: true }] };
    }
    case "roundRect": case "flowChartAlternateProcess": case "wedgeRoundRectCallout": return one(rounded(w, h, ss * a("adj", 16667), [1, 1, 1, 1]));
    case "round1Rect": return one(rounded(w, h, ss * a("adj", 16667), [0, 1, 0, 0]));
    case "round2SameRect": return one(rounded(w, h, ss * a("adj1", 16667), [1, 1, 0, 0]));
    case "flowChartTerminator": return one(rounded(w, h, ss / 2, [1, 1, 1, 1]));
    case "triangle": { const x = w * a("adj", 50000); return one(poly([[x, 0], [w, h], [0, h]])); }
    case "rtTriangle": return one(poly([[0, 0], [w, h], [0, h]]));
    case "diamond": case "flowChartDecision": return one(poly([[w / 2, 0], [w, h / 2], [w / 2, h], [0, h / 2]]));
    case "parallelogram": { const x = ss * a("adj", 25000); return one(poly([[x, 0], [w, 0], [w - x, h], [0, h]])); }
    case "trapezoid": { const x = ss * a("adj", 25000); return one(poly([[x, 0], [w - x, 0], [w, h], [0, h]])); }
    case "pentagon": return one(poly([[w / 2, 0], [w, h * 0.38], [w * 0.81, h], [w * 0.19, h], [0, h * 0.38]]));
    case "hexagon": { const x = ss * a("adj", 25000); return one(poly([[x, 0], [w - x, 0], [w, h / 2], [w - x, h], [x, h], [0, h / 2]])); }
    case "octagon": { const x = ss * a("adj", 29289); return one(poly([[x, 0], [w - x, 0], [w, x], [w, h - x], [w - x, h], [x, h], [0, h - x], [0, x]])); }
    case "chevron": { const x = ss * a("adj", 50000); return one(poly([[0, 0], [w - x, 0], [w, h / 2], [w - x, h], [0, h], [x, h / 2]])); }
    case "homePlate": { const x = ss * a("adj", 50000); return one(poly([[0, 0], [w - x, 0], [w, h / 2], [w - x, h], [0, h]])); }
    case "rightArrow": case "notchedRightArrow": { const t = h * a("adj1", 50000) / 2, x = w - ss * a("adj2", 50000); return one(poly([[0, h / 2 - t], [x, h / 2 - t], [x, 0], [w, h / 2], [x, h], [x, h / 2 + t], [0, h / 2 + t]])); }
    case "leftArrow": { const t = h * a("adj1", 50000) / 2, x = ss * a("adj2", 50000); return one(poly([[w, h / 2 - t], [x, h / 2 - t], [x, 0], [0, h / 2], [x, h], [x, h / 2 + t], [w, h / 2 + t]])); }
    case "upArrow": { const t = w * a("adj1", 50000) / 2, y = ss * a("adj2", 50000); return one(poly([[w / 2 - t, h], [w / 2 - t, y], [0, y], [w / 2, 0], [w, y], [w / 2 + t, y], [w / 2 + t, h]])); }
    case "downArrow": { const t = w * a("adj1", 50000) / 2, y = h - ss * a("adj2", 50000); return one(poly([[w / 2 - t, 0], [w / 2 - t, y], [0, y], [w / 2, h], [w, y], [w / 2 + t, y], [w / 2 + t, 0]])); }
    case "leftRightArrow": { const t = h * a("adj1", 50000) / 2, x = ss * a("adj2", 50000); return one(poly([[0, h / 2], [x, 0], [x, h / 2 - t], [w - x, h / 2 - t], [w - x, 0], [w, h / 2], [w - x, h], [w - x, h / 2 + t], [x, h / 2 + t], [x, h]])); }
    case "snip1Rect": { const x = ss * a("adj", 16667); return one(poly([[0, 0], [w - x, 0], [w, x], [w, h], [0, h]])); }
    case "plus": { const x = ss * a("adj", 25000); return one(poly([[x, 0], [w - x, 0], [w - x, x], [w, x], [w, h - x], [w - x, h - x], [w - x, h], [x, h], [x, h - x], [0, h - x], [0, x], [x, x]])); }
    case "star5": {
      const pts = [];
      for (let i = 0; i < 10; i++) {
        const r = i % 2 ? 0.38 : 1, ang = -Math.PI / 2 + (i * Math.PI) / 5;
        pts.push([w / 2 + (w / 2) * r * Math.cos(ang), h / 2 + (h / 2) * r * Math.sin(ang) * 1.05 + h * 0.025]);
      }
      return one(poly(pts));
    }
    case "teardrop": return one(rounded(w, h, ss / 2, [1, 0, 1, 1]));
    case "frame": case "halfFrame": {
      const d = ss * a("adj1", 12500);
      return { paths: [{ cmds: [...poly([[0, 0], [w, 0], [w, h], [0, h]]), ...poly([[d, d], [d, h - d], [w - d, h - d], [w - d, d]])], fill: true, stroke: true, evenodd: true }] };
    }
    case "cube": { const d = ss * a("adj", 25000); return { paths: [{ cmds: poly([[0, d], [d, 0], [w, 0], [w, h - d], [w - d, h], [0, h]]), fill: true, stroke: true }, { cmds: [P(0, d), L(w - d, d), L(w, 0), P(w - d, d), L(w - d, h)], fill: false, stroke: true }] }; }
    case "can": { const d = ss * a("adj", 25000) / 2; return { paths: [{ cmds: [P(0, d), L(0, h - d), { op: "E", cx: w / 2, cy: h - d, rx: w / 2, ry: d, a0: Math.PI, a1: 0, ccw: true }, L(w, d), { op: "E", cx: w / 2, cy: d, rx: w / 2, ry: d, a0: 0, a1: Math.PI, ccw: true }, Z], fill: true, stroke: true }, { cmds: [{ op: "E", cx: w / 2, cy: d, rx: w / 2, ry: d, a0: Math.PI, a1: 0, ccw: true }], fill: false, stroke: true }] }; }
    default: return one(poly([[0, 0], [w, 0], [w, h], [0, h]]));
  }
}

// textRect is the part of a shape of size w × h its text is laid out in,
// as the presets define it: inside an ellipse's inscribed rectangle, a
// triangle's lower half, an arrow's shaft. Others take their whole box.
export function textRect(g, w, h) {
  const a = (name, d) => ((g.adj || {})[name] ?? d) / 100000;
  const ss = Math.min(w, h);
  const r = (l, t, rr, b) => ({ x: l, y: t, w: Math.max(0, rr - l), h: Math.max(0, b - t) });
  switch (g.custom ? "" : g.preset) {
    case "ellipse": case "flowChartConnector": return r(w * 0.1464, h * 0.1464, w * 0.8536, h * 0.8536);
    case "roundRect": case "flowChartAlternateProcess": { const i = Math.min(ss * a("adj", 16667), w / 2, h / 2) * 0.2929; return r(i, i, w - i, h - i); }
    case "flowChartTerminator": { const i = (ss / 2) * 0.2929; return r(i, 0, w - i, h); }
    case "triangle": { const x = w * a("adj", 50000); return r(x / 2, h / 2, (x + w) / 2, h); }
    case "rtTriangle": return r(w / 12, h * 7 / 12, w * 7 / 12, h * 11 / 12);
    case "diamond": case "flowChartDecision": return r(w / 4, h / 4, w * 3 / 4, h * 3 / 4);
    case "chevron": { const x = ss * a("adj", 50000); return r(x, 0, w, h); }
    case "homePlate": { const x = ss * a("adj", 50000); return r(0, 0, w - x / 2, h); }
    case "rightArrow": case "notchedRightArrow": { const t = h * a("adj1", 50000) / 2; return r(0, h / 2 - t, w - ss * a("adj2", 50000) / 2, h / 2 + t); }
    case "leftArrow": { const t = h * a("adj1", 50000) / 2; return r(ss * a("adj2", 50000) / 2, h / 2 - t, w, h / 2 + t); }
    case "leftRightArrow": { const t = h * a("adj1", 50000) / 2, x = ss * a("adj2", 50000) / 2; return r(x, h / 2 - t, w - x, h / 2 + t); }
    case "upArrow": { const t = w * a("adj1", 50000) / 2; return r(w / 2 - t, ss * a("adj2", 50000) / 2, w / 2 + t, h); }
    case "downArrow": { const t = w * a("adj1", 50000) / 2; return r(w / 2 - t, 0, w / 2 + t, h - ss * a("adj2", 50000) / 2); }
    case "hexagon": { const x = ss * a("adj", 25000) / 2; return r(x, 0, w - x, h); }
    case "octagon": { const x = ss * a("adj", 29289) / 2; return r(x, x, w - x, h - x); }
    case "parallelogram": case "trapezoid": { const x = ss * a("adj", 25000) / 2; return r(x, 0, w - x, h); }
    case "pentagon": return r(w * 0.2, h * 0.38, w * 0.8, h);
    case "plus": { const x = ss * a("adj", 25000); return r(0, x, w, h - x); }
    case "star5": return r(w * 0.31, h * 0.38, w * 0.69, h * 0.75);
    case "donut": return r(w * 0.1464, h * 0.1464, w * 0.8536, h * 0.8536);
    default: return r(0, 0, w, h);
  }
}

// rounded is a rectangle with rounded corners: top left, top right, bottom
// right, bottom left, each rounded when its flag is 1.
function rounded(w, h, r0, corners) {
  const r = Math.min(r0, w / 2, h / 2);
  const [tl, tr, br, bl] = corners.map((c) => (c ? r : 0));
  const cmds = [P(tl, 0), L(w - tr, 0)];
  if (tr) cmds.push({ op: "E", cx: w - tr, cy: tr, rx: tr, ry: tr, a0: -Math.PI / 2, a1: 0 });
  cmds.push(L(w, h - br));
  if (br) cmds.push({ op: "E", cx: w - br, cy: h - br, rx: br, ry: br, a0: 0, a1: Math.PI / 2 });
  cmds.push(L(bl, h));
  if (bl) cmds.push({ op: "E", cx: bl, cy: h - bl, rx: bl, ry: bl, a0: Math.PI / 2, a1: Math.PI });
  cmds.push(L(0, tl));
  if (tl) cmds.push({ op: "E", cx: tl, cy: tl, rx: tl, ry: tl, a0: Math.PI, a1: Math.PI * 1.5 });
  cmds.push(Z);
  return cmds;
}

// customPath scales a freeform path to the shape: its points are in the
// path's own w × h; names such as r or b stand for its edges.
function customPath(p, w, h) {
  const sx = p.w ? w / p.w : 1, sy = p.h ? h / p.h : 1;
  const guide = { l: 0, t: 0, r: p.w || w, b: p.h || h, w: p.w || w, h: p.h || h, hc: (p.w || w) / 2, vc: (p.h || h) / 2 };
  const v = (s) => (s in guide ? guide[s] : num(s));
  const cmds = [];
  let cur = [0, 0];
  for (const c of p.cmds) {
    const pts = c.pts.map(([x, y]) => [v(x) * sx, v(y) * sy]);
    switch (c.op) {
      case "moveTo": cmds.push(P(...pts[0])); cur = pts[0]; break;
      case "lnTo": cmds.push(L(...pts[0])); cur = pts[0]; break;
      case "cubicBezTo": cmds.push({ op: "C", x1: pts[0][0], y1: pts[0][1], x2: pts[1][0], y2: pts[1][1], x: pts[2][0], y: pts[2][1] }); cur = pts[2]; break;
      case "quadBezTo": cmds.push({ op: "Q", x1: pts[0][0], y1: pts[0][1], x: pts[1][0], y: pts[1][1] }); cur = pts[1]; break;
      case "arcTo": {
        const rx = v(c.wR) * sx, ry = v(c.hR) * sy;
        const st = (num(c.stAng) / 60000) * (Math.PI / 180), sw = (num(c.swAng) / 60000) * (Math.PI / 180);
        const cx = cur[0] - rx * Math.cos(st), cy = cur[1] - ry * Math.sin(st);
        cmds.push({ op: "E", cx, cy, rx, ry, a0: st, a1: st + sw, ccw: sw < 0 });
        cur = [cx + rx * Math.cos(st + sw), cy + ry * Math.sin(st + sw)];
        break;
      }
      case "close": cmds.push(Z); break;
      default:
    }
  }
  return { cmds, fill: p.fill, stroke: p.stroke };
}
