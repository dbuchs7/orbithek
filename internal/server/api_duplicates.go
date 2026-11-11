package server

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"sync"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/item"
	"github.com/dbuchs7/orbithek/internal/photo"
)

// duplicateRoutes are the routes that find duplicate pictures in the album
// (ADR-0195). Neither is an MCP tool: they serve the album's upload and
// its view of duplicates, both in the browser.
func duplicateRoutes(h itemHandlers) []Route {
	return []Route{
		{Method: http.MethodGet, Pattern: "/api/album/duplicates", Summary: "The pictures one may read that are the same file or look the same, in groups (same: every picture of the group is the same file), the newest group first, at most the budget PageItems groups; a copy and its original are no duplicates; space narrows it. pending counts the pictures not yet compared by how they look, of which one request reads at most the budget FingerprintReads", Handler: http.HandlerFunc(h.albumDuplicates)},
		{Method: http.MethodPost, Pattern: "/api/album/duplicates", Summary: "The pictures one may read that are duplicates of files about to be added: files, at most the budget DuplicateChecks, each with sha256 and, for a picture, fingerprint, width and height as the browser read them; answers matches per file, each a picture with same (the same file) or not (it looks the same), and pending as the GET does", Handler: http.HandlerFunc(h.checkDuplicates)},
	}
}

// fingerprints keeps the fingerprints made of pictures stored before
// fingerprints were recorded (ADR-0195), by their content's SHA-256; ""
// says one was tried and the picture has none. It holds 16 hexadecimal
// digits per picture of the store at most.
type fingerprints struct {
	mu   sync.Mutex
	made map[string]string
}

func (f *fingerprints) get(sum string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fp, ok := f.made[sum]
	return fp, ok
}

func (f *fingerprints) put(sum, fp string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.made == nil {
		f.made = map[string]string{}
	}
	f.made[sum] = fp
}

// fingerprintTypes are the pictures the server can decode for a
// fingerprint; WebP and videos have none and are compared as files only.
var fingerprintTypes = map[string]bool{"image/jpeg": true, "image/png": true, "image/gif": true}

// makeFingerprint decodes a stored picture, waiting for a free worker
// of the thumbnails, and gives its fingerprint; "" for a file that has
// none or cannot be read.
func (h itemHandlers) makeFingerprint(ctx context.Context, id item.ID, a item.Attachment) string {
	if !fingerprintTypes[a.Type] {
		return ""
	}
	p := h.photos()
	select {
	case p.working <- struct{}{}:
		defer func() { <-p.working }()
	case <-ctx.Done():
		return ""
	}
	c, err := h.d.DB.OpenAttachment(id, a.ID)
	if err != nil {
		return ""
	}
	defer c.Close()
	orientation := 0
	if a.Photo != nil {
		orientation = a.Photo.Orientation
	}
	fp, err := photo.Fingerprint(c, a.Type, orientation, budget.Get().ThumbnailPixels)
	if err != nil {
		h.log.Info("no fingerprint made", "item", id, "attachment", a.ID, "err", err)
		return ""
	}
	return fp
}

// duplicate is a picture the album compares: the top of a stack of
// versions, read with the caller's rights.
type duplicate struct {
	pic     albumPicture
	item    item.ID
	file    item.Attachment
	created int64
	// lineage holds the versions of the stack and the files they were
	// copied from: two pictures whose lineages meet are a copy and its
	// original (ADR-0055), never duplicates.
	lineage []item.ID
	// fp is the fingerprint, from the file or made since; w and h the
	// size turned upright, 0 when unknown.
	fp   string
	w, h int
}

// duplicateCandidates are the pictures the caller may read, one per stack.
func (h itemHandlers) duplicateCandidates(x *item.Session, space string) []duplicate {
	var out []duplicate
	for _, st := range albumStacks(x, albumFilter{space: space}) {
		top := st[0]
		d := duplicate{pic: h.albumPictureOf(top), item: top.Item.ID, file: top.Attachment, created: top.Attachment.Created, fp: top.Attachment.Fingerprint}
		taken := takenOf(top.Attachment)
		d.pic.Taken = &taken
		for _, v := range st {
			d.lineage = append(d.lineage, v.Attachment.ID)
			if v.Attachment.Source != nil {
				d.lineage = append(d.lineage, v.Attachment.Source.Attachment)
			}
		}
		if p := top.Attachment.Photo; p != nil && p.Width > 0 && p.Height > 0 {
			d.w, d.h = p.Width, p.Height
			if p.Orientation >= 5 && p.Orientation <= 8 {
				d.w, d.h = d.h, d.w
			}
		}
		out = append(out, d)
	}
	return out
}

