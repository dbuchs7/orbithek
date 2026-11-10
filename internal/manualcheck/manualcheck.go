// Package manualcheck holds the rules of Check B of ADR-0185: a pull
// request that changes what people see changes every user manual, or says
// in its description why it need not.
//
// A pull request changes what people see when it adds or changes a value
// of the source catalogue (locales/en.js), adds a route or a view to the UI
// shell (app.js), or adds a module of the UI. It passes when it changes
// every manual (manual/<lang>.md), or when its description holds a line of
// its own "Manual: not needed — <reason>". The rules judge by files and
// catalogue values, not by meaning; cmd/manualcheck reads them from git.
package manualcheck

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"
)

// The files the rules look at, relative to the repository's root.
const (
	Assets    = "internal/web/static/assets/"
	Catalogue = Assets + "locales/en.js"
	Shell     = Assets + "app.js"
	Manuals   = Assets + "manual/"
)

// Change is what a pull request changes, as far as the rules need it.
type Change struct {
	// OldCatalogue and NewCatalogue are the source catalogue at the base and
	// at the head; empty where the file does not exist.
	OldCatalogue, NewCatalogue string
	// OldShell and NewShell are app.js at the base and at the head.
	OldShell, NewShell string
	// Added lists the files the pull request adds; Changed every file it
	// adds, changes or removes.
	Added, Changed []string
	// Manuals lists the manuals at the head, one per UI language.
	Manuals []string
	// Body is the pull request's description.
	Body string
}

// Report is the outcome of the rules for one pull request.
type Report struct {
	// Keys are the catalogue keys whose value was added or changed.
	Keys []string
	// Routes are the route sections the shell gained; Views the view
	// functions it gained.
	Routes, Views []string
	// Modules are the UI modules added.
	Modules []string
	// Missing lists the manuals a pull request that changes what people see
	// leaves untouched.
	Missing []string
	// Reason is the reason given in the description, if any.
	Reason string
}

// Visible reports whether the pull request changes what people see.
func (r Report) Visible() bool {
	return len(r.Keys)+len(r.Routes)+len(r.Views)+len(r.Modules) > 0
}

// OK reports whether the pull request passes: it changes nothing people
// see, changes every manual, or gives a reason.
func (r Report) OK() bool {
	return !r.Visible() || len(r.Missing) == 0 || r.Reason != ""
}

// Evaluate applies the rules to a change.
func Evaluate(c Change) (Report, error) {
	var r Report
	oldCat, err := ParseCatalogue(c.OldCatalogue)
	if err != nil {
		return r, fmt.Errorf("the base's %s: %w", Catalogue, err)
	}
	newCat, err := ParseCatalogue(c.NewCatalogue)
	if err != nil {
		return r, fmt.Errorf("the head's %s: %w", Catalogue, err)
	}
	r.Keys = changedValues(oldCat, newCat)
	r.Routes = gained(Routes(c.OldShell), Routes(c.NewShell))
	r.Views = gained(Views(c.OldShell), Views(c.NewShell))
	for _, f := range c.Added {
		if IsModule(f) {
			r.Modules = append(r.Modules, f)
		}
	}
	slices.Sort(r.Modules)
	for _, m := range c.Manuals {
		if !slices.Contains(c.Changed, m) {
			r.Missing = append(r.Missing, m)
		}
	}
	slices.Sort(r.Missing)
	r.Reason = NotNeeded(c.Body)
	return r, nil
}

// ParseCatalogue reads a catalogue module, whose default export is strict
// JSON (ADR-0017). An empty text is an empty catalogue.
func ParseCatalogue(src string) (map[string]any, error) {
	if strings.TrimSpace(src) == "" {
		return map[string]any{}, nil
	}
	const export = "export default "
	start := strings.Index(src, export)
	end := strings.LastIndex(src, "};")
	if start < 0 || end < start {
		return nil, errors.New("no `export default {…};`")
	}
	var c map[string]any
	if err := json.Unmarshal([]byte(src[start+len(export):end+1]), &c); err != nil {
		return nil, fmt.Errorf("not strict JSON: %w", err)
	}
	return c, nil
}

// changedValues lists the keys whose value the new catalogue adds or
// changes; "$name", the language's own name, is none. A removed key is
// none either: a manual that still names its label fails Check A.
func changedValues(old, new map[string]any) []string {
	var out []string
	for key, v := range new {
		if strings.HasPrefix(key, "$") {
			continue
		}
		before, ok := old[key]
		if !ok || !sameValue(before, v) {
			out = append(out, key)
		}
	}
	slices.Sort(out)
	return out
}

func sameValue(a, b any) bool {
	ja, errA := json.Marshal(a)
	jb, errB := json.Marshal(b)
	return errA == nil && errB == nil && string(ja) == string(jb)
}

// routeRE finds a route of the shell: a section of the hash that the
// router's switch hands to a view, `case "album": return albumView();`.
var routeRE = regexp.MustCompile(`(?m)^\s*case "([^"]+)":\s*return\b[^;\n]*\b\w+(?:View|Screen|Overview)\(`)

// viewRE finds a view of the shell: a function whose name ends in View or
// Screen, the convention of app.js.
var viewRE = regexp.MustCompile(`(?m)^(?:async\s+)?function\s+(\w+(?:View|Screen))\s*\(`)

// Routes lists the route sections of a shell.
func Routes(shell string) []string { return matches(routeRE, shell) }

// Views lists the view functions of a shell.
func Views(shell string) []string { return matches(viewRE, shell) }

func matches(re *regexp.Regexp, text string) []string {
	set := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(text, -1) {
		set[m[1]] = true
	}
	return slices.Sorted(maps.Keys(set))
}

// gained lists what new has and old has not.
func gained(old, new []string) []string {
	var out []string
	for _, s := range new {
		if !slices.Contains(old, s) {
			out = append(out, s)
		}
	}
	return out
}

// IsModule reports whether a path is a module of the UI: a script directly
// in the assets directory. Catalogues, manuals, fonts, icons and palettes
// live in directories of their own and are none.
func IsModule(p string) bool {
	rest, ok := strings.CutPrefix(p, Assets)
	return ok && !strings.Contains(rest, "/") && path.Ext(rest) == ".js"
}

// IsManual reports whether a path is a user manual.
func IsManual(p string) bool {
	rest, ok := strings.CutPrefix(p, Manuals)
	return ok && !strings.Contains(rest, "/") && path.Ext(rest) == ".md"
}

// notNeededRE is the line by which a description says the manual needs no
// change: "Manual: not needed — <reason>", on a line of its own. An en
// dash or a hyphen stands for the dash, as keyboards type it.
var notNeededRE = regexp.MustCompile(`(?im)^[ \t]*Manual:[ \t]*not needed[ \t]*(?:—|–|-{1,2})[ \t]*(\S[^\r\n]*?)[ \t]*\r?$`)

// NotNeeded returns the reason of the description's "Manual: not needed"
// line, or "" when it has none or the line gives no reason.
func NotNeeded(body string) string {
	if m := notNeededRE.FindStringSubmatch(body); m != nil {
		return m[1]
	}
	return ""
}
