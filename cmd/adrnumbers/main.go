// Command adrnumbers warns when a pull request adds an architecture
// decision record under a number that main or another open pull request
// already uses. It is run by CI on pull requests:
//
//	go run ./cmd/adrnumbers -repo owner/name -pr N [-base main] [-advisory] [-api URL]
//
// Records are numbered without gaps (docs/adr), so two pull requests that
// each add a record while both are open take the same number. The tests in
// docs/adr find the clash only once one of them is merged and the other is
// brought up to date with main; this command finds it while both are open,
// so the one merged second can renumber early.
//
// It reads, through the REST API, the files of every open pull request
// into the base branch and the records on the base branch. A number this
// pull request adds clashes when the base branch holds a record of that
// number under another name, or when another open pull request adds one.
// For each clash it says which pull request was opened first and the next
// number free of the base branch and every open pull request.
//
// GITHUB_TOKEN, if set, authenticates the requests; the workflow's token
// needs "pull-requests: read" and "contents: read".
//
// With -advisory a clash, and a check that could not run, become warning
// annotations and the command exits 0. Exit status otherwise: 0 without a
// clash, 1 with one, 2 when the check could not run.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

func main() {
	cfg := config{Token: os.Getenv("GITHUB_TOKEN"), Actions: os.Getenv("GITHUB_ACTIONS") == "true"}
	flag.StringVar(&cfg.Repo, "repo", os.Getenv("GITHUB_REPOSITORY"), "repository as owner/name")
	flag.IntVar(&cfg.PR, "pr", 0, "number of the pull request to check")
	flag.StringVar(&cfg.Base, "base", "main", "branch the pull requests merge into")
	flag.StringVar(&cfg.API, "api", "https://api.github.com", "base URL of the GitHub REST API")
	flag.BoolVar(&cfg.Advisory, "advisory", false, "report clashes as warnings and exit 0")
	flag.Parse()
	if cfg.Repo == "" || cfg.PR <= 0 || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: adrnumbers -repo owner/name -pr N [-base main] [-advisory] [-api URL]")
		os.Exit(2)
	}
	os.Exit(run(cfg, http.DefaultClient, os.Stdout))
}

type config struct {
	Repo, Base, API, Token string
	PR                     int
	Actions                bool // emit GitHub Actions annotations
	Advisory               bool // a clash or failure warns and exits 0
}

// recordRE matches a decision record's path and captures its number.
var recordRE = regexp.MustCompile(`^docs/adr/(\d{4})-[^/]+\.md$`)

// numberOf is the number of a decision record's path, "" for any other
// file.
func numberOf(path string) string {
	if m := recordRE.FindStringSubmatch(path); m != nil {
		return m[1]
	}
	return ""
}

// pull is an open pull request with the records it adds, by number.
type pull struct {
	Number  int
	Title   string
	Created time.Time
	Adds    map[string]string // record number → path
}

// clash is a record number this pull request adds that is taken elsewhere.
type clash struct {
	Number string
	Path   string // this pull request's record
	OnBase string // the base branch's record of that number, if any
	Other  *pull  // another open pull request adding that number, if any
}

// clashes compares the records own adds with the base branch's records
// (number → path) and those the other open pull requests add.
func clashes(own pull, base map[string]string, others []pull) []clash {
	var out []clash
	for _, n := range sortedKeys(own.Adds) {
		path := own.Adds[n]
		if b, ok := base[n]; ok && b != path {
			out = append(out, clash{Number: n, Path: path, OnBase: b})
		}
		for i := range others {
			if others[i].Number == own.Number {
				continue
			}
			if _, ok := others[i].Adds[n]; ok {
				out = append(out, clash{Number: n, Path: path, Other: &others[i]})
			}
		}
	}
	return out
}

// nextFree is the number after the highest one the base branch holds or
// any open pull request adds.
func nextFree(base map[string]string, pulls []pull) string {
	high := 0
	note := func(n string) {
		if v, err := strconv.Atoi(n); err == nil && v > high {
			high = v
		}
	}
	for n := range base {
		note(n)
	}
	for _, p := range pulls {
		for n := range p.Adds {
			note(n)
		}
	}
	return fmt.Sprintf("%04d", high+1)
}

// message says what a clash means and what to do about it.
func message(c clash, own pull, free string) string {
	if c.Other == nil {
		return fmt.Sprintf("ADR-%s is already taken on the base branch by %s. Renumber this record; the next number free of the base branch and every open pull request is ADR-%s.",
			c.Number, c.OnBase, free)
	}
	first := "this pull request was opened first"
	if c.Other.Created.Before(own.Created) || (c.Other.Created.Equal(own.Created) && c.Other.Number < own.Number) {
		first = fmt.Sprintf("#%d was opened first", c.Other.Number)
	}
	return fmt.Sprintf("ADR-%s is also added by the open pull request #%d (%s); %s. Whichever is merged second must renumber; the next number free of the base branch and every open pull request is ADR-%s.",
		c.Number, c.Other.Number, c.Other.Title, first, free)
}

