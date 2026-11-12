// pptxchart reads a chart of a presentation (ADR-0186) from the values the
// presentation keeps for it — every chart part caches its categories,
// series names and numbers beside the reference into its workbook — and
// gives the scales and number formats it is drawn with. Column, bar,
// line, area, pie, doughnut and scatter charts are read, their 3-D forms
// as flat ones; other kinds (radar, bubble, stock, surface) are not, and
// the slide shows them as grey boxes. assets/pptxdraw.js draws the result.
// It needs no browser, so node --test runs it.
import { all, child, kids } from "./pptx.js";

const num = (v, d = 0) => (v === undefined || v === null || v === "" || Number.isNaN(Number(v)) ? d : Number(v));
const val = (node, name, d) => {
  const el = node ? child(node, name) : null;
  return el ? el.attrs.val ?? d : d;
};
const isOn = (node, name, d = false) => {
  const v = val(node, name, undefined);
  return v === undefined ? d : v === "1" || v === "true";
};

// KINDS maps the plot elements read to the kind drawn.
const KINDS = {
  barChart: "bar", bar3DChart: "bar", lineChart: "line", line3DChart: "line", areaChart: "area", area3DChart: "area",
  pieChart: "pie", pie3DChart: "pie", doughnutChart: "doughnut", scatterChart: "scatter",
};

// cacheOf reads the points a reference or literal keeps: an array by
// index, with its number format.
function cacheOf(node) {
  if (!node) return { points: [], format: "" };
  const cache = all(node, "strCache")[0] || all(node, "numCache")[0] || child(node, "strLit") || child(node, "numLit");
  if (!cache) {
    // Multi-level categories keep their levels; the innermost names them.
    const lvl = all(node, "lvl")[0];
    return lvl ? { points: kids(lvl, "pt").map((p) => child(p, "v")?.children.join("") ?? ""), format: "" } : { points: [], format: "" };
  }
  const count = num(val(cache, "ptCount", 0));
  const points = new Array(count).fill(null);
  for (const p of kids(cache, "pt")) {
    const i = num(p.attrs.idx);
    if (i >= points.length) points.length = i + 1;
    points[i] = child(p, "v")?.children.filter((c) => typeof c === "string").join("") ?? null;
  }
  return { points, format: child(cache, "formatCode")?.children.join("") || "" };
}

const richText = (rich) => (rich ? all(rich, "p").map((p) => all(p, "t").map((t) => t.children.join("")).join("")).join("\n").trim() : "");

// textStyle reads the size, weight and colour a text properties element
// (c:txPr) sets for its first paragraph.
function textStyle(txPr, helpers, ctx, base) {
  const def = txPr ? all(txPr, "defRPr")[0] : null;
  if (!def) return base;
  const out = { ...base };
  if (def.attrs.sz) out.size = num(def.attrs.sz) / 100;
  if (def.attrs.b !== undefined) out.bold = def.attrs.b === "1";
  const fill = child(def, "solidFill");
  if (fill) out.color = helpers.colorOf(fill, ctx);
  const latin = child(def, "latin");
  if (latin?.attrs.typeface) out.font = latin.attrs.typeface;
  return out;
}

