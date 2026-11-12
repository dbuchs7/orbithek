package share

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseAddress(t *testing.T) {
	a, err := ParseAddress(" /mnt/nas/photos/ ")
	if err != nil || a.Kind != Dir || a.Dir != filepath.Clean("/mnt/nas/photos") {
		t.Fatalf("dir: %+v %v", a, err)
	}
	a, err = ParseAddress("s3://photos/family/2024?endpoint=https://s3.example.com&region=eu-central-1")
	if err != nil || a.Kind != S3 || a.Bucket != "photos" || a.Prefix != "family/2024" || a.Region != "eu-central-1" || a.Endpoint.Host != "s3.example.com" {
		t.Fatalf("s3: %+v %v", a, err)
	}
	if a, _ := ParseAddress("s3://photos?endpoint=https://s3.example.com"); a.Region != "us-east-1" || a.Prefix != "" {
		t.Errorf("default region: %+v", a)
	}
	for _, bad := range []string{
		"",
		"relative/dir",
		"/a\x00b",
		"s3://photos",
		"s3://photos?endpoint=http://s3.example.com",
		"s3://photos?endpoint=https://user:pw@s3.example.com",
		"s3://key:secret@photos?endpoint=https://s3.example.com",
		"s3://No_Bucket?endpoint=https://s3.example.com",
		"s3://photos/a/../b?endpoint=https://s3.example.com",
		"smb://nas/photos",
	} {
		if _, err := ParseAddress(bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

func TestCheckPathAndSkipped(t *testing.T) {
	for _, ok := range []string{"a.jpg", "2024/Ferien/IMG_0001.JPG", "Fotos – Sommer/ä.png"} {
		if err := CheckPath(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "/a", "a/", "a//b", "./a", "a/../b", "a\tb", `a\b`, "\xff", strings.Repeat("a", MaxPath+1)} {
		if CheckPath(bad) == nil {
			t.Errorf("%q passed", bad)
		}
	}
	for p, want := range map[string]bool{
		"a.jpg": false, ".DS_Store": true, "x/.thumbs/a.jpg": true,
		"@eaDir/a.jpg": true, "2024/@eaDir/a.jpg/SYNOFILE_THUMB_M.jpg": true, "#recycle/a.jpg": true, "#snapshot/a": true,
	} {
		if Skipped(p) != want {
			t.Errorf("Skipped(%q) = %v", p, !want)
		}
	}
}

func TestRootsAndCheckDir(t *testing.T) {
	sep := string(os.PathListSeparator)
	roots := Roots(func(k string) string {
		if k == RootsVar {
			return "/mnt/nas/ " + sep + "relative" + sep + sep + "/srv/photos"
		}
		return ""
	})
	if len(roots) != 2 || roots[0] != filepath.Clean("/mnt/nas") || roots[1] != filepath.Clean("/srv/photos") {
		t.Fatalf("roots %v", roots)
	}
	if err := CheckDir("/mnt/nas/family", roots, "/var/lib/orbithek"); err != nil {
		t.Error(err)
	}
	if err := CheckDir("/srv/photos", roots, "/var/lib/orbithek"); err != nil {
		t.Error(err)
	}
	for _, c := range []struct{ dir, data string }{
		{"/mnt/nasty", ""},
		{"/etc", ""},
		{"/mnt/nas/../../etc", ""},
		{"/mnt/nas/data", "/mnt/nas/data/wiki"},
		{"/mnt/nas/data/wiki/attachments", "/mnt/nas/data/wiki"},
	} {
		if err := CheckDir(c.dir, roots, c.data); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v: %v", c, err)
		}
	}
	if err := CheckDir("/mnt/nas", nil, ""); err == nil || !strings.Contains(err.Error(), RootsVar) {
		t.Errorf("no roots: %v", err)
	}
}

func TestKeyVarsAndLinks(t *testing.T) {
	a, s := KeyVars("nas.photos-1")
	if a != "OT_SHARE_NAS_PHOTOS_1_ACCESS_KEY" || s != "OT_SHARE_NAS_PHOTOS_1_SECRET_KEY" {
		t.Errorf("%s %s", a, s)
	}
	if got := Link("smb://nas/photos/{path}", "2024/Ferien Tessin/a#1.jpg"); got != "smb://nas/photos/2024/Ferien%20Tessin/a%231.jpg" {
		t.Errorf("smb link %q", got)
	}
	if got := Link("https://nas.example/photo/?p={path}", "a.jpg"); got != "https://nas.example/photo/?p=a.jpg" {
		t.Errorf("https link %q", got)
	}
	for _, bad := range []string{"", "file:///mnt/{path}", "http://nas/{path}", "javascript:alert(1)//{path}", "https://nas/photos", "https:///{path}", "https://u:p@nas/{path}", "smb://nas/a b/{path}"} {
		if LinkTemplateOK(bad) || Link(bad, "a.jpg") != "" {
			t.Errorf("template %q accepted", bad)
		}
	}
	if Link("smb://nas/{path}", "../etc/passwd") != "" {
		t.Error("a bad path made a link")
	}
}

func write(t *testing.T, root, p, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(p))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDirSource(t *testing.T) {
	root := t.TempDir()
	write(t, root, "b.jpg", "bbb")
	write(t, root, "2024/Ferien/a.jpg", "0123456789")
	write(t, root, "2024/@eaDir/a.jpg/SYNOFILE_THUMB_M.jpg", "x")
	write(t, root, ".hidden/c.jpg", "x")
	write(t, root, "2024/.DS_Store", "x")
	outside := t.TempDir()
	write(t, outside, "secret.jpg", "secret")
	if runtime.GOOS != "windows" {
		if err := os.Symlink(filepath.Join(outside, "secret.jpg"), filepath.Join(root, "link.jpg")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, "linkdir")); err != nil {
			t.Fatal(err)
		}
	}
	a, _ := ParseAddress(root)
	dev, err := DeviceOf(root)
	if err != nil {
		t.Fatal(err)
	}
	src, err := Open(a, "nas", dev, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	ctx := context.Background()
	objs, more, err := src.List(ctx, 10)
	if err != nil || more {
		t.Fatalf("list: %v %v", more, err)
	}
	var paths []string
	for _, o := range objs {
		paths = append(paths, o.Path)
	}
	if strings.Join(paths, ",") != "2024/Ferien/a.jpg,b.jpg" {
		t.Fatalf("paths %v", paths)
	}
	if objs[0].Size != 10 || objs[0].Tag == "" || objs[0].ModTime.IsZero() {
		t.Errorf("object %+v", objs[0])
	}
	if objs, more, _ := src.List(ctx, 1); len(objs) != 1 || !more {
		t.Errorf("max: %d %v", len(objs), more)
	}

	r, err := src.Open(ctx, "2024/Ferien/a.jpg")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(r)
	r.Close()
	if string(b) != "0123456789" {
		t.Errorf("open %q", b)
	}
	r, err = src.OpenRange(ctx, "2024/Ferien/a.jpg", 3, 4)
	if err != nil {
		t.Fatal(err)
	}
	b, _ = io.ReadAll(r)
	r.Close()
	if string(b) != "3456" {
		t.Errorf("range %q", b)
	}
	for _, bad := range []string{"../" + filepath.Base(outside) + "/secret.jpg", "link.jpg", "linkdir/secret.jpg", "2024", "missing.jpg", "/etc/passwd"} {
		if r, err := src.Open(ctx, bad); err == nil {
			r.Close()
			t.Errorf("opened %q", bad)
		}
	}
	if _, err := src.OpenRange(ctx, "b.jpg", -1, 2); !errors.Is(err, ErrInvalid) {
		t.Errorf("negative range: %v", err)
	}
	if _, err := src.OpenRange(ctx, "missing.jpg", 0, 2); err == nil {
		t.Error("range of a missing file")
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := src.List(cancelled, 10); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: %v", err)
	}
}

func TestDirSourceUnreadable(t *testing.T) {
	none := func(string) string { return "" }
	if _, err := Open(Address{Kind: Dir, Dir: filepath.Join(t.TempDir(), "gone")}, "nas", 0, none); !errors.Is(err, ErrUnreadable) {
		t.Errorf("missing: %v", err)
	}
	file := filepath.Join(t.TempDir(), "f")
	write(t, filepath.Dir(file), "f", "x")
	if _, err := Open(Address{Kind: Dir, Dir: file}, "nas", 0, none); !errors.Is(err, ErrUnreadable) {
		t.Errorf("file: %v", err)
	}
	dir := t.TempDir()
	dev, _ := DeviceOf(dir)
	if dev != 0 {
		if _, err := Open(Address{Kind: Dir, Dir: dir}, "nas", dev+1, none); !errors.Is(err, ErrUnreadable) {
			t.Errorf("other device: %v", err)
		}
	}
	if _, err := DeviceOf(filepath.Join(dir, "gone")); err == nil {
		t.Error("device of a missing directory")
	}
	if _, err := Open(Address{Kind: "ftp"}, "x", 0, none); !errors.Is(err, ErrInvalid) {
		t.Errorf("kind: %v", err)
	}
}
