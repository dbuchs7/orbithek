package storage

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/diskspace"
	"github.com/dbuchs7/orbithek/internal/item"
)

// AttachmentsDir holds the content of attachments (ADR-0045), one directory
// per item and one file per attachment, each encrypted with its item's
// key: destroying the key (ADR-0026) makes the files unreadable, and a
// purge removes them.
const AttachmentsDir = "attachments"

// ErrAttachmentTooLarge means the content exceeds the limit given.
var ErrAttachmentTooLarge = errors.New("the file is larger than allowed")

// The format of an attachment file: a magic number, a random nonce prefix,
// then the content in chunks, each sealed with AES-256-GCM under the item's
// key with a nonce of the prefix and the chunk's number. The last chunk is
// marked in the associated data, so a file cut at a chunk boundary does not
// authenticate.
const (
	attachmentMagic = "WWBBATT1"
	chunkSize       = 64 << 10
)

// PosterSuffix ends the name of the file that holds a video's poster, a
// picture taken from it (ADR-0189), beside the video's own file: sealed
// with the item's key like it, kept while the video is, moved and removed
// with it.
const PosterSuffix = ".poster"

// FacesSuffix ends the name of the file that holds what the browser
// computed of the faces on a picture to recognise them (ADR-0194), beside
// the picture's own file: sealed like it, kept while it is, moved and
// removed with it, but never backed up, and forgotten as a whole when
// recognition is turned off.
const FacesSuffix = ".faces"

// PreviewSuffix ends the name of the file that holds the preview of an
// entry of a file share (ADR-0197): the picture scaled down, sealed with
// the item's key, kept while the entry is, backed up, moved and removed
// with it. Such an entry has no file of its own; its original stays on
// the share.
const PreviewSuffix = ".preview"

// companions are the files kept beside an attachment's own.
var companions = []string{PosterSuffix, FacesSuffix, PreviewSuffix}

// backedUpCompanions are the companions a backup holds; what was computed
// of faces never leaves the server (ADR-0194).
var backedUpCompanions = []string{PosterSuffix, PreviewSuffix}

// companionOf answers the attachment a companion file belongs to, or ""
// for a name that is none.
func companionOf(name string) item.ID {
	for _, suffix := range companions {
		if base, ok := strings.CutSuffix(name, suffix); ok && item.ValidAttachmentID(base) {
			return item.ID(base)
		}
	}
	return ""
}

// validFile reports whether a name is an attachment's file or one of its
// companions'.
func validFile(name item.ID) bool {
	return item.ValidAttachmentID(string(name)) || companionOf(string(name)) != ""
}

func attachmentPath(dir string, id, attachment item.ID) string {
	return filepath.Join(dir, AttachmentsDir, string(id), string(attachment))
}

// The prefix binds the ciphertext to its purpose. It is part of the stored
// data and keeps the product's former name; changing it makes everything
// sealed before unreadable (ADR-0153, ADR-0155).
func chunkAAD(id, attachment item.ID, n uint32, last bool) []byte {
	flag := byte(0)
	if last {
		flag = 1
	}
	return fmt.Appendf(nil, "wikiwikibangbang attachment %s %s %d %d", id, attachment, n, flag)
}

func chunkNonce(prefix []byte, n uint32) []byte {
	return binary.BigEndian.AppendUint32(append([]byte{}, prefix...), n)
}

// itemKey returns a copy of an item's key, for content stored outside the
// event log.
func (db *DB) itemKey(id item.ID) ([]byte, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.keys.shredded[id] {
		return nil, fmt.Errorf("storage: the key of %s was destroyed", id)
	}
	k := db.keys.keys[id]
	if k == nil {
		return nil, fmt.Errorf("storage: item %s has no key yet", id)
	}
	return append([]byte{}, k...), nil
}

// StoreAttachment encrypts content of at most limit bytes for an item and
// stores it under a new attachment ID. The result describes it except for
// its name; recording it on the item (item.Store.AddAttachment) is the
// caller's next step, and a file never recorded is removed on the next
// start.
func (db *DB) StoreAttachment(id item.ID, content io.Reader, limit int64) (item.Attachment, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return item.Attachment{}, err
	}
	return db.sealAttachment(id, item.ID(hex.EncodeToString(raw[:])), content, limit)
}

