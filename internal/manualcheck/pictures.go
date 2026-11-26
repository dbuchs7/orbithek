package manualcheck

// Check C of ADR-0254: a pull
// request that changes what a picture of the manual shows takes the picture
// again, in every language, or says in its description why it need not.
//
// What a picture shows is written down beside the pictures, in
// manual/img/sources.txt: per picture the files, the top-level functions or
// constants of a file and the prefixes of catalogue keys that draw it. The
// rules compare these sources at the base and at the head; a picture whose
// source differs is stale unless the pull request changes its files.

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// The pictures and what they show.
const (
	Pictures       = Manuals + "img/"
	PictureSources = Pictures + "sources.txt"
)

// Picture is a line of sources.txt: a picture and what it shows.
type Picture struct {
	// Name is the picture without its language and extension: "tasks" for
	// img/<lang>/tasks.png in every language, "icons/shop" for the
	// language-neutral img/icons/shop.png.
	Name string
	// Sources are what draws it: a file of the assets directory ("shop.js"),
	// a top-level function or constant of one ("app.js#homeView") or a
	// prefix of catalogue keys ("key:shop.setup").
	Sources []string
}

// Neutral reports whether a picture is the same in every language.
func (p Picture) Neutral() bool { return strings.HasPrefix(p.Name, "icons/") }

// Files are the picture's files relative to the repository's root, one per
// language unless it is neutral.
func (p Picture) Files(langs []string) []string {
	if p.Neutral() {
		return []string{Pictures + p.Name + ".png"}
	}
	out := make([]string, 0, len(langs))
	for _, l := range langs {
		out = append(out, Pictures+l+"/"+p.Name+".png")
	}
	return out
}

var (
	pictureNameRE = regexp.MustCompile(`^(?:icons/)?[a-z0-9][a-z0-9-]*$`)
	sourceRE      = regexp.MustCompile(`^(?:key:[a-zA-Z0-9][a-zA-Z0-9._-]*|[a-z0-9][a-z0-9/_-]*\.(?:js|css)(?:#[A-Za-z_$][\w$]*)?)$`)
)

// ParseSources reads sources.txt: a picture per line, its name and then
// its sources separated by spaces; empty lines and lines starting with #
// are none.
func ParseSources(text string) ([]Picture, error) {
	var out []Picture
	seen := map[string]bool{}
	for i, line := range strings.Split(text, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 || strings.HasPrefix(f[0], "#") {
			continue
		}
		if !pictureNameRE.MatchString(f[0]) {
			return nil, fmt.Errorf("line %d: %q is no picture name", i+1, f[0])
		}
		if seen[f[0]] {
			return nil, fmt.Errorf("line %d: %s is listed twice", i+1, f[0])
		}
		seen[f[0]] = true
		if len(f) == 1 {
			return nil, fmt.Errorf("line %d: %s names nothing it shows", i+1, f[0])
		}
		for _, s := range f[1:] {
			if !sourceRE.MatchString(s) {
				return nil, fmt.Errorf("line %d: %q is no source (a file, file#name or key:prefix)", i+1, s)
			}
		}
		out = append(out, Picture{Name: f[0], Sources: f[1:]})
	}
	return out, nil
}

// SourceFile is the file of the assets directory a source reads, relative
// to the repository's root, or "" for a prefix of catalogue keys.
func SourceFile(source string) string {
	if strings.HasPrefix(source, "key:") {
		return ""
	}
	file, _, _ := strings.Cut(source, "#")
	return Assets + file
}

// Excerpt returns what a source names in its file's text: the whole text,
// or the top-level function or constant the source names after #, from its
// first line to the closing brace at the start of a line. It reports false
// when the text has no such function or constant.
func Excerpt(text, source string) (string, bool) {
	_, name, ok := strings.Cut(source, "#")
	if !ok {
		return text, true
	}
	start := regexp.MustCompile(`(?m)^(?:export\s+)?(?:async\s+)?(?:function\*?\s+|const\s+|let\s+)` + regexp.QuoteMeta(name) + `\b`)
	loc := start.FindStringIndex(text)
	if loc == nil {
		return "", false
	}
	rest := text[loc[0]:]
	if end := regexp.MustCompile(`(?m)^[}\]]`).FindStringIndex(rest); end != nil {
		rest = rest[:end[1]]
	}
	return rest, true
}

// PictureChange is what the picture rules need of a pull request.
type PictureChange struct {
	// Pictures is sources.txt at the head.
	Pictures []Picture
	// Old and New hold, per file a source reads, its text at the base and
	// at the head; a file missing at one of them is "".
	Old, New map[string]string
	// Keys are the catalogue keys whose value was added or changed.
	Keys []string
	// Langs are the manual's languages.
	Langs []string
	// Changed lists every file the pull request adds, changes or removes.
	Changed []string
}

// StalePicture is a picture whose sources changed but whose files did not.
type StalePicture struct {
	Name string
	// Because lists the sources that changed.
	Because []string
	// Files lists the picture's files the pull request leaves untouched.
	Files []string
}

// StalePictures lists the pictures a pull request changes what they show
// of, but does not take again.
func StalePictures(c PictureChange) []StalePicture {
	var out []StalePicture
	for _, p := range c.Pictures {
		var because []string
		for _, s := range p.Sources {
			if sourceChanged(c, s) {
				because = append(because, s)
			}
		}
		if len(because) == 0 {
			continue
		}
		var files []string
		for _, f := range p.Files(c.Langs) {
			if !slices.Contains(c.Changed, f) {
				files = append(files, f)
			}
		}
		if len(files) > 0 {
			out = append(out, StalePicture{Name: p.Name, Because: because, Files: files})
		}
	}
	return out
}

func sourceChanged(c PictureChange, s string) bool {
	if prefix, ok := strings.CutPrefix(s, "key:"); ok {
		return slices.ContainsFunc(c.Keys, func(k string) bool { return strings.HasPrefix(k, prefix) })
	}
	file := SourceFile(s)
	if !slices.Contains(c.Changed, file) {
		return false
	}
	before, okBefore := Excerpt(c.Old[file], s)
	after, okAfter := Excerpt(c.New[file], s)
	return okBefore != okAfter || before != after
}

// picturesNotNeededRE is the line by which a description says the pictures
// need not be taken again: "Pictures: not needed — <reason>".
var picturesNotNeededRE = regexp.MustCompile(`(?im)^[ \t]*Pictures:[ \t]*not needed[ \t]*(?:—|–|-{1,2})[ \t]*(\S[^\r\n]*?)[ \t]*\r?$`)

// PicturesNotNeeded returns the reason of the description's "Pictures: not
// needed" line, or "" when it has none.
func PicturesNotNeeded(body string) string {
	if m := picturesNotNeededRE.FindStringSubmatch(body); m != nil {
		return m[1]
	}
	return ""
}