// run performs the check and returns the exit status.
func run(cfg config, client *http.Client, out io.Writer) int {
	g := &github{cfg: cfg, client: client}
	base, err := g.baseRecords()
	var pulls []pull
	if err == nil {
		pulls, err = g.openPulls()
	}
	if err != nil {
		fmt.Fprintln(out, "adrnumbers:", err)
		if cfg.Actions {
			level := "error"
			if cfg.Advisory {
				level = "warning"
			}
			fmt.Fprintf(out, "::%s title=ADR number check could not run::%s\n", level, escape(err.Error()))
		}
		if cfg.Advisory {
			return 0
		}
		return 2
	}
	var own *pull
	for i := range pulls {
		if pulls[i].Number == cfg.PR {
			own = &pulls[i]
		}
	}
	if own == nil {
		// Closed, merged, or into another branch: nothing to compare.
		fmt.Fprintf(out, "pull request #%d is not open against %s; nothing to check\n", cfg.PR, cfg.Base)
		return 0
	}
	if len(own.Adds) == 0 {
		fmt.Fprintln(out, "this pull request adds no decision record")
		return 0
	}
	found := clashes(*own, base, pulls)
	if len(found) == 0 {
		fmt.Fprintf(out, "ADR %s: no other open pull request and not the base branch use these numbers\n", strings.Join(sortedKeys(own.Adds), ", "))
		return 0
	}
	free := nextFree(base, pulls)
	for _, c := range found {
		text := message(c, *own, free)
		fmt.Fprintln(out, text)
		if cfg.Actions {
			level := "error"
			if cfg.Advisory {
				level = "warning"
			}
			fmt.Fprintf(out, "::%s file=%s,title=ADR-%s is taken::%s\n", level, c.Path, c.Number, escape(text))
		}
	}
	if cfg.Advisory {
		return 0
	}
	return 1
}

// escape keeps a message on one line of a workflow command.
func escape(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(s)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// perPage is GitHub's largest page; maxPages bounds how many pages of one
// listing are followed: 3,000 files of a pull request is what the API
// lists at most, and a repository with more open pull requests than that
// is beyond what this check is for.
const (
	perPage  = 100
	maxPages = 30
)

type github struct {
	cfg    config
	client *http.Client
}

// baseRecords lists the decision records on the base branch.
func (g *github) baseRecords() (map[string]string, error) {
	var entries []struct {
		Path string `json:"path"`
	}
	if err := g.get("/repos/"+g.cfg.Repo+"/contents/docs/adr?ref="+url.QueryEscape(g.cfg.Base), &entries); err != nil {
		return nil, err
	}
	out := make(map[string]string)
	for _, e := range entries {
		if n := numberOf(e.Path); n != "" {
			out[n] = e.Path
		}
	}
	return out, nil
}

// openPulls lists the open pull requests into the base branch, each with
// the records it adds.
func (g *github) openPulls() ([]pull, error) {
	var out []pull
	for page := 1; page <= maxPages; page++ {
		var list []struct {
			Number  int       `json:"number"`
			Title   string    `json:"title"`
			Created time.Time `json:"created_at"`
		}
		path := fmt.Sprintf("/repos/%s/pulls?state=open&base=%s&per_page=%d&page=%d", g.cfg.Repo, url.QueryEscape(g.cfg.Base), perPage, page)
		if err := g.get(path, &list); err != nil {
			return nil, err
		}
		for _, p := range list {
			adds, err := g.addedRecords(p.Number)
			if err != nil {
				return nil, err
			}
			out = append(out, pull{Number: p.Number, Title: p.Title, Created: p.Created, Adds: adds})
		}
		if len(list) < perPage {
			break
		}
	}
	return out, nil
}

// addedRecords lists the decision records a pull request adds, renames
// into place or copies, by number.
func (g *github) addedRecords(number int) (map[string]string, error) {
	out := make(map[string]string)
	for page := 1; page <= maxPages; page++ {
		var files []struct {
			Name   string `json:"filename"`
			Status string `json:"status"`
		}
		if err := g.get(fmt.Sprintf("/repos/%s/pulls/%d/files?per_page=%d&page=%d", g.cfg.Repo, number, perPage, page), &files); err != nil {
			return nil, err
		}
		for _, f := range files {
			if f.Status != "added" && f.Status != "renamed" && f.Status != "copied" {
				continue
			}
			if n := numberOf(f.Name); n != "" {
				out[n] = f.Name
			}
		}
		if len(files) < perPage {
			break
		}
	}
	return out, nil
}

func (g *github) get(path string, v any) error {
	req, err := http.NewRequest(http.MethodGet, strings.TrimSuffix(g.cfg.API, "/")+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if g.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.cfg.Token)
	}
	client := *g.client
	if client.Timeout == 0 {
		client.Timeout = 30 * time.Second
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &e)
		return fmt.Errorf("GET %s: %s %s", path, resp.Status, e.Message)
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	return nil
}
