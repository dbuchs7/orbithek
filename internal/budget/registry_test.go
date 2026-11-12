package budget

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ceilingCalls are the calls that bound how much input the process reads,
// with the index of the argument that carries the bound.
var ceilingCalls = map[string]int{
	"io.LimitReader":      1,
	"io.CopyN":            2,
	"http.MaxBytesReader": 2,
}

// classified are the ceilings that are deliberately not budgets, each with
// its reason. A key is "<package dir>.<name>" for a package-level constant
// or variable, or "<file>:<bound as printed>" for a ceiling call.
var classified = map[string]string{
	"internal/export.maxEmbedDepth": "the shape of an export (ADR-0071): how deep embedded items are written inside each other, " +
		"like a heading level; what an export gathers is bounded by PageItems",
	"internal/site.MaxBackupVersions": "a retention policy (ADR-0073): how many backup versions an administrator may keep " +
		"at the destination, not something the server holds in memory",
	"internal/backup/dest.maxListing": "a backup destination's listing or error answer (ADR-0073), read and parsed; " +
		"not a caller's input to this server, it guards against a destination that misbehaves",
	"internal/backup/dest/remote.go:2048": "the start of a destination's error answer, quoted in the error message (ADR-0073)",
	"internal/item.maxURL":                "the shape of a url field (ADR-0074), like the length of a username; what a request carries is bounded by RequestBody",
	"internal/connector.maxAnswer": "the answer of the catalog source a connector reads (ADR-0075, ADR-0077), not a caller's input to this server; " +
		"it guards the connector against a source that misbehaves",
	"internal/connector/ckan.go:maxAnswer": "the same bound, where the answer is read",
	"internal/connector.maxSpec": "the size of an API description a connector reads (ADR-0130), not a caller's input to this server; " +
		"it guards the server against a description that misbehaves",
	"internal/connector.maxLists": "how many lists an API description offers to choose from (ADR-0130), the length of a list on a page",
	"internal/connector.maxMarkdown": "a Markdown file of a Git repository a page is written from (ADR-0138), not a caller's input to this server; " +
		"it guards against a file that would make a page of megabytes",
	"internal/connector.maxGitFiles": "the size of one Git connector's run (ADR-0138), so a repository with a huge folder cannot make it endless; " +
		"not memory a caller of this server can claim",
	"internal/connector.maxPictures": "the shape of a page written from a Git repository (ADR-0138): how many pictures it keeps; " +
		"each picture is bounded by AttachmentBytes",
	"internal/noteimport.maxNote": "the shape of a page written from an imported note (ADR-0211), like a Markdown file of a repository; " +
		"what an archive makes the server read is bounded by NoteUnpackedBytes",
	"internal/noteimport.maxTable": "the shape of a database of an imported workspace (ADR-0211): the CSV file its rows are read from; " +
		"what an archive makes the server read is bounded by NoteUnpackedBytes",
	"internal/noteimport.maxPictures": "the shape of a page written from an imported note (ADR-0211): how many pictures it keeps; " +
		"each picture is bounded by AttachmentBytes",
	"internal/gitrepo.maxListing": "an answer of the API of a Git host the server reads (ADR-0138), not a caller's input to this server; " +
		"it guards against a host that misbehaves",
	"internal/gitrepo/gitrepo.go:maxListing + 1": "the same bound, where the answer is read",
	"internal/gitrepo.maxListPages": "how many pages of a Git host's listing are followed (ADR-0138), not a caller's input to this server; " +
		"it guards against a host that names a next page for ever",
	"internal/connector.maxPages": "how many pages of a JSON source's list a connector reads (ADR-0127, ADR-0135), not a caller's input to this server; " +
		"it guards the connector against a source that names a next page for ever",
	"internal/item.maxSharePath": "the shape of an entry of a file share (ADR-0197): how long the path it names may be, as the share's own path bound; " +
		"what a request carries is bounded by RequestBody",
	"internal/item.maxShareTag": "the shape of an entry of a file share (ADR-0197): how long the tag of its content (an ETag, a time) may be",
	"internal/photo.maxEmbeddedCandidates": "the shape of a camera's RAW as read for its preview: how many starts of a JPEG are looked at; " +
		"what the RAW holds is bounded by SharePreviewBytes",
	"internal/share.MaxPath": "the shape of a path below a file share's address (ADR-0197), like the length of a file name; " +
		"a run's size is bounded by ShareFiles",
	"internal/share.maxListingPage": "a page of an S3 share's listing (ADR-0197), read and parsed; not a caller's input to this server, " +
		"it guards against a store that misbehaves",
	"internal/share/s3.go:maxListingPage": "the same bound, where the page is read",
	"internal/share.maxErrorAnswer":       "an S3 store's error answer (ADR-0197), read for its code only; it guards against a store that misbehaves",
	"internal/share/s3.go:maxErrorAnswer": "the same bound, where the answer is read",
	"internal/connector.maxCell":          "the shape of a generated catalog page (ADR-0080): how long one value of the source may be on it, like the length of a title",
	"internal/site.maxConnectors":         "the shape of the site's settings (ADR-0077): how many connectors an administrator keeps, like the number of regulations",
	"internal/site.maxEngines":            "the shape of the site's settings (ADR-0129): how many process engines an administrator keeps, like the number of connectors",
	"internal/site.maxShares":             "the shape of the site's settings (ADR-0197): how many file shares an administrator binds, like the number of connectors",
	"internal/site.maxWordTemplates":      "the shape of the site's settings (ADR-0167): how many Word templates an administrator keeps, like the number of engines",
	"internal/wordtemplate.maxParts": "the shape of a Word template (ADR-0167): how many parts its package holds, like the part names it may use; " +
		"what the parts hold is bounded by WordTemplateBytes",
	"internal/wordtemplate.maxFrameNodes": "the shape of a Word template's page as the reading view draws it (ADR-0167), " +
		"from a file an administrator uploaded and bounded by WordTemplateBytes",
	"internal/tasklink.maxProblems": "the shape of a round's report (ADR-0151): how many problems it names before it counts the rest, held for one engine at a time",
	"internal/site.maxTaskRules":    "the shape of the site's settings (ADR-0151): how many task link rules an administrator keeps per engine, like the number of engines",
	"internal/tasklink.maxEndReads": "how many tasks that left the list one round asks the engine about (ADR-0152), so many at once are settled over several rounds; " +
		"each question is one bounded read of the engine, not memory a caller of this server can claim",
	"internal/tasklink.maxFactPages": "how many pages of an engine's feed one round reads (ADR-0151), so a long feed is read over several rounds; " +
		"each page is bounded by EngineResponseBytes and applied before the next is read, not memory a caller of this server can claim",
	"internal/connector.maxDatasets": "the size of one connector's run (ADR-0075), so a misbehaving source cannot make it endless; " +
		"not memory a caller of this server can claim",
	"internal/item.maxColumnWidth":                     "the shape of a table (ADR-0214): how wide a person may drag a column, a size on the screen, not memory a caller claims",
	"internal/item.maxRowHeight":                       "the shape of a table (ADR-0214): how tall a person may drag a row, a size on the screen, not memory a caller claims",
	"internal/item.maxColumns":                         "the shape of a kanban (ADR-0076), like the length of its name; a definition of the space, not memory a caller claims",
	"internal/item.maxCardFields":                      "the shape of a kanban's cards (ADR-0076): how many fields a card shows",
	"internal/item.maxColumnLimit":                     "a sanity bound of a column's limit of cards (ADR-0134): a setting a person types, not memory a caller claims",
	"internal/item.maxSpaceName":                       "the shape of a space (ADR-0086): the length of its display name, like the length of a title",
	"internal/item.maxSecretReaders":                   "the shape of a protected passage (ADR-0088): how many people and groups it names, like the lists of a restriction",
	"internal/item.maxPictureLabels":                   "the shape of a picture (ADR-0090): how many labels it carries, a list a person types, not a memory setting",
	"internal/item.MaxFaces":                           "the shape of a picture (ADR-0194): how many faces it shows, like the labels it carries, not a memory setting",
	"internal/item.MaxRating":                          "the scale of a rating field (ADR-0202): one to five stars, a property of the field kind, not memory a caller claims",
	"internal/item.maxPersonRunes":                     "the shape of a face (ADR-0194): the length of a person's name, like the length of a title",
	"internal/server.maxKeptVectors":                   "a bound of the face vectors kept in memory for suggestions (ADR-0194), at most a few hundred bytes a picture; past it they are read again, so nothing a caller claims grows with it",
	"internal/storage.maxFaceVectorBytes":              "the size of a file of face vectors (ADR-0194), fixed by MaxFaces and the 128 numbers of a face; written by this server, not a caller's upload",
	"internal/storage/faces.go:maxFaceVectorBytes + 1": "the same bound, where the file is read",
	"internal/item.MaxPrefill":                         "the shape of a start form (ADR-0148): how many of its fields it fills in beforehand, a list a person chooses, like the labels of a picture",
	"internal/item.MaxViewColumns":                     "the shape of a view in a text (ADR-0203): how many columns it names, a list a person chooses, like the fields a start form fills in",
	"internal/item.maxViewName":                        "the shape of a view in a text (ADR-0203): the length of the name of a type or field it names, like the length of a title",
	"internal/engine/grant.go:4096": "an engine's answer that a task ended (ADR-0152), a small fixed object read only for the word it says; " +
		"not memory a caller of this server can claim",
	"internal/engine/engine.go:4096": "the start of an engine's refusal of a start link's submission, quoted in the error message (ADR-0148); " +
		"not a caller's input to this server, the whole answer is bounded by EngineResponseBytes elsewhere",
	"internal/item.maxRetentionKeep":                   "a retention policy (ADR-0104): how many versions of a picture a rule may keep, not something the server holds in memory",
	"internal/item.maxRetentionDays":                   "a retention policy (ADR-0104): how many days a rule may keep versions of a picture",
	"internal/notes.maxNoteText":                       "the shape of a personal note (ADR-0107): how long its text may be, like the length of a comment",
	"internal/notes.maxNoteQuote":                      "the shape of a personal note (ADR-0107): how long the passage it names may be",
	"internal/notes.maxNotesItem":                      "the shape of a person's notes on one item (ADR-0107), like the labels of a picture; a request is bounded by RequestBody",
	"internal/auth.MaxFailures":                        "a security policy (ADR-0013): failed logins before the throttle, not a memory setting",
	"internal/auth.MaxTokenLifetime":                   "a security policy (ADR-0027): how long an API token may live",
	"internal/auth.MaxReasonLength":                    "the shape of an account request (ADR-0061), like the length of a username; the waiting requests are bounded by AccountRequests",
	"internal/auth.maxCalendarViews":                   "the shape of one's preferences (ADR-0168), like the number of favourites: views are kept whole in each journal record, and what a request carries is bounded by RequestBody",
	"internal/auth.maxViewName":                        "the shape of a saved view (ADR-0168), like the length of a username",
	"internal/auth.maxViewParams":                      "the shape of a saved view (ADR-0168): the parameters of a calendar's address, which hold a few choices and a query",
	"internal/item.maxInterval":                        "the shape of a rule (ADR-0168), like the values of an enumeration: every 999 periods is longer than any calendar shows",
	"internal/item.maxCount":                           "the shape of a rule (ADR-0168): how many times it repeats, which bounds the occurrences it gives",
	"internal/item.maxExceptions":                      "the shape of a rule (ADR-0168): its cancelled and moved occurrences are kept in the field's text, which every listing carries",
	"internal/item.maxSteps":                           "a guard of the walk through a rule's days (ADR-0168), not a choice: the walk ends a year after the stretch asked for, far below it",
	"internal/photo.maxAltitude":                       "the shape of photo data (ADR-0177): a height no photo is taken at is no height",
	"internal/photo.maxEntries":                        "the shape of an EXIF directory (ADR-0177): more entries than any camera writes mark a damaged file, read from a file already bounded by AttachmentBytes",
	"internal/photo.maxVideoBoxes":                     "the shape of a video's boxes (ADR-0189): more boxes than any camera writes mark a damaged file; each box is visited by seeking, not held",
	"internal/photo.maxVideoDepth":                     "the shape of a video's boxes (ADR-0189): how deep the boxes the album reads are nested",
	"internal/photo.maxAudioElements":                  "the shape of a WebM file's elements (ADR-0219): more elements before the first Cluster than any recorder writes mark a damaged file; each is visited by seeking, not held",
	"internal/photo.maxAudioDepth":                     "the shape of a WebM file's elements (ADR-0219): how deep the Tracks element is nested",
	"internal/photo.maxVideoBoxRead":                   "the shape of a small box of a video's header (ADR-0189), read whole; larger boxes are skipped, never read",
	"internal/photo.maxExifBytes":                      "the shape of a picture's EXIF chunk (ADR-0177), read from a file already bounded by AttachmentBytes",
	"internal/photo.maxExposureSecs":                   "the shape of photo data (ADR-0177): an exposure no camera takes is no exposure",
	"internal/photo.maxFill":                           "a guard of reading a JPEG's markers (ADR-0177), read from a file already bounded by AttachmentBytes",
	"internal/photo.maxOffsetHours":                    "the shape of a time zone offset (ADR-0177), like the length of a username",
	"internal/photo.maxPNGChunks":                      "a guard of walking a PNG's chunks (ADR-0177), so a damaged file cannot make it endless",
	"internal/photo.maxSegments":                       "a guard of walking a JPEG's segments (ADR-0177), so a damaged file cannot make it endless",
	"internal/photo.maxAPP1":                           "a file-format limit: the most a JPEG segment holds after its length, so a time set by hand is not written into an Exif segment that cannot hold it (ADR-0198)",
	"internal/photo.maxTIFFBytes":                      "the shape of a picture's EXIF chunk as Read takes it (ADR-0177), written by SetTaken into a file already bounded by AttachmentBytes (ADR-0198)",
	"internal/item.maxTakenYear":                       "a validation rule: the last year a time set by hand may lie in, the years the EXIF reader takes (ADR-0177, ADR-0198)",
	"internal/photo.maxStringRunes":                    "the shape of photo data (ADR-0177): a camera's name, like the length of a title",
	"internal/photo.maxWebPChunks":                     "a guard of walking a WebP's chunks (ADR-0177), so a damaged file cannot make it endless",
	"internal/photosource.maxPending":                  "a guard of sign-ins at Google in progress (ADR-0177), each forgotten after ten minutes, as engine and calendar connections are",
	"internal/photosource.maxSessions":                 "the shape of one person's open pickers at Google (ADR-0177); the server's bound across everyone is the budget PhotoImports",
	"internal/photosource.maxItems":                    "a guard of reading Google's answer (ADR-0177): Google's own picker hands out at most 2000 items",
	"internal/photosource.maxItemPages":                "a guard of reading Google's pages (ADR-0177), so a service that names a next page for ever cannot make it endless",
	"internal/photosource.maxAnswer":                   "an answer of another photo service (ADR-0177), not a caller's input to this server; it guards against a service that misbehaves",
	"internal/photosource.maxAlbumAnswer":              "the answer of a shared iCloud album (ADR-0177), not a caller's input to this server; it guards against a service that misbehaves",
	"internal/photosource.maxPhotos":                   "the shape of a shared iCloud album (ADR-0177): Apple's own limit of photos in one",
	"internal/photosource/body.go:drainBytes":          "the start of another service's refusal (ADR-0177), read only to be dropped so the connection is reused",
	"internal/server.maxTileBytes":                     "a tile of the tile service (ADR-0177), not a caller's input to this server; a larger answer is no tile",
	"internal/server/api_photos.go:maxTileBytes + 1":   "the same bound, where the tile is read",
	"internal/server.maxPlaceAnswer":                   "an answer of the place search (ADR-0181), not a caller's input to this server; it lists a few places, and a larger answer is none",
	"internal/server/api_geotag.go:maxPlaceAnswer + 1": "the same bound, where the answer is read",
	"internal/server.maxTileZoom":                      "the shape of the album's map (ADR-0177): the deepest zoom tile services draw",
	"internal/extcal.maxSources":                       "the shape of one's preferences (ADR-0174), like the number of saved views: the calendars one shows beside the wiki's",
	"internal/extcal.maxAccounts":                      "the shape of one's preferences (ADR-0174): the accounts at Google or CalDAV servers one connects",
	"internal/extcal.maxName":                          "the shape of a calendar (ADR-0174), like the length of a saved view's name",
	"internal/extcal.maxSecret":                        "the shape of a credential (ADR-0174): an address, refresh token or password, far longer than any service hands out",
	"internal/extcal.maxEntries":                       "a guard of reading another service's calendar (ADR-0174): the entries kept of one answer, which maxBody bounds already",
	"internal/extcal.maxLine":                          "part of reading iCalendar (ADR-0174): the longest unfolded line, within an answer maxBody bounds",
	"internal/extcal.maxPeriods":                       "a guard of the walk through a repeating entry's periods (ADR-0174), not a choice: daily since 1970 ends far below it",
	"internal/extcal.maxOccurrences":                   "a guard of what a window shows of one repeating entry of another service (ADR-0174), not a choice",
	"internal/extcal.maxBody":                          "a guard of reading another service's answer (ADR-0174): one calendar, read at most four at once, every 15 minutes",
	"internal/extcal/reader.go:maxBody + 1":            "the same bound, where the answer is read",
	"internal/extcal/reader.go:64 << 10":               "the start of a refusal, read only to be dropped",
	"internal/extcal.maxPages":                         "a guard of reading Google's pages (ADR-0174): ten pages of 2,500 events of one month and its edges",
	"internal/extcal.maxPending":                       "a guard of begun Google connections (ADR-0174), as EngineConnectsPending is for engines: each expires after ten minutes",
	"internal/extcal.maxRedirects":                     "a guard of following a calendar service's redirects (ADR-0174), not a choice",
	"internal/server.maxEventDays":                     "the shape of a request for another service's entries (ADR-0174): a calendar shows at most a few weeks",
	"internal/auth.maxFavourites": "the shape of one's preferences (ADR-0145), like the length of a username: favourites are space keys, " +
		"which only an administrator creates, so the list cannot outgrow the spaces",
	"internal/wal.MaxRecord": "part of the event log format: a server must read every record a server wrote, " +
		"so the bound may not differ between them",
	"internal/backup.maxIterations": "part of the backup format: the work factor a header may demand " +
		"must not depend on the restoring server's environment",
	"internal/backup.maxManifest": "part of the backup format: the manifest of a backup, which lists every file, " +
		"read by the restore command from its own format",
	"internal/backup/backup.go:maxManifest": "the same bound, where the manifest is read",
	"cmd/settingscheck/main.go:4 << 20":     "a command-line client reading GitHub's answer, not this server's input",
	"cmd/adrnumbers/main.go:8 << 20":        "a command-line client reading GitHub's answer, not this server's input",
	"cmd/adrnumbers.maxPages":               "how many pages of a GitHub listing a command-line client follows, not this server's input; it guards CI against an API that names a next page for ever",
	"internal/mcp.maxAnswer": "the MCP adapter reading an answer of the API it calls; the API's own budgets bound " +
		"what it sends, this only guards the adapter against a server that misbehaves",
	"internal/mcp/client.go:maxAnswer": "the same bound, where the adapter reads",
	"internal/migrate.maxSourceAnswer": "the import reading an answer of the source site (ADR-0023), " +
		"not a caller's input to this server; it guards the import against a source that misbehaves",
	"internal/notify.maxHookAnswer": "the answer of a webhook a message was posted to (ADR-0064), read and dropped; " +
		"not a caller's input to this server",
	"internal/notify/hooks.go:maxHookAnswer":    "the same bound, where the answer is dropped",
	"internal/migrate/cloud.go:maxSourceAnswer": "the same bound, where the import reads (ADR-0023)",
	"internal/linkpreview.maxField": "the shape of a web link's fields (ADR-0208): how long one value a page names may be, like the length of a title; " +
		"the page itself is bounded by LinkPreviewPageBytes",
}