// readChart reads a chart part (c:chartSpace) into what is drawn: title,
// legend, the text's size and colour, each plot with its series and their
// values, and the axes. helpers are pptxscene's colorOf, fillOf and
// lineOf; ctx its theme and colour map. It gives null when the chart holds
// no kind it reads.
export function readChart(doc, ctx, helpers) {
  const space = doc ? kids(doc)[0] : null;
  const chart = space ? child(space, "chart") : null;
  const area = chart ? child(chart, "plotArea") : null;
  if (!area) return null;
  const tx1 = ctx.theme.colors[ctx.clrMap?.tx1 || "dk1"];
  const font = textStyle(child(space, "txPr"), helpers, ctx, { size: 10, bold: false, color: { ...tx1, a: 0.75 }, font: ctx.theme.minor });
  const accent = (i) => {
    const base = ctx.theme.colors[`accent${(i % 6) + 1}`] || { r: 0.27, g: 0.45, b: 0.77, a: 1 };
    // Past six series the accents come again, darker.
    const f = i >= 6 ? 0.6 : 1;
    return { r: base.r * f, g: base.g * f, b: base.b * f, a: 1 };
  };
  const plots = [];
  let serials = 0;
  for (const p of kids(area)) {
    const kind = KINDS[p.name];
    if (!kind) continue;
    const grouping = val(p, "grouping", kind === "bar" ? "clustered" : "standard");
    const vary = isOn(p, "varyColors", kind === "pie" || kind === "doughnut");
    const plotLabels = labelsOf(child(p, "dLbls"));
    const series = kids(p, "ser").map((s) => {
      const idx = num(val(s, "idx", serials));
      serials++;
      const pr = child(s, "spPr");
      const fill = helpers.fillOf(pr, ctx);
      const line = helpers.lineOf(pr ? child(pr, "ln") : null, ctx);
      const name = (() => {
        const tx = child(s, "tx");
        if (!tx) return `Series ${idx + 1}`;
        const v = child(tx, "v");
        return v ? v.children.join("") : cacheOf(tx).points[0] ?? "";
      })();
      const cats = cacheOf(child(s, "cat") || child(s, "xVal"));
      const vals = cacheOf(child(s, "val") || child(s, "yVal"));
      const markerSym = val(child(s, "marker"), "symbol", undefined);
      const points = kids(s, "dPt").map((d) => ({ idx: num(val(d, "idx", 0)), fill: helpers.fillOf(child(d, "spPr"), ctx) })).filter((d) => d.fill);
      const color = fill?.type === "solid" ? fill.color : fill?.type === "grad" ? fill.stops[0]?.color : line?.color || accent(idx);
      // Each point's colour: its own, else the next accent where the
      // plot varies them (a pie's slices), else the series'.
      const count = Math.max(cats.points.length, vals.points.length);
      const colors = Array.from({ length: count }, (_, i) => {
        const own = points.find((d) => d.idx === i)?.fill;
        if (own?.type === "solid") return own.color;
        return vary ? accent(i) : color;
      });
      return {
        name, idx, color, colors, noFill: fill?.type === "none", line,
        categories: cats.points.map((c) => c ?? ""), values: vals.points.map((v) => (v === null ? null : num(v, null))),
        format: vals.format, xNumeric: kind === "scatter" && cats.points.every((c) => c === null || !Number.isNaN(Number(c))),
        marker: markerSym === undefined ? undefined : markerSym !== "none", smooth: isOn(s, "smooth"),
        points, labels: labelsOf(child(s, "dLbls")) || plotLabels,
      };
    });
    if (!series.length) continue;
    plots.push({
      kind, dir: val(p, "barDir", "col"), grouping, vary, series, labels: plotLabels,
      gap: num(val(p, "gapWidth", 150)), overlap: num(val(p, "overlap", grouping === "clustered" ? 0 : 100)),
      hole: num(val(p, "holeSize", 50)), firstAngle: num(val(p, "firstSliceAng", 0)),
      markers: isOn(p, "marker", kind === "line"), scatterStyle: val(p, "scatterStyle", "marker"),
    });
  }
  if (!plots.length) return null;
  const axis = (el) => el && {
    deleted: isOn(el, "delete"), pos: val(el, "axPos", "b"), gridlines: Boolean(child(el, "majorGridlines")),
    min: child(child(el, "scaling") || { children: [] }, "min") ? num(val(child(el, "scaling"), "min")) : null,
    max: child(child(el, "scaling") || { children: [] }, "max") ? num(val(child(el, "scaling"), "max")) : null,
    reversed: val(child(el, "scaling"), "orientation", "minMax") === "maxMin",
    unit: child(el, "majorUnit") ? num(val(el, "majorUnit")) : null,
    format: child(el, "numFmt")?.attrs.sourceLinked === "1" ? "" : child(el, "numFmt")?.attrs.formatCode || "",
    labels: val(el, "tickLblPos", "nextTo") !== "none", title: richText(all(child(el, "title") || { children: [] }, "rich")[0]),
  };
  const valAxes = kids(area, "valAx").map(axis);
  const catAx = axis(child(area, "catAx") || child(area, "dateAx"));
  const titleEl = child(chart, "title");
  const allSeries = plots.flatMap((p) => p.series);
  let title = null;
  if (titleEl) {
    const text = richText(all(titleEl, "rich")[0]) || (allSeries.length === 1 ? allSeries[0].name : "");
    if (text) title = { text, ...textStyle(child(titleEl, "txPr") || all(titleEl, "rich")[0], helpers, ctx, { ...font, size: font.size * 1.4, color: { ...tx1, a: 0.75 } }) };
  } else if (!isOn(chart, "autoTitleDeleted") && allSeries.length === 1) {
    title = { text: allSeries[0].name, ...font, size: font.size * 1.4 };
  }
  const legendEl = child(chart, "legend");
  const legend = legendEl ? { pos: val(legendEl, "legendPos", "r") || "r", ...textStyle(child(legendEl, "txPr"), helpers, ctx, font) } : null;
  return {
    title, legend, font, plots,
    catAx, valAx: valAxes.find((a) => !["b", "t"].includes(a.pos)) || valAxes[0] || null, xAx: valAxes.find((a) => ["b", "t"].includes(a.pos)) || null,
    background: helpers.fillOf(child(space, "spPr"), ctx) || null,
    plotBackground: helpers.fillOf(child(area, "spPr"), ctx) || null,
  };
}

