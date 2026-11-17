// Numbered headings (ADR-draft-numbered-headings): a body that is numbered
// shows the number of each heading by its level — 1, 1.1, 1.1.1 — in the
// text and in its tables of contents. The numbers are computed when shown,
// never stored, exactly as the server's item.HeadingNumbers computes them
// for exports; internal/web/jstest/heading-numbers-vector.json holds both
// to the same cases. This module has no dependencies, so it runs in Node.

// headingNumbers returns the number of each block, "" for one that is no
// heading or a heading without text. blocks are [{kind, level, text}]. The
// shallowest level used counts as the first; a level skipped below another
// counts as 0, as in "1.0.1".
export function headingNumbers(blocks) {
  const numbered = (b) => b.kind === "heading" && String(b.text ?? "").trim() !== "";
  const level = (b) => Math.min(Math.max(Number(b.level) || 1, 1), 3);
  let top = 0;
  for (const b of blocks) if (numbered(b) && (top === 0 || level(b) < top)) top = level(b);
  const count = [0, 0, 0];
  return blocks.map((b) => {
    if (!numbered(b)) return "";
    const depth = level(b) - top;
    count[depth]++;
    for (let j = depth + 1; j < count.length; j++) count[j] = 0;
    return count.slice(0, depth + 1).join(".");
  });
}
