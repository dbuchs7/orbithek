// A thin client of the HTTP API (ADR-0028). Requests carry the session
// cookie; bodies are always JSON, which is also the CSRF defence (ADR-0027).

import { t } from "./i18n.js";

// nameOf answers a person's name for their ID, or nothing; the app sets it
// once it knows names, so that an error names a person and not their ID.
let nameOf = () => null;
export function setNamer(fn) { nameOf = fn; }

// errorDetail is the server's own words for a failure, in English, made
// readable: without the class it begins with, which the lead says in the
// reader's language, and with people named instead of their IDs.
export function errorDetail(message) {
  return String(message || "")
    .replace(/^(?:forbidden|not found|validation failed|bad input): /, "")
    .replace(/\b[0-9a-f]{32}\b/g, (id) => nameOf(id) || id);
}

// wordError words a failed request for the reader (ADR-0017): what kind
// of failure it is, in their language, by its status, followed by the
// server's detail where it tells more. path names the request, since a
// refused sign-in is not an ended session.
export function wordError(status, message, path = "") {
  const detail = errorDetail(message);
  const say = (lead, more = detail) => (more ? `${lead}: ${more}` : `${lead}.`);
  switch (true) {
    case status === 401 && path === "/api/login": return say(t("error.credentials"), "");
    case status === 401: return say(t("error.signedOut"), "");
    case status === 400 || status === 422: return say(t("error.invalid"));
    case status === 403: return say(t("error.forbidden"));
    case status === 404: return say(t("error.notFound"));
    case status === 409: return say(t("error.conflict"));
    case status === 413: return say(t("error.tooLarge"));
    case status === 429: return say(t("error.tooMany"));
    case status === 507: return say(t("error.storage"));
    case status >= 500: return say(t("error.server"));
    default: return detail;
  }
}

export class ApiError extends Error {
  // The message is worded for the reader (wordError); detail keeps the
  // server's own words. A failure the UI words itself keeps them as its
  // message: one with at, where a mistake in a query is, in characters
  // from 0 (ADR-0136), or with code, such as a calendar server refusing a
  // password (ADR-0174). body is the whole answer, such as the problems an
  // engine names with a refusal (ADR-0251); path names the request.
  constructor(status, message, at, code, body, path) {
    super(at === undefined && !code ? wordError(status, message, path) : message);
    this.status = status;
    this.detail = message;
    this.at = at;
    this.code = code;
    this.body = body;
  }
}

export async function api(method, path, body) {
  const init = { method, credentials: "same-origin", headers: {} };
  if (body !== undefined) {
    init.headers["Content-Type"] = "application/json";
    init.body = JSON.stringify(body);
  }
  const res = await fetch(path, init);
  let data = null;
  if (res.status !== 204) {
    const text = await res.text();
    if (text) {
      try {
        data = JSON.parse(text);
      } catch {
        data = { error: text };
      }
    }
  }
  if (!res.ok) throw new ApiError(res.status, (data && data.error) || res.statusText, data?.at, data?.code, data, path);
  return data;
}

export const get = (p) => api("GET", p);
export const post = (p, b = {}) => api("POST", p, b);
export const put = (p, b = {}) => api("PUT", p, b);
export const del = (p) => api("DELETE", p);

// upload sends a file as an attachment of an item (ADR-0045). The body is
// the file itself, as application/octet-stream, a type a cross-site form
// cannot send. source ({item, attachment}) names the file it is a copy or
// a new version of (ADR-0055).
export async function upload(itemPath, file, source, extra = {}) {
  const q = new URLSearchParams({ name: file.name, ...extra });
  if (source) q.set("sourceItem", source.item), q.set("sourceAttachment", source.attachment);
  const res = await fetch(`${itemPath}/attachments?${q}`, {
    method: "POST", credentials: "same-origin",
    headers: { "Content-Type": "application/octet-stream" }, body: file,
  });
  let data = null;
  try { data = await res.json(); } catch { /* not JSON */ }
  if (!res.ok) throw new ApiError(res.status, (data && data.error) || res.statusText);
  return data.attachment;
}

// download POSTs body and returns the response as a file: its contents and
// the name the server suggests.
export async function download(path, body) {
  const res = await fetch(path, {
    method: "POST", credentials: "same-origin",
    headers: { "Content-Type": "application/json" }, body: JSON.stringify(body),
  });
  if (!res.ok) {
    let message = res.statusText;
    try { message = (await res.json()).error || message; } catch { /* not JSON */ }
    throw new ApiError(res.status, message);
  }
  const name = /filename="([^"]+)"/.exec(res.headers.get("Content-Disposition") || "")?.[1] || "download";
  return { blob: await res.blob(), name };
}