function labelsOf(d) {
  if (!d || isOn(d, "delete")) return null;
  const show = { value: isOn(d, "showVal"), percent: isOn(d, "showPercent"), category: isOn(d, "showCatName"), series: isOn(d, "showSerName") };
  if (!show.value && !show.percent && !show.category && !show.series) return null;
  const fmt = child(d, "numFmt");
  return { ...show, format: fmt && fmt.attrs.sourceLinked !== "1" ? fmt.attrs.formatCode : "", pos: val(d, "dLblPos", "") };
}

// ---- scales ----

// niceScale picks an axis from lo to hi: round ends and a round step, about
// count steps, a given minimum, maximum or step kept.
export function niceScale(lo, hi, count = 5, fixed = {}) {
  if (!Number.isFinite(lo) || !Number.isFinite(hi)) { lo = 0; hi = 1; }
  if (lo === hi) { if (lo === 0) hi = 1; else if (lo > 0) lo = 0; else hi = 0; }
  const span = (fixed.max ?? hi) - (fixed.min ?? lo);
  const raw = span / Math.max(1, count);
  const mag = 10 ** Math.floor(Math.log10(raw));
  const norm = raw / mag;
  const step = fixed.unit || (norm <= 1 ? 1 : norm <= 2 ? 2 : norm <= 2.5 ? 2.5 : norm <= 5 ? 5 : 10) * mag;
  const min = fixed.min ?? Math.floor(lo / step + 1e-9) * step;
  const max = fixed.max ?? Math.ceil(hi / step - 1e-9) * step;
  const ticks = [];
  for (let v = min, i = 0; v <= max + step * 1e-6 && i < 1000; v = min + ++i * step) ticks.push(+v.toFixed(12));
  return { min, max: max > min ? max : min + step, step, ticks };
}

