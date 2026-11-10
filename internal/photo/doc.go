// Package photo serves the album's photo management (ADR-0177): it reads
// what a picture tells about itself — capture time, place, camera and
// exposure from its EXIF data, and its pixel size from its header — and it
// makes thumbnails.
//
// Pictures are uploads and therefore untrusted. Read is best effort and never
// panics; every loop over the file's structure (segments, chunks, IFD
// entries) is bounded, no offset is followed outside the data it belongs to,
// and only the IFDs named by IFD0 are followed, each at most once.
// Thumbnail checks the pixel count from the header before it decodes, so a
// small file that claims a huge picture is refused instead of decoded.
//
// Only the standard library is used. WebP has no decoder there, so Read gives
// its size and EXIF data but Thumbnail refuses it with ErrUnsupported.
package photo