// TestNoCeilingWithoutAName keeps the budgets complete (ADR-0015). Every
// package-level constant or variable named max… and every bound passed to a
// reading ceiling is either read from Budgets or classified above with the
// reason. A new limit therefore cannot hide in a package: it becomes a
// budget, or its exemption is written down.
func TestNoCeilingWithoutAName(t *testing.T) {
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()
	seen := map[string]bool{}
	var unnamed []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "testdata", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		pkg := filepath.ToSlash(filepath.Dir(rel))
		consts := map[string]bool{}
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || (gd.Tok != token.CONST && gd.Tok != token.VAR) {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range vs.Names {
					consts[name.Name] = true
					if strings.HasPrefix(strings.ToLower(name.Name), "max") {
						key := pkg + "." + name.Name
						seen[key] = true
						if classified[key] == "" {
							unnamed = append(unnamed, key)
						}
					}
				}
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			x, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			arg, ok := ceilingCalls[x.Name+"."+sel.Sel.Name]
			if !ok || arg >= len(call.Args) || !fixed(call.Args[arg], consts) {
				return true
			}
			var b strings.Builder
			printer.Fprint(&b, fset, call.Args[arg])
			key := rel + ":" + b.String()
			seen[key] = true
			if classified[key] == "" {
				unnamed = append(unnamed, key)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(unnamed)
	for _, key := range unnamed {
		t.Errorf("%s is a ceiling without a name: make it a field of Budgets, or classify it with the reason", key)
	}
	for key := range classified {
		if !seen[key] {
			t.Errorf("classified %s no longer exists; remove it from the list", key)
		}
	}
}

// fixed reports whether a bound is chosen in the file itself — a literal,
// or a package-level name — rather than read from Budgets or passed in.
func fixed(e ast.Expr, consts map[string]bool) bool {
	switch e := e.(type) {
	case *ast.BasicLit:
		return true
	case *ast.Ident:
		return consts[e.Name]
	case *ast.BinaryExpr:
		return fixed(e.X, consts) && fixed(e.Y, consts)
	case *ast.ParenExpr:
		return fixed(e.X, consts)
	}
	return false
}
