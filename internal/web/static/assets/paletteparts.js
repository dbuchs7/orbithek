// The parts of the command palette (ADR-0207) that need no browser: how
// well what is typed matches a name, the order of the matches, the list of
// recently opened items, and which keys open the palette. palette.js draws
// it; these are plain functions, tested by
// internal/web/jstest/palette.test.mjs.

// fold makes a text comparable: lower case, without accents.
const fold = (s) => String(s || "").normalize("NFD").replace(/\p{M}/gu, "").toLowerCase();

// score rates how well typed matches text, or is -1 when it does not: the
// letters typed must appear in the text in their order. A match at the
// start of the text or of a word, and letters that follow each other,
// count more; a shorter text wins a tie. Nothing typed matches everything
// equally.
export function score(typed, text) {
  const q = fold(typed).replace(/\s+/g, "");
  const s = fold(text);
  if (!q) return 0;
  let at = 0;
  let total = 0;
  let run = 0;
  for (const ch of q) {
    const i = s.indexOf(ch, at);
    if (i < 0) return -1;
    const wordStart = i === 0 || /[\s\-_./:·(]/.test(s[i - 1]);
    run = i === at && at > 0 ? run + 1 : 0;
    total += 1 + (wordStart ? 6 : 0) + run * 3 - Math.min(i - at, 10) * 0.2;
    at = i + 1;
  }
  if (s.startsWith(fold(typed).trim())) total += 20;
  return total - s.length * 0.01;
}

// rank keeps the entries whose text matches what is typed, best first;
// equal ones keep their order. text(entry) gives an entry's words.
export function rank(typed, entries, text) {
  return entries.map((e, i) => ({ e, i, s: score(typed, text(e)) }))
    .filter((x) => x.s >= 0)
    .sort((a, b) => b.s - a.s || a.i - b.i)
    .map((x) => x.e);
}

// RECENT is how many recently opened items this browser remembers.
export const RECENT = 12;

// remember puts an item first in the list of recently opened ones, each
// once, the oldest dropped beyond RECENT. An entry is { key, title }.
export function remember(list, entry) {
  const rest = (Array.isArray(list) ? list : []).filter((e) => e && typeof e.key === "string" && e.key !== entry.key);
  return [{ key: entry.key, title: String(entry.title || "") }, ...rest].slice(0, RECENT);
}

// opens tells whether a key press opens the palette: Ctrl+Shift+P
// (Cmd+Shift+P) anywhere, and Ctrl+K (Cmd+K) where no text is being
// edited, as the editor keeps Ctrl+K for links (ADR-0096).
export function opens(ev, editing) {
  const mod = ev.ctrlKey || ev.metaKey;
  if (!mod || ev.altKey) return false;
  const key = String(ev.key || "").toLowerCase();
  if (ev.shiftKey) return key === "p";
  return key === "k" && !editing;
}
