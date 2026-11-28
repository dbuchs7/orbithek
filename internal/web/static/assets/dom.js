// The only way this UI creates elements (ADR-0014): every piece of text is
// inserted as text, never parsed as markup. Nothing here or elsewhere in the
// UI may use innerHTML, outerHTML, insertAdjacentHTML, document.write or
// eval; a test in internal/web enforces that.

// h creates an element. attrs may hold: class, text, on<event> handlers,
// and plain attributes; children are elements, strings or arrays of them.
export function h(tag, attrs = {}, ...children) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (v === undefined || v === null || v === false) continue;
    if (k === "class") el.className = v;
    else if (k === "text") el.textContent = v;
    else if (k.startsWith("on")) el.addEventListener(k.slice(2), v);
    else if (k === "href") el.setAttribute("href", safeHref(v));
    else if (k === "value") el.value = v;
    else if (k === "checked") el.checked = Boolean(v);
    else el.setAttribute(k, v === true ? "" : String(v));
  }
  append(el, children);
  return el;
}

function append(el, children) {
  for (const c of children) {
    if (c === undefined || c === null || c === false) continue;
    if (Array.isArray(c)) append(el, c);
    else el.append(c instanceof Node ? c : document.createTextNode(String(c)));
  }
}

// safeHref allows same-origin paths and fragments, http(s) and mailto
// (ADR-0014); anything else, javascript: and data: in particular, becomes "#".
export function safeHref(v) {
  const s = String(v);
  if (s.startsWith("#") || (s.startsWith("/") && !s.startsWith("//"))) return s;
  try {
    const u = new URL(s);
    if (["http:", "https:", "mailto:"].includes(u.protocol)) return u.href;
  } catch {}
  return "#";
}

// shareHref allows what opens an original where it lies on a file share
// (ADR-0197): https: and smb: addresses only, as the share's link
// template is checked on the server. It is set only on the link that
// opens an original; every other link keeps safeHref's rule. Anything
// else becomes "#".
export function shareHref(v) {
  // The wiki's own route that signs a link to an S3 share's original.
  if (/^\/api\/items\/[^/]+\/attachments\/[0-9a-f]{32}\/original$/.test(String(v))) return String(v);
  try {
    const u = new URL(String(v));
    if (u.protocol === "https:" || u.protocol === "smb:") return u.href;
  } catch {}
  return "#";
}

// mount replaces the content of a container.
export function mount(container, ...children) {
  container.replaceChildren();
  append(container, children);
}

// when writes a point in time where the reader is, in the UI language
// (ADR-0017), which i18n.js sets on the document: the date and the time to
// the minute, the same in every view. What is no time stays as it is.
export function when(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? String(iso) : d.toLocaleString(document.documentElement.lang || undefined, { dateStyle: "medium", timeStyle: "short" });
}

// day writes a calendar day, YYYY-MM-DD, in the UI language. A day has no
// time zone: it is the same day wherever it is read. A point in time is
// written as when writes it, and what is no date stays as it is.
export function day(iso) {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(String(iso || ""));
  if (!m) return /^\d{4}-\d{2}-\d{2}T/.test(String(iso || "")) ? when(iso) : String(iso || "");
  return new Date(+m[1], +m[2] - 1, +m[3]).toLocaleDateString(document.documentElement.lang || undefined, { dateStyle: "medium" });
}