// valueRange is the range the values of a chart's plots span: stacked
// series by their sums, percent stacks from 0 to 1, and from zero as the
// presentation program starts unless the values lie far from it.
export function valueRange(plots, which = "y") {
  let lo = Infinity, hi = -Infinity;
  let zero = false;
  for (const p of plots) {
    if (p.kind === "pie" || p.kind === "doughnut") continue;
    if (which === "x") {
      for (const s of p.series) for (const c of s.categories) { const v = Number(c); if (c !== "" && Number.isFinite(v)) { lo = Math.min(lo, v); hi = Math.max(hi, v); } }
      continue;
    }
    if (p.grouping === "percentStacked") { lo = Math.min(lo, 0); hi = Math.max(hi, 1); continue; }
    if (p.kind !== "scatter" && p.kind !== "line") zero = true;
    if (p.grouping === "stacked") {
      const n = Math.max(...p.series.map((s) => s.values.length));
      for (let i = 0; i < n; i++) {
        let pos = 0, neg = 0;
        for (const s of p.series) { const v = s.values[i] ?? 0; if (v >= 0) pos += v; else neg += v; }
        hi = Math.max(hi, pos); lo = Math.min(lo, neg);
      }
    } else {
      for (const s of p.series) for (const v of s.values) if (v !== null) { lo = Math.min(lo, v); hi = Math.max(hi, v); }
    }
  }
  if (lo === Infinity) return { lo: 0, hi: 1 };
  // Values far from zero keep their own range; otherwise the axis starts
  // at zero.
  if (lo > 0 && (zero || lo < hi * 5 / 6)) lo = 0;
  if (hi < 0 && (zero || hi > lo * 5 / 6)) hi = 0;
  return { lo, hi };
}

// formatNumber writes a value in a number format of the presentation:
// General, decimals (0.0), thousands (#,##0), percent (0%), with literal
// text before and after ("CHF "#,##0). locale is for the separators.
export function formatNumber(v, code = "", locale = undefined) {
  if (v === null || v === undefined || Number.isNaN(v)) return "";
  const section = String(code).split(";")[v < 0 && String(code).includes(";") ? 1 : 0] || "";
  if (!section || /^general$/i.test(section.trim())) {
    const n = Number(Number(v).toPrecision(10));
    return Math.abs(n) >= 1e11 || (Math.abs(n) < 1e-4 && n !== 0) ? n.toExponential(2) : new Intl.NumberFormat(locale, { maximumFractionDigits: 9, useGrouping: false }).format(n);
  }
  const clean = section.replace(/\[[^\]]*\]/g, "").replace(/_.|\*./g, "");
  const lit = clean.replace(/"([^"]*)"|\\(.)/g, (_, q, e) => `\u0001${q ?? e}\u0002`);
  // Literal text is masked while the placeholders are looked for.
  const masked = lit.replace(/\u0001[^\u0002]*\u0002/g, (m) => "\u0003".repeat(m.length));
  const first = masked.search(/[#0?]/);
  if (first < 0) return section.replace(/"/g, "");
  let last = masked.length - 1;
  while (last >= 0 && !/[#0?.,%]/.test(masked[last])) last--;
  const unquote = (s) => s.replace(/\u0001([^\u0002]*)\u0002/g, "$1");
  const prefix = unquote(lit.slice(0, first)), suffix = unquote(lit.slice(last + 1));
  const body = masked.slice(first, last + 1).replace(/\u0003/g, "");
  const percent = body.includes("%") || suffix.includes("%");
  const decimals = (body.split(".")[1] || "").replace(/[^0?#]/g, "").length;
  const minDec = (body.split(".")[1] || "").replace(/[^0]/g, "").length;
  const grouping = /[#0],[#0]/.test(body);
  const scaled = (percent ? v * 100 : v) / 1000 ** ((body.match(/,+$/)?.[0].length) || 0);
  const out = new Intl.NumberFormat(locale, { minimumFractionDigits: minDec, maximumFractionDigits: decimals, useGrouping: grouping }).format(Math.abs(scaled));
  const sign = v < 0 && !String(code).includes(";") ? "-" : "";
  return `${sign}${prefix}${out}${body.includes("%") ? "%" : ""}${suffix}`;
}