// MoveAttachmentContent stores the content of a file of one item for
// another item under the same attachment ID, sealed with the other item's
// key, for a move that keeps the file's identity (ADR-0188). The content
// read must be the file as recorded (want); the file of the first item
// stays until the move is recorded, which removes it. Recording the move
// is the caller's next step; a file never recorded is removed on the next
// start, or now with DiscardAttachment.
func (db *DB) MoveAttachmentContent(from, to item.ID, want item.Attachment) error {
	// An entry of a file share has no file of its own here, only its
	// preview, which must move (ADR-0197).
	if want.Linked() {
		p, err := db.OpenPreview(from, want.ID)
		if err != nil {
			return err
		}
		defer p.Close()
		if _, err := db.sealAttachment(to, want.ID+PreviewSuffix, p, budget.Get().AttachmentBytes); err != nil {
			db.DiscardAttachment(to, want.ID)
			return err
		}
		db.moveFaces(from, to, want.ID)
		return nil
	}
	r, err := db.OpenAttachment(from, want.ID)
	if err != nil {
		return err
	}
	defer r.Close()
	got, err := db.sealAttachment(to, want.ID, r, want.Size)
	if err != nil {
		return err
	}
	if got.Size != want.Size || got.SHA256 != want.SHA256 {
		db.DiscardAttachment(to, want.ID)
		return fmt.Errorf("storage: the content of %s is not the file recorded", want.ID)
	}
	// A video's poster goes with it (ADR-0189); one that does not read is
	// left behind, and the video shows without it.
	if p, err := db.OpenPoster(from, want.ID); err == nil {
		_, err := db.sealAttachment(to, want.ID+PosterSuffix, p, budget.Get().AttachmentBytes)
		p.Close()
		if err != nil {
			os.Remove(attachmentPath(db.dir, to, want.ID+PosterSuffix))
		}
	}
	db.moveFaces(from, to, want.ID)
	return nil
}

// moveFaces moves what was computed of a picture's faces with it
// (ADR-0194); what does not read is left behind.
func (db *DB) moveFaces(from, to, picture item.ID) {
	if f, err := db.openFile(from, picture+FacesSuffix); err == nil {
		_, err := db.sealAttachment(to, picture+FacesSuffix, f, maxFaceVectorBytes)
		f.Close()
		if err != nil {
			os.Remove(attachmentPath(db.dir, to, picture+FacesSuffix))
		}
	}
}

// StorePreview stores the preview of an entry of a file share
// (ADR-0197), replacing one stored before, sealed with the item's key.
// The entry is recorded after its preview, so a preview never recorded is
// removed on the next start, or now with DiscardAttachment.
func (db *DB) StorePreview(id, entry item.ID, content io.Reader, limit int64) (item.Attachment, error) {
	if !item.ValidAttachmentID(string(entry)) {
		return item.Attachment{}, fmt.Errorf("storage: attachment ID %q", entry)
	}
	return db.sealAttachment(id, entry+PreviewSuffix, content, limit)
}

// OpenPreview returns the decrypted preview of an entry of a file share,
// or an error that matches os.ErrNotExist when it has none.
func (db *DB) OpenPreview(id, entry item.ID) (io.ReadSeekCloser, error) {
	if !item.ValidAttachmentID(string(entry)) {
		return nil, fmt.Errorf("storage: attachment %q: %w", entry, os.ErrNotExist)
	}
	return db.openFile(id, entry+PreviewSuffix)
}

// StorePoster stores a picture taken from a video as the video's poster
// (ADR-0189), replacing one stored before, sealed with the item's key.
// The caller has checked that the video is a recorded file of the item;
// the result tells the poster's type and size.
func (db *DB) StorePoster(id, video item.ID, content io.Reader, limit int64) (item.Attachment, error) {
	if !item.ValidAttachmentID(string(video)) {
		return item.Attachment{}, fmt.Errorf("storage: attachment ID %q", video)
	}
	return db.sealAttachment(id, video+PosterSuffix, content, limit)
}

// HasPoster reports whether a video has a poster.
func (db *DB) HasPoster(id, video item.ID) bool {
	if !item.ValidAttachmentID(string(video)) {
		return false
	}
	_, err := os.Stat(attachmentPath(db.dir, id, video+PosterSuffix))
	return err == nil
}

// DiscardPoster removes a video's poster.
func (db *DB) DiscardPoster(id, video item.ID) {
	if item.ValidAttachmentID(string(video)) {
		os.Remove(attachmentPath(db.dir, id, video+PosterSuffix))
	}
}

// OpenPoster returns the decrypted poster of a video, or an error that
// matches os.ErrNotExist when it has none.
func (db *DB) OpenPoster(id, video item.ID) (io.ReadSeekCloser, error) {
	if !item.ValidAttachmentID(string(video)) {
		return nil, fmt.Errorf("storage: attachment %q: %w", video, os.ErrNotExist)
	}
	return db.openFile(id, video+PosterSuffix)
}

