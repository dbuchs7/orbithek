// zipread reads the files of a ZIP archive in the browser (ADR-0175), so
// a photo export packed as an archive is unpacked where it was chosen and
// its pictures are uploaded one by one: the server never holds the archive
// or a file outside its item's encryption. It reads stored and deflated
// entries, ZIP64 included, with the platform's DecompressionStream, and
// needs no browser beyond that, so node --test runs it.

const EOCD = 0x06054b50;
const EOCD64 = 0x06064b50;
const EOCD64_LOCATOR = 0x07064b50;
const CENTRAL = 0x02014b50;
const LOCAL = 0x04034b50;

// MAX_ENTRIES bounds the entries read from one archive.
export const MAX_ENTRIES = 100000;

const bytes = async (blob, start, end) => new Uint8Array(await blob.slice(start, end).arrayBuffer());

// findEnd finds the end of central directory record in the archive's
// last 64 KiB and 22 bytes, the most a comment allows.
async function findEnd(blob) {
  const from = Math.max(0, blob.size - 65557);
  const tail = await bytes(blob, from, blob.size);
  const v = new DataView(tail.buffer);
  for (let i = tail.length - 22; i >= 0; i--) {
    if (v.getUint32(i, true) === EOCD) return { view: v, at: i, offset: from + i };
  }
  throw new Error("zip: not a ZIP archive");
}

// big reads a 64-bit length as a number; archives beyond 2^53 bytes are
// not read.
const big = (v, at) => {
  const n = v.getBigUint64(at, true);
  if (n > BigInt(Number.MAX_SAFE_INTEGER)) throw new Error("zip: too large");
  return Number(n);
};

// entries lists the files of an archive: name, sizes, method and where
// their data begins, from its central directory.
export async function entries(blob) {
  const end = await findEnd(blob);
  let count = end.view.getUint16(end.at + 10, true);
  let size = end.view.getUint32(end.at + 12, true);
  let start = end.view.getUint32(end.at + 16, true);
  if (count === 0xffff || size === 0xffffffff || start === 0xffffffff) {
    const loc = new DataView((await bytes(blob, end.offset - 20, end.offset)).buffer);
    if (loc.getUint32(0, true) !== EOCD64_LOCATOR) throw new Error("zip: damaged ZIP64 archive");
    const rec = new DataView((await bytes(blob, big(loc, 8), big(loc, 8) + 56)).buffer);
    if (rec.getUint32(0, true) !== EOCD64) throw new Error("zip: damaged ZIP64 archive");
    count = big(rec, 32);
    size = big(rec, 40);
    start = big(rec, 48);
  }
  if (count > MAX_ENTRIES) throw new Error("zip: too many entries");
  if (start + size > blob.size) throw new Error("zip: damaged archive");
  const dir = await bytes(blob, start, start + size);
  const v = new DataView(dir.buffer);
  const out = [];
  const utf8 = new TextDecoder("utf-8");
  let at = 0;
  for (let i = 0; i < count; i++) {
    if (at + 46 > dir.length || v.getUint32(at, true) !== CENTRAL) throw new Error("zip: damaged directory");
    const flags = v.getUint16(at + 8, true);
    const method = v.getUint16(at + 10, true);
    let compressed = v.getUint32(at + 20, true);
    let length = v.getUint32(at + 24, true);
    const nameLen = v.getUint16(at + 28, true);
    const extraLen = v.getUint16(at + 30, true);
    const commentLen = v.getUint16(at + 32, true);
    let local = v.getUint32(at + 42, true);
    const raw = dir.subarray(at + 46, at + 46 + nameLen);
    // Names are UTF-8 when the archive says so (bit 11); archives that do
    // not are read as UTF-8 too, which their ASCII names are.
    const name = utf8.decode(raw);
    let e = at + 46 + nameLen;
    const extraEnd = e + extraLen;
    while (e + 4 <= extraEnd) {
      const id = v.getUint16(e, true);
      const n = v.getUint16(e + 2, true);
      if (id === 0x0001) {
        let p = e + 4;
        if (length === 0xffffffff) { length = big(v, p); p += 8; }
        if (compressed === 0xffffffff) { compressed = big(v, p); p += 8; }
        if (local === 0xffffffff) { local = big(v, p); }
      }
      e += 4 + n;
    }
    out.push({ name, method, compressed, size: length, local, encrypted: (flags & 1) === 1 });
    at = extraEnd + commentLen;
  }
  return out;
}

// read gives the content of one entry as a Blob of the given type.
export async function read(blob, entry, type = "") {
  if (entry.encrypted) throw new Error("zip: the file is encrypted");
  const head = new DataView((await bytes(blob, entry.local, entry.local + 30)).buffer);
  if (head.getUint32(0, true) !== LOCAL) throw new Error("zip: damaged entry");
  const data = entry.local + 30 + head.getUint16(26, true) + head.getUint16(28, true);
  const part = blob.slice(data, data + entry.compressed);
  if (entry.method === 0) return new Blob([part], { type });
  if (entry.method !== 8) throw new Error("zip: unknown compression");
  const stream = part.stream().pipeThrough(new DecompressionStream("deflate-raw"));
  const out = await new Response(stream).blob();
  return new Blob([out], { type });
}
