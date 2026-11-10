// filenameOf reads the file name a download names in its
// Content-Disposition header: filename*=UTF-8''… (RFC 5987), which the
// server sends for names beyond ASCII, or filename="…". It answers "" when
// the header names none. The drawing editor keeps a drawing's name this
// way when it saves a new version (ADR-0180).
export function filenameOf(header) {
  if (typeof header !== "string") return "";
  const ext = /filename\*\s*=\s*([^']*)'[^']*'([^;]+)/i.exec(header);
  if (ext) {
    try {
      return decodeURIComponent(ext[2].trim());
    } catch {
      // a malformed encoding: fall back to the plain parameter
    }
  }
  const quoted = /filename\s*=\s*"((?:[^"\\]|\\.)*)"/i.exec(header);
  if (quoted) return quoted[1].replace(/\\(.)/g, "$1");
  const plain = /filename\s*=\s*([^;\s]+)/i.exec(header);
  return plain ? plain[1] : "";
}
