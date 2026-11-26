package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dbuchs7/orbithek/internal/manualcheck"
)

// gitRepo creates a repository with a base commit holding a catalogue, a
// shell and two manuals, and returns a function that runs git in it and
// one that writes a file there.
func gitRepo(t *testing.T) (dir string, gitc func(args ...string) string, write func(path, text string)) {
	t.Helper()
	dir = t.TempDir()
	gitc = func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=Claude", "GIT_AUTHOR_EMAIL=noreply@anthropic.com",
			"GIT_COMMITTER_NAME=Claude", "GIT_COMMITTER_EMAIL=noreply@anthropic.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write = func(path, text string) {
		t.Helper()
		p := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	gitc("init", "-q", "-b", "main")
	write(manualcheck.Catalogue, "export default {\n  \"$name\": \"English\",\n  \"common.close\": \"Close\"\n};\n")
	write(manualcheck.Shell, "switch (section) {\n  case \"album\": return albumView();\n}\nfunction albumView() {}\n")
	write(manualcheck.Manuals+"en.md", "# Manual\n")
	write(manualcheck.Manuals+"de.md", "# Handbuch\n")
	gitc("add", "-A")
	gitc("commit", "-q", "-m", "base")
	return dir, gitc, write
}

// inDir runs f with the working directory set to dir, because run calls git
// in the current directory.
func inDir(t *testing.T, dir string, f func()) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(old); err != nil {
			t.Fatal(err)
		}
	}()
	f()
}

func writeBody(t *testing.T, text string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "body.txt")
	if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRun(t *testing.T) {
	dir, gitc, write := gitRepo(t)
	base := gitc("rev-parse", "HEAD")

	check := func(name, body string, wantOK bool, wantOut string) {
		t.Helper()
		var out bytes.Buffer
		inDir(t, dir, func() {
			v, err := run(&out, base, "HEAD", body)
			if ok := v.manual && v.pictures; err != nil || ok != wantOK {
				t.Errorf("%s: ok=%v err=%v, want %v\n%s", name, ok, err, wantOK, out.String())
			}
		})
		if !strings.Contains(out.String(), wantOut) {
			t.Errorf("%s: the report lacks %q:\n%s", name, wantOut, out.String())
		}
	}

	// Server code only: nothing people see.
	write("internal/item/x.go", "package item\n")
	gitc("add", "-A")
	gitc("commit", "-q", "-m", "server")
	check("server code", "", true, "changes nothing people see")

	// A new label, a route, a view and a module, without the manuals.
	write(manualcheck.Catalogue, "export default {\n  \"$name\": \"English\",\n  \"common.close\": \"Close\",\n  \"present.open\": \"Presentation window\"\n};\n")
	write(manualcheck.Shell, "switch (section) {\n  case \"album\": return albumView();\n  case \"present\": return presentScreen();\n}\nfunction albumView() {}\nfunction presentScreen() {}\n")
	write(manualcheck.Assets+"present.js", "export {};\n")
	gitc("add", "-A")
	gitc("commit", "-q", "-m", "present")
	check("no manual", "", false, manualcheck.Manuals+"de.md")
	check("no manual", "", false, "present.open")
	check("no manual", "", false, "presentScreen")
	check("no manual", "", false, manualcheck.Assets+"present.js")
	check("a reason", writeBody(t, "Summary\n\nManual: not needed — a label no one looks up\n"), true, "a label no one looks up")
	check("a line without a reason", writeBody(t, "Manual: not needed\n"), false, "FAIL")

	// One manual is not enough; both are.
	write(manualcheck.Manuals+"en.md", "# Manual\n\nPresenting.\n")
	gitc("commit", "-q", "-am", "en manual")
	check("one manual", "", false, manualcheck.Manuals+"de.md")
	write(manualcheck.Manuals+"de.md", "# Handbuch\n\nPräsentieren.\n")
	gitc("commit", "-q", "-am", "de manual")
	check("every manual", "", true, "every user manual is changed")

	inDir(t, dir, func() {
		if _, err := run(&bytes.Buffer{}, base, "HEAD", filepath.Join(dir, "missing.txt")); err == nil {
			t.Error("missing body file: want an error")
		}
		if _, err := run(&bytes.Buffer{}, "no-such-ref", "HEAD", ""); err == nil {
			t.Error("unknown ref: want an error")
		}
	})

	// A broken catalogue at the head is an error, not a pass.
	write(manualcheck.Catalogue, "export default { broken };\n")
	gitc("commit", "-q", "-am", "broken")
	inDir(t, dir, func() {
		if _, err := run(&bytes.Buffer{}, base, "HEAD", ""); err == nil {
			t.Error("broken catalogue: want an error")
		}
	})
}

