// Package noteimport imports a ZIP archive of notes into a room
// (ADR-0209): a folder of Markdown files, such as an Obsidian vault or the
// Markdown export of another notes application, or a Notion workspace
// export of Markdown and CSV files. Folders become a tree of pages, links
// between notes become mentions of the imported pages, pictures become
// attachments, front matter and tags become fields and labels. A dry run
// lists what a run would do; a run updates what an earlier one made and
// never makes anything twice.
package noteimport

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"strings"
	"unicode/utf8"
)

// Limits bound what one archive may make the server read (ADR-0015); the
// caller takes them from the budgets.
type Limits struct {
	// Files is how many files the archive may hold.
	Files int
	// Bytes is how many bytes all files read from it may hold unpacked,
	// together: what keeps a small archive from unpacking into much.
	Bytes int64
}

// ErrArchive is an archive that is not read: not a ZIP archive, or one
// beyond its limits.
var ErrArchive = errors.New("noteimport: archive")

// ErrTooLarge is a file of the archive beyond what is read of it.
var ErrTooLarge = errors.New("noteimport: file too large")

// Archive is a ZIP archive opened for reading: its files by their paths,
// cleaned, with the folder that holds all of them cut off.
type Archive struct {
	files map[string]*zip.File
	paths []string
	// Root is the name of the one folder that held every file, if there
	// was one; it names the import.
	Root string
	// left counts the bytes that may still be read.
	left int64
	// Skipped names entries that are not read, and why.
	Skipped []string
}

// ignored are folders and files that belong to the application that
// wrote the archive, not to the notes: settings, the trash, the
// archiver's own entries.
func ignored(p string) bool {
	for _, part := range strings.Split(p, "/") {
		if strings.HasPrefix(part, ".") || part == "__MACOSX" || part == "Thumbs.db" {
			return true
		}
	}
	return false
}

// Open reads the directory of a ZIP archive of size bytes. Entries whose
// names leave the archive (absolute, "..", a drive letter) are refused
// and named, as are links; nothing is ever written to disk, so a name
// only ever names a file within the archive.
func Open(r io.ReaderAt, size int64, lim Limits) (*Archive, error) {
	zr, err := zip.NewReader(r, size)
	if errors.Is(err, zip.ErrInsecurePath) {
		err = nil // such names are refused one by one below
	}
	if err != nil {
		return nil, fmt.Errorf("%w: not a ZIP archive: %v", ErrArchive, err)
	}
	a := &Archive{files: map[string]*zip.File{}, left: lim.Bytes}
	count := 0
	for _, f := range zr.File {
		name := strings.ReplaceAll(f.Name, "\\", "/")
		if strings.HasSuffix(name, "/") || f.FileInfo().IsDir() {
			continue
		}
		count++
		if count > lim.Files {
			return nil, fmt.Errorf("%w: it holds more than %d files", ErrArchive, lim.Files)
		}
		p, ok := clean(name)
		switch {
		case !ok:
			a.Skipped = append(a.Skipped, fmt.Sprintf("%q leaves the archive and is not read", f.Name))
			continue
		case !f.Mode().IsRegular():
			a.Skipped = append(a.Skipped, fmt.Sprintf("%s is not a plain file and is not read", p))
			continue
		case ignored(p):
			continue
		}
		if _, dup := a.files[p]; dup {
			a.Skipped = append(a.Skipped, fmt.Sprintf("%s is in the archive twice; the first is read", p))
			continue
		}
		a.files[p] = f
		a.paths = append(a.paths, p)
	}
	slices.Sort(a.paths)
	a.cutRoot()
	return a, nil
}

// clean is a name of the archive as a path within it, and whether it
// stays within it.
func clean(name string) (string, bool) {
	if name == "" || strings.ContainsRune(name, 0) || !utf8.ValidString(name) ||
		strings.HasPrefix(name, "/") || (len(name) > 1 && name[1] == ':') {
		return "", false
	}
	p := path.Clean(name)
	if p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return "", false
	}
	return p, true
}

// cutRoot drops the one folder that holds every file, as an archive made
// of a folder has: the folder's name names the import.
func (a *Archive) cutRoot() {
	if len(a.paths) == 0 {
		return
	}
	root, _, ok := strings.Cut(a.paths[0], "/")
	if !ok {
		return
	}
	for _, p := range a.paths {
		if !strings.HasPrefix(p, root+"/") {
			return
		}
	}
	files := map[string]*zip.File{}
	for i, p := range a.paths {
		a.paths[i] = strings.TrimPrefix(p, root+"/")
		files[a.paths[i]] = a.files[p]
	}
	a.files, a.Root = files, root
}

// Paths are the files of the archive, in the order of their paths.
func (a *Archive) Paths() []string { return slices.Clone(a.paths) }

// Has reports whether the archive holds a file at p.
func (a *Archive) Has(p string) bool { _, ok := a.files[p]; return ok }

// Read reads the file at p, at most limit bytes of it unpacked, and
// counts what it read against the archive's limit.
func (a *Archive) Read(p string, limit int64) ([]byte, error) {
	f, ok := a.files[p]
	if !ok {
		return nil, fmt.Errorf("noteimport: %s is not in the archive", p)
	}
	if f.UncompressedSize64 > uint64(limit) {
		return nil, fmt.Errorf("%w: %s has %d bytes, more than %d", ErrTooLarge, p, f.UncompressedSize64, limit)
	}
	if int64(f.UncompressedSize64) > a.left {
		return nil, fmt.Errorf("%w: the files read from it would hold more than its limit unpacked", ErrArchive)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("noteimport: %s: %v", p, err)
	}
	defer rc.Close()
	// The size the directory names may lie; what is read is bounded
	// either way.
	var buf bytes.Buffer
	n, err := io.Copy(&buf, io.LimitReader(rc, min(limit, a.left)+1))
	if err != nil {
		return nil, fmt.Errorf("noteimport: %s: %v", p, err)
	}
	if n > limit {
		return nil, fmt.Errorf("%w: %s has more than %d bytes", ErrTooLarge, p, limit)
	}
	if n > a.left {
		return nil, fmt.Errorf("%w: the files read from it would hold more than its limit unpacked", ErrArchive)
	}
	a.left -= n
	return buf.Bytes(), nil
}