// fill gives the candidates without a recorded fingerprint the one made
// before, and makes at most limit more; it answers how many remain.
func (h itemHandlers) fill(ctx context.Context, cands []duplicate, limit int) (pending int) {
	cache := &h.photos().fingerprints
	var todo []int
	for i := range cands {
		c := &cands[i]
		if c.fp != "" || !fingerprintTypes[c.file.Type] {
			continue
		}
		if fp, ok := cache.get(c.file.SHA256); ok {
			c.fp = fp
			continue
		}
		todo = append(todo, i)
	}
	pending = len(todo)
	if len(todo) > limit {
		todo = todo[:limit]
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, i := range todo {
		wg.Go(func() {
			c := &cands[i]
			fp := h.makeFingerprint(ctx, c.item, c.file)
			if ctx.Err() != nil {
				return
			}
			cache.put(c.file.SHA256, fp)
			mu.Lock()
			c.fp = fp
			pending--
			mu.Unlock()
		})
	}
	wg.Wait()
	return pending
}

// sameShape reports whether two pictures have the same proportions, as
// far as their sizes are known: a picture cut differently is no duplicate.
func sameShape(w1, h1, w2, h2 int) bool {
	if w1 <= 0 || h1 <= 0 || w2 <= 0 || h2 <= 0 {
		return true
	}
	a, b := float64(w1)/float64(h1), float64(w2)/float64(h2)
	return math.Abs(a-b)/max(a, b) <= 0.02
}

// related reports whether two pictures are a copy and its original.
func related(a, b duplicate) bool {
	for _, id := range a.lineage {
		if slices.Contains(b.lineage, id) {
			return true
		}
	}
	return false
}

// fingerprintParts splits a fingerprint into SimilarBits+1 parts: two
// fingerprints at most SimilarBits apart agree in at least one of them,
// so only pictures sharing a part need to be compared.
func fingerprintParts(fp string) []uint64 {
	v, err := strconv.ParseUint(fp, 16, 64)
	if err != nil {
		return nil
	}
	n := photo.SimilarBits + 1
	parts := make([]uint64, n)
	shift := 0
	for i := range n {
		width := 64 / n
		if i < 64%n {
			width++
		}
		parts[i] = uint64(i)<<56 | (v>>shift)&(1<<width-1)
		shift += width
	}
	return parts
}

// groupDuplicates joins the candidates that are the same file or look the
// same into groups of two and more.
func groupDuplicates(cands []duplicate) [][]int {
	parent := make([]int, len(cands))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	join := func(i, j int) {
		if !related(cands[i], cands[j]) {
			parent[find(i)] = find(j)
		}
	}
	bySum := map[string][]int{}
	byPart := map[uint64][]int{}
	for i, c := range cands {
		bySum[c.file.SHA256] = append(bySum[c.file.SHA256], i)
		if c.fp != "" {
			for _, p := range fingerprintParts(c.fp) {
				byPart[p] = append(byPart[p], i)
			}
		}
	}
	for _, same := range bySum {
		for _, j := range same[1:] {
			join(same[0], j)
		}
	}
	for _, near := range byPart {
		for x, i := range near {
			for _, j := range near[x+1:] {
				if find(i) != find(j) && lookAlike(cands[i], cands[j]) {
					join(i, j)
				}
			}
		}
	}
	at := map[int]int{}
	var groups [][]int
	for i := range cands {
		r := find(i)
		g, ok := at[r]
		if !ok {
			g = len(groups)
			at[r] = g
			groups = append(groups, nil)
		}
		groups[g] = append(groups[g], i)
	}
	return slices.DeleteFunc(groups, func(g []int) bool { return len(g) < 2 })
}

// lookAlike reports whether two pictures look the same: their
// fingerprints are near and their proportions equal.
func lookAlike(a, b duplicate) bool {
	return photo.LookAlike(a.fp, b.fp) && sameShape(a.w, a.h, b.w, b.h)
}

// duplicateJSON is a picture found as a duplicate; same says it is the
// same file, else it looks the same.
type duplicateJSON struct {
	albumPicture
	Same bool `json:"same"`
}

// albumDuplicates answers the groups of duplicate pictures (ADR-0195).
func (h itemHandlers) albumDuplicates(w http.ResponseWriter, r *http.Request) {
	var cands []duplicate
	h.d.DB.Read(func(s *item.Store) { cands = h.duplicateCandidates(s.As(who(r)), r.URL.Query().Get("space")) })
	// Fingerprints are made outside the store's lock: decoding takes long.
	pending := h.fill(r.Context(), cands, max(0, budget.Get().FingerprintReads))
	groups := groupDuplicates(cands)
	newest := func(g []int) int64 {
		var n int64
		for _, i := range g {
			n = max(n, cands[i].created)
		}
		return n
	}
	for _, g := range groups {
		slices.SortStableFunc(g, func(a, b int) int { return cmp.Compare(cands[a].created, cands[b].created) })
	}
	slices.SortStableFunc(groups, func(a, b []int) int { return cmp.Compare(newest(b), newest(a)) })
	total := len(groups)
	groups = groups[:min(len(groups), max(1, budget.Get().PageItems))]
	out := []map[string]any{}
	for _, g := range groups {
		same := true
		pics := []duplicateJSON{}
		for _, i := range g {
			same = same && cands[i].file.SHA256 == cands[g[0]].file.SHA256
			pics = append(pics, duplicateJSON{albumPicture: cands[i].pic})
		}
		for k, i := range g {
			for _, j := range g {
				if i != j && cands[i].file.SHA256 == cands[j].file.SHA256 {
					pics[k].Same = true
				}
			}
		}
		out = append(out, map[string]any{"same": same, "pictures": pics})
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": out, "total": total, "pending": pending, "similarBits": photo.SimilarBits})
}

// fileCheck is a file about to be added, as the browser read it.
type fileCheck struct {
	SHA256      string `json:"sha256"`
	Fingerprint string `json:"fingerprint"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// checkDuplicates answers which readable pictures are duplicates of files
// about to be added (ADR-0195).
func (h itemHandlers) checkDuplicates(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Files []fileCheck `json:"files"`
	}
	if !decode(w, r, &req) {
		return
	}
	if limit := budget.Get().DuplicateChecks; len(req.Files) > limit {
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("at most %d files are checked at once", limit))
		return
	}
	for i, f := range req.Files {
		if !sha256Hex.MatchString(f.SHA256) || (f.Fingerprint != "" && !photo.ValidFingerprint(f.Fingerprint)) || f.Width < 0 || f.Height < 0 {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("file %d: sha256 must be 64 and a fingerprint 16 lower-case hexadecimal digits", i+1))
			return
		}
	}
	var cands []duplicate
	h.d.DB.Read(func(s *item.Store) { cands = h.duplicateCandidates(s.As(who(r)), "") })
	pending := 0
	if slices.ContainsFunc(req.Files, func(f fileCheck) bool { return f.Fingerprint != "" }) {
		pending = h.fill(r.Context(), cands, max(0, budget.Get().FingerprintReads))
	}
	const perFile = 10
	matches := make([][]duplicateJSON, len(req.Files))
	for i, f := range req.Files {
		mine := duplicate{fp: f.Fingerprint, w: f.Width, h: f.Height}
		found := []duplicateJSON{}
		for _, c := range cands {
			switch {
			case c.file.SHA256 == f.SHA256:
				found = append(found, duplicateJSON{albumPicture: c.pic, Same: true})
			case f.Fingerprint != "" && lookAlike(mine, c):
				found = append(found, duplicateJSON{albumPicture: c.pic})
			}
		}
		slices.SortStableFunc(found, func(a, b duplicateJSON) int {
			if a.Same != b.Same {
				if a.Same {
					return -1
				}
				return 1
			}
			return 0
		})
		matches[i] = found[:min(len(found), perFile)]
	}
	writeJSON(w, http.StatusOK, map[string]any{"matches": matches, "pending": pending, "similarBits": photo.SimilarBits})
}