// TestRunPictures: Check C. A pull request that changes the function a
// picture shows takes it again in every language, or gives a reason; a
// change elsewhere in the same file, or to another picture's keys, does not
// call for it.
func TestRunPictures(t *testing.T) {
	dir, gitc, write := gitRepo(t)
	write(manualcheck.PictureSources, "# picture sources\nhome app.js#homeView key:home.\nicons/shop shop.js\n")
	write(manualcheck.Shell, "function homeView() {\n  return 1;\n}\nfunction other() {\n  return 1;\n}\n")
	for _, f := range []string{"en/home.png", "de/home.png", "icons/shop.png"} {
		write(manualcheck.Pictures+f, "png")
	}
	gitc("add", "-A")
	gitc("commit", "-q", "-m", "pictures")
	base := gitc("rev-parse", "HEAD")
	reason := writeBody(t, "Manual: not needed — no new words\n")

	check := func(name, body string, wantOK bool, wantOut string) {
		t.Helper()
		var out bytes.Buffer
		inDir(t, dir, func() {
			v, err := run(&out, base, "HEAD", body)
			if err != nil || v.pictures != wantOK {
				t.Errorf("%s: pictures=%v err=%v, want %v\n%s", name, v.pictures, err, wantOK, out.String())
			}
		})
		if !strings.Contains(out.String(), wantOut) {
			t.Errorf("%s: the report lacks %q:\n%s", name, wantOut, out.String())
		}
	}

	write(manualcheck.Shell, "function homeView() {\n  return 1;\n}\nfunction other() {\n  return 2;\n}\n")
	gitc("commit", "-q", "-am", "elsewhere")
	check("another function", reason, true, "no picture of the manual")

	write(manualcheck.Shell, "function homeView() {\n  return 2;\n}\nfunction other() {\n  return 2;\n}\n")
	gitc("commit", "-q", "-am", "home")
	check("the function shown", reason, false, "home (because of app.js#homeView)")
	check("the function shown", reason, false, manualcheck.Pictures+"de/home.png")
	check("a reason", writeBody(t, "Pictures: not needed – only a comment\n"), true, "only a comment")

	write(manualcheck.Pictures+"en/home.png", "png 2")
	gitc("commit", "-q", "-am", "en picture")
	check("one language", reason, false, manualcheck.Pictures+"de/home.png")
	write(manualcheck.Pictures+"de/home.png", "png 2")
	gitc("commit", "-q", "-am", "de picture")
	check("every language", reason, true, "no picture of the manual")

	write(manualcheck.Catalogue, "export default {\n  \"$name\": \"English\",\n  \"common.close\": \"Close\",\n  \"home.quick\": \"Quick\"\n};\n")
	write(manualcheck.Assets+"shop.js", "export {};\n")
	gitc("add", "-A")
	gitc("commit", "-q", "-m", "keys and a file")
	check("a key and a file", reason, false, "icons/shop (because of shop.js)")
	// The home picture's key changed too, but the pull request has taken
	// that picture again already.
	var out bytes.Buffer
	inDir(t, dir, func() {
		if _, err := run(&out, base, "HEAD", reason); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(out.String(), "- home ") {
		t.Errorf("a picture taken again is reported stale:\n%s", out.String())
	}

	write(manualcheck.PictureSources, "home app.js#homeView key:home.\nhome app.js\n")
	gitc("commit", "-q", "-am", "broken list")
	inDir(t, dir, func() {
		if _, err := run(&bytes.Buffer{}, base, "HEAD", reason); err == nil {
			t.Error("a broken list of pictures: want an error")
		}
	})
}

func TestShowMissingFile(t *testing.T) {
	dir, _, _ := gitRepo(t)
	inDir(t, dir, func() {
		if s, err := show("HEAD", "no/such/file"); err != nil || s != "" {
			t.Errorf("show of a missing file = %q, %v", s, err)
		}
	})
}

func TestReportListsAtMostTen(t *testing.T) {
	var out bytes.Buffer
	keys := make([]string, 12)
	for i := range keys {
		keys[i] = "k" + string(rune('a'+i))
	}
	report(&out, manualcheck.Report{Keys: keys})
	if !strings.Contains(out.String(), "… and 2 more") {
		t.Errorf("report = %s", out.String())
	}
}