// sealAttachment encrypts content of at most limit bytes for an item and
// stores it under the attachment ID given.
func (db *DB) sealAttachment(id, attachment item.ID, content io.Reader, limit int64) (item.Attachment, error) {
	if db.Space() >= diskspace.Critical {
		return item.Attachment{}, ErrNoSpace
	}
	if !validFile(attachment) {
		return item.Attachment{}, fmt.Errorf("storage: attachment ID %q", attachment)
	}
	key, err := db.itemKey(id)
	if err != nil {
		return item.Attachment{}, err
	}
	a := item.Attachment{ID: attachment}
	path := attachmentPath(db.dir, id, a.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return item.Attachment{}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "upload-*.tmp")
	if err != nil {
		return item.Attachment{}, err
	}
	defer func() {
		if tmp != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	gcm, err := newGCM(key)
	if err != nil {
		return item.Attachment{}, err
	}
	prefix := make([]byte, gcm.NonceSize()-4)
	if _, err := rand.Read(prefix); err != nil {
		return item.Attachment{}, err
	}
	w := bufio.NewWriter(tmp)
	w.WriteString(attachmentMagic)
	w.Write(prefix)
	in := bufio.NewReaderSize(io.LimitReader(content, limit+1), chunkSize)
	sum := sha256.New()
	buf := make([]byte, chunkSize)
	var sealed []byte
	for n := uint32(0); ; n++ {
		got, err := io.ReadFull(in, buf)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			return item.Attachment{}, err
		}
		a.Size += int64(got)
		if a.Size > limit {
			return item.Attachment{}, ErrAttachmentTooLarge
		}
		if n == 0 {
			a.Type = http.DetectContentType(buf[:got])
		}
		_, peek := in.Peek(1)
		last := got < chunkSize || errors.Is(peek, io.EOF)
		sum.Write(buf[:got])
		sealed = gcm.Seal(sealed[:0], chunkNonce(prefix, n), buf[:got], chunkAAD(id, a.ID, n, last))
		if _, err := w.Write(sealed); err != nil {
			return item.Attachment{}, err
		}
		if last {
			break
		}
	}
	a.SHA256 = hex.EncodeToString(sum.Sum(nil))
	if err := w.Flush(); err != nil {
		return item.Attachment{}, err
	}
	if err := tmp.Sync(); err != nil {
		return item.Attachment{}, err
	}
	if err := tmp.Close(); err != nil {
		return item.Attachment{}, err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return item.Attachment{}, err
	}
	tmp = nil
	return a, nil
}

// OpenAttachment returns the decrypted content of an attachment. A read
// fails if the file was changed, cut short or extended. The content can be
// read from any position, as a video player asks for its parts (ADR-0097);
// each chunk is authenticated as it is read.
func (db *DB) OpenAttachment(id, attachment item.ID) (io.ReadSeekCloser, error) {
	if !item.ValidAttachmentID(string(attachment)) {
		return nil, fmt.Errorf("storage: attachment %q: %w", attachment, os.ErrNotExist)
	}
	r, err := db.openFile(id, attachment)
	// An entry of a file share has no file of its own here: what is given
	// out of it, wherever that is, is its preview (ADR-0197). Only such an
	// entry has a preview.
	if errors.Is(err, os.ErrNotExist) {
		if p, perr := db.openFile(id, attachment+PreviewSuffix); perr == nil {
			return p, nil
		}
	}
	return r, err
}

// openFile opens an attachment's file or its poster's.
func (db *DB) openFile(id, attachment item.ID) (io.ReadSeekCloser, error) {
	key, err := db.itemKey(id)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(attachmentPath(db.dir, id, attachment))
	if err != nil {
		return nil, err
	}
	gcm, err := newGCM(key)
	if err != nil {
		f.Close()
		return nil, err
	}
	head := make([]byte, len(attachmentMagic)+gcm.NonceSize()-4)
	info, err := f.Stat()
	if err == nil {
		_, err = io.ReadFull(f, head)
	}
	if err != nil || string(head[:len(attachmentMagic)]) != attachmentMagic {
		f.Close()
		return nil, fmt.Errorf("storage: attachment %s is not an attachment file", attachment)
	}
	sealed := int64(chunkSize + gcm.Overhead())
	body := info.Size() - int64(len(head))
	chunks := (body + sealed - 1) / sealed
	if chunks < 1 {
		f.Close()
		return nil, fmt.Errorf("storage: attachment %s is cut short", attachment)
	}
	return &attachmentReader{f: f, head: int64(len(head)), sealed: sealed, chunks: chunks, size: body - chunks*int64(gcm.Overhead()),
		open: func(n uint32, ct []byte, last bool) ([]byte, error) {
			return gcm.Open(nil, chunkNonce(head[len(attachmentMagic):], n), ct, chunkAAD(id, attachment, n, last))
		}, loaded: -1}, nil
}

