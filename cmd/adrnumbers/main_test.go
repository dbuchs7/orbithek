package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakePR struct {
	number  int
	title   string
	created string
	files   [][2]string // filename, status
}

// fakeGitHub serves the records on main and the open pull requests with
// their files, as the REST API does, paged by per_page and page.
func fakeGitHub(t *testing.T, base []string, pulls []fakePR) *httptest.Server {
	t.Helper()
	page := func(r *http.Request, n int) (int, int) {
		per, p := 30, 1
		fmt.Sscan(r.URL.Query().Get("per_page"), &per)
		fmt.Sscan(r.URL.Query().Get("page"), &p)
		from := min((p-1)*per, n)
		return from, min(from+per, n)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/o/r/contents/docs/adr", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("ref") != "main" {
			http.Error(w, `{"message":"No commit found for the ref"}`, http.StatusNotFound)
			return
		}
		var out []map[string]string
		for _, p := range append([]string{"docs/adr/README.md", "docs/adr/template.md"}, base...) {
			out = append(out, map[string]string{"path": p})
		}
		json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("GET /repos/o/r/pulls", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != "open" || r.URL.Query().Get("base") != "main" {
			t.Errorf("pulls listed with %s", r.URL.RawQuery)
		}
		from, to := page(r, len(pulls))
		out := []map[string]any{}
		for _, p := range pulls[from:to] {
			out = append(out, map[string]any{"number": p.number, "title": p.title, "created_at": p.created})
		}
		json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("GET /repos/o/r/pulls/{n}/files", func(w http.ResponseWriter, r *http.Request) {
		for _, p := range pulls {
			if fmt.Sprint(p.number) != r.PathValue("n") {
				continue
			}
			from, to := page(r, len(p.files))
			out := []map[string]string{}
			for _, f := range p.files[from:to] {
				out = append(out, map[string]string{"filename": f[0], "status": f[1]})
			}
			json.NewEncoder(w).Encode(out)
			return
		}
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

var mainRecords = []string{"docs/adr/0184-pictures-are-moved.md", "docs/adr/0185-the-manual-is-kept-current.md"}

func check(t *testing.T, srv *httptest.Server, pr int, advisory bool) (int, string) {
	t.Helper()
	var out bytes.Buffer
	code := run(config{Repo: "o/r", Base: "main", API: srv.URL + "/", PR: pr, Actions: true, Advisory: advisory}, srv.Client(), &out)
	return code, out.String()
}

func TestTwoOpenPullRequestsAddingTheSameNumberClash(t *testing.T) {
	srv := fakeGitHub(t, mainRecords, []fakePR{
		{263, "Import a presentation", "2026-11-10T14:00:00Z", [][2]string{
			{"docs/adr/0186-a-presentation-is-imported.md", "added"}, {"docs/adr/README.md", "modified"}}},
		{266, "Check ADR numbers", "2026-11-10T15:00:00Z", [][2]string{
			{"docs/adr/0186-adr-numbers-are-checked.md", "added"}, {"cmd/adrnumbers/main.go", "added"}}},
	})
	code, out := check(t, srv, 266, false)
	if code != 1 {
		t.Fatalf("exit %d, want 1:\n%s", code, out)
	}
	for _, want := range []string{
		"ADR-0186 is also added by the open pull request #263 (Import a presentation); #263 was opened first.",
		"the next number free of the base branch and every open pull request is ADR-0187",
		"::error file=docs/adr/0186-adr-numbers-are-checked.md,title=ADR-0186 is taken::",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	// The other side sees the same clash, and that it came first.
	if code, out := check(t, srv, 263, true); code != 0 || !strings.Contains(out, "this pull request was opened first") ||
		!strings.Contains(out, "::warning file=docs/adr/0186-a-presentation-is-imported.md,") {
		t.Errorf("advisory: exit %d, want 0 with a warning that it came first:\n%s", code, out)
	}
}

func TestANumberTakenOnTheBaseBranchClashes(t *testing.T) {
	srv := fakeGitHub(t, mainRecords, []fakePR{
		{263, "Import", "2026-11-10T14:00:00Z", [][2]string{{"docs/adr/0184-a-presentation-is-imported.md", "added"}}},
	})
	code, out := check(t, srv, 263, false)
	if code != 1 || !strings.Contains(out, "ADR-0184 is already taken on the base branch by docs/adr/0184-pictures-are-moved.md.") ||
		!strings.Contains(out, "is ADR-0186.") {
		t.Errorf("exit %d, want 1 naming main's record and ADR-0186:\n%s", code, out)
	}
}

func TestNoClash(t *testing.T) {
	srv := fakeGitHub(t, mainRecords, []fakePR{
		{263, "Import", "2026-11-10T14:00:00Z", [][2]string{
			{"docs/adr/0186-a-presentation-is-imported.md", "renamed"},
			{"docs/adr/0185-the-manual-is-kept-current.md", "modified"}, // changing main's record is no claim
			{"docs/adr/0184-pictures-are-moved.md", "removed"}}},
		{270, "Docs", "2026-11-10T16:00:00Z", [][2]string{{"README.md", "modified"}}},
	})
	if code, out := check(t, srv, 263, false); code != 0 || !strings.Contains(out, "ADR 0186: no other open pull request") {
		t.Errorf("exit %d, want 0:\n%s", code, out)
	}
	if code, out := check(t, srv, 270, false); code != 0 || !strings.Contains(out, "adds no decision record") {
		t.Errorf("exit %d, want 0 for a pull request without records:\n%s", code, out)
	}
	if code, out := check(t, srv, 999, false); code != 0 || !strings.Contains(out, "#999 is not open against main") {
		t.Errorf("exit %d, want 0 for a pull request that is not open:\n%s", code, out)
	}
}

func TestListingsArePaged(t *testing.T) {
	var pulls []fakePR
	for i := 1; i <= 105; i++ {
		pulls = append(pulls, fakePR{number: i, title: fmt.Sprint("PR ", i), created: "2026-11-01T00:00:00Z"})
	}
	var many [][2]string
	for i := 0; i < 150; i++ {
		many = append(many, [2]string{fmt.Sprintf("internal/x/f%d.go", i), "added"})
	}
	pulls[2].files = append(many, [2]string{"docs/adr/0186-late-in-the-list.md", "added"})
	pulls[104].files = [][2]string{{"docs/adr/0186-on-the-second-page.md", "added"}}
	srv := fakeGitHub(t, mainRecords, pulls)
	code, out := check(t, srv, 105, false)
	if code != 1 || !strings.Contains(out, "#3 (PR 3)") {
		t.Errorf("exit %d, want the record on the second page of #3's files found:\n%s", code, out)
	}
}

func TestACheckThatCannotRunFailsUnlessAdvisory(t *testing.T) {
	srv := fakeGitHub(t, mainRecords, nil)
	var out bytes.Buffer
	cfg := config{Repo: "o/r", Base: "develop", API: srv.URL, PR: 1, Actions: true}
	if code := run(cfg, srv.Client(), &out); code != 2 || !strings.Contains(out.String(), "::error title=ADR number check could not run::GET /repos/o/r/contents/docs/adr?ref=develop: 404 Not Found") {
		t.Errorf("exit %d, want 2 with an error annotation:\n%s", code, out.String())
	}
	out.Reset()
	cfg.Advisory = true
	if code := run(cfg, srv.Client(), &out); code != 0 || !strings.Contains(out.String(), "::warning title=ADR number check could not run::") {
		t.Errorf("advisory: exit %d, want 0 with a warning:\n%s", code, out.String())
	}
	out.Reset()
	cfg = config{Repo: "o/r", Base: "main", API: "http://127.0.0.1:1", PR: 1}
	if code := run(cfg, srv.Client(), &out); code != 2 {
		t.Errorf("unreachable API: exit %d, want 2", code)
	}
}

func TestBadAnswers(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/o/r/contents/docs/adr", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`[]`)) })
	mux.HandleFunc("GET /repos/o/r/pulls", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"number": 1, "title": "x", "created_at": "2026-11-10T00:00:00Z"}]`))
	})
	mux.HandleFunc("GET /repos/o/r/pulls/1/files", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`not json`)) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	var out bytes.Buffer
	if code := run(config{Repo: "o/r", Base: "main", API: srv.URL, PR: 1, Token: "secret"}, srv.Client(), &out); code != 2 ||
		!strings.Contains(out.String(), "GET /repos/o/r/pulls/1/files?per_page=100&page=1: invalid character") {
		t.Errorf("exit %d, want 2 for an answer that is not JSON:\n%s", code, out.String())
	}
	if strings.Contains(out.String(), "secret") {
		t.Error("the token is printed")
	}
}

func TestTheTokenIsSent(t *testing.T) {
	var auth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = append(auth, r.Header.Get("Authorization"))
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	var out bytes.Buffer
	run(config{Repo: "o/r", Base: "main", API: srv.URL, PR: 1, Token: "t0ken"}, srv.Client(), &out)
	if len(auth) == 0 || auth[0] != "Bearer t0ken" {
		t.Errorf("Authorization headers %q", auth)
	}
}

func TestNumbersAndOrder(t *testing.T) {
	for path, want := range map[string]string{
		"docs/adr/0186-a-presentation.md": "0186",
		"docs/adr/README.md":              "",
		"docs/adr/template.md":            "",
		"docs/adr/186-short.md":           "",
		"docs/adr/sub/0186-nested.md":     "",
		"other/0186-elsewhere.md":         "",
	} {
		if got := numberOf(path); got != want {
			t.Errorf("numberOf(%q) = %q, want %q", path, got, want)
		}
	}
	if got := nextFree(map[string]string{"0009": "x"}, nil); got != "0010" {
		t.Errorf("nextFree = %s", got)
	}
	same := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	own := pull{Number: 5, Created: same}
	if m := message(clash{Number: "0001", Other: &pull{Number: 4, Title: "t", Created: same}}, own, "0002"); !strings.Contains(m, "#4 was opened first") {
		t.Errorf("at the same time the lower number is first: %s", m)
	}
	if got := escape("a\nb%c\r"); got != "a%0Ab%25c%0D" {
		t.Errorf("escape = %q", got)
	}
}