// attachmentReader reads an attachment's content from any position: the
// chunk holding it is read and authenticated, the last chunk as the last.
type attachmentReader struct {
	f      *os.File
	open   func(n uint32, ct []byte, last bool) ([]byte, error)
	head   int64 // bytes before the first chunk
	sealed int64 // the size of a full chunk, sealed
	chunks int64
	size   int64 // the content's size
	pos    int64
	loaded int64 // the chunk in plain, or -1
	plain  []byte
}

func (r *attachmentReader) Read(p []byte) (int, error) {
	if r.pos >= r.size {
		// An empty file is one empty chunk, which must still authenticate.
		if r.size == 0 && r.loaded != 0 {
			if err := r.load(0); err != nil {
				return 0, err
			}
		}
		return 0, io.EOF
	}
	k := r.pos / chunkSize
	if k != r.loaded {
		if err := r.load(k); err != nil {
			return 0, err
		}
	}
	n := copy(p, r.plain[r.pos-k*chunkSize:])
	r.pos += int64(n)
	return n, nil
}

func (r *attachmentReader) load(k int64) error {
	ct := make([]byte, r.sealed)
	got, err := r.f.ReadAt(ct, r.head+k*r.sealed)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	last := k == r.chunks-1
	if !last && int64(got) < r.sealed {
		return errors.New("storage: attachment content is cut short")
	}
	pt, err := r.open(uint32(k), ct[:got], last)
	if err != nil {
		return fmt.Errorf("storage: attachment content does not authenticate: %w", err)
	}
	if !last && len(pt) != chunkSize {
		return errors.New("storage: attachment content does not authenticate")
	}
	r.plain, r.loaded = pt, k
	return nil
}

func (r *attachmentReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += r.pos
	case io.SeekEnd:
		offset += r.size
	default:
		return 0, errors.New("storage: invalid whence")
	}
	if offset < 0 {
		return 0, errors.New("storage: negative position")
	}
	r.pos = offset
	return offset, nil
}

func (r *attachmentReader) Close() error { return r.f.Close() }

// DiscardAttachment removes the file of an upload that was not recorded.
func (db *DB) DiscardAttachment(id, attachment item.ID) {
	if item.ValidAttachmentID(string(attachment)) {
		os.Remove(attachmentPath(db.dir, id, attachment))
		for _, suffix := range companions {
			os.Remove(attachmentPath(db.dir, id, attachment+item.ID(suffix)))
		}
	}
}

// removeAttachments deletes stored content that is no longer needed: the
// directories of items gone (purged, redacted) and the files of deleted
// attachments. A failure is left to healAttachments on the next start.
func (db *DB) removeAttachments(items []item.ID, files map[item.ID][]item.ID) {
	for _, id := range items {
		os.RemoveAll(filepath.Join(db.dir, AttachmentsDir, string(id)))
	}
	for id, list := range files {
		for _, a := range list {
			os.Remove(attachmentPath(db.dir, id, a))
			for _, suffix := range companions {
				os.Remove(attachmentPath(db.dir, id, a+item.ID(suffix)))
			}
		}
	}
}

// healAttachments removes stored files that no attachment needs: left by a
// crash before an upload was recorded, or after a deletion or purge was
// recorded but before its files were removed. It returns how many.
func (db *DB) healAttachments() (int, error) {
	root := filepath.Join(db.dir, AttachmentsDir)
	dirs, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	live := db.Store.LiveAttachments()
	removed := 0
	for _, d := range dirs {
		need := map[item.ID]bool{}
		for _, a := range live[item.ID(d.Name())] {
			need[a] = true
		}
		files, err := os.ReadDir(filepath.Join(root, d.Name()))
		if err != nil {
			return removed, err
		}
		for _, f := range files {
			name := item.ID(f.Name())
			if !need[name] && !need[companionOf(f.Name())] || strings.HasSuffix(f.Name(), ".tmp") {
				if err := os.Remove(filepath.Join(root, d.Name(), f.Name())); err != nil {
					return removed, err
				}
				removed++
			}
		}
		if len(need) == 0 {
			os.Remove(filepath.Join(root, d.Name()))
		}
	}
	return removed, nil
}
