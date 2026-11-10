package photosource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var epoch = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

// fakeGoogle answers as Google's token endpoint, the Picker API and the
// host of photos do.
type fakeGoogle struct {
	mu       sync.Mutex
	tokens   map[string]bool // access tokens accepted
	issued   int
	verifier string // the code verifier of the last exchange
	sessions map[string]bool
	created  int
	deleted  []string
	items    int    // how many items a session holds
	page     int    // items per page
	baseHost string // the host of the base addresses handed out
	scope    string
	expires  int // expires_in, seconds
	picked   bool
	status   map[string]int // a path answered with this status
	pickURI  string
	sameID   string // the ID every new session gets, if set
	video    bool   // item 0 is a video
	photo    []byte
	auths    []string // Authorization of each download
}

func newFakeGoogle() *fakeGoogle {
	return &fakeGoogle{tokens: map[string]bool{}, sessions: map[string]bool{}, items: 3, page: 2,
		baseHost: "lh3.googleusercontent.com", scope: pickerScope, expires: 3600,
		status: map[string]int{}, pickURI: "https://photos.google.com/picker/abc", photo: []byte("JPEGDATA")}
}

func (f *fakeGoogle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if st := f.status[r.Method+" "+r.URL.Path]; st != 0 {
		http.Error(w, "refused", st)
		return
	}
	switch hostOf(r) {
	case "oauth2.googleapis.com":
		r.ParseForm()
		if r.URL.Path != "/token" || r.Form.Get("code") != "good" || r.Form.Get("client_id") != "id" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid_grant","error_description":"secret-ish detail"}`))
			return
		}
		f.verifier = r.Form.Get("code_verifier")
		f.issued++
		tok := fmt.Sprintf("at-%d", f.issued)
		f.tokens[tok] = true
		answer := map[string]any{"access_token": tok, "token_type": "Bearer", "refresh_token": "rt-never-kept"}
		if f.expires > 0 {
			answer["expires_in"] = f.expires
		}
		if f.scope != "" {
			answer["scope"] = f.scope
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(answer)
		return
	case "photospicker.googleapis.com":
		if !f.tokens[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")] {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		f.picker(w, r)
		return
	case "lh3.googleusercontent.com", "other.googleusercontent.com":
		f.auths = append(f.auths, r.Header.Get("Authorization"))
		if strings.HasPrefix(r.URL.Path, "/redirect-away") {
			http.Redirect(w, r, "https://evil.googleusercontent.com/x=d", http.StatusFound)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/redirect-same") {
			http.Redirect(w, r, "/item-0=d", http.StatusFound)
			return
		}
		if !f.tokens[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")] {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.(http.Flusher).Flush() // no length: the limit is found while reading
		switch {
		case strings.HasSuffix(r.URL.Path, "=d"):
			w.Write(f.photo)
		case strings.HasSuffix(r.URL.Path, "=dv"):
			w.Write([]byte("VIDEODATA"))
		case strings.HasSuffix(r.URL.Path, "=w1600-h1600"):
			w.Write([]byte("POSTER"))
		default:
			http.NotFound(w, r)
		}
		return
	}
	http.NotFound(w, r)
}

func (f *fakeGoogle) session(id string) map[string]any {
	return map[string]any{"id": id, "pickerUri": f.pickURI,
		"pollingConfig": map[string]any{"pollInterval": "5s", "timeoutIn": "1800s"},
		"expireTime":    epoch.Add(2 * time.Hour).Format(time.RFC3339Nano), "mediaItemsSet": f.picked}
}

func (f *fakeGoogle) picker(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/sessions":
		f.created++
		id := fmt.Sprintf("sess-%d", f.created)
		if f.sameID != "" {
			id = f.sameID
		}
		f.sessions[id] = true
		enc.Encode(f.session(id))
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/sessions/"):
		id := strings.TrimPrefix(r.URL.Path, "/v1/sessions/")
		if !f.sessions[id] {
			http.NotFound(w, r)
			return
		}
		enc.Encode(f.session(id))
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/sessions/"):
		f.deleted = append(f.deleted, strings.TrimPrefix(r.URL.Path, "/v1/sessions/"))
		w.Write([]byte("{}"))
	case r.Method == http.MethodGet && r.URL.Path == "/v1/mediaItems":
		q := r.URL.Query()
		if !f.sessions[q.Get("sessionId")] || q.Get("pageSize") != "100" {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		start, _ := strconv.Atoi(q.Get("pageToken"))
		var items []map[string]any
		for i := start; i < f.items && i < start+f.page; i++ {
			typ := "PHOTO"
			if f.video && i == 0 {
				typ = "VIDEO"
			}
			items = append(items, map[string]any{"id": fmt.Sprintf("item-%d", i), "createTime": "2024-05-01T10:00:00.123Z", "type": typ,
				"mediaFile": map[string]any{"baseUrl": fmt.Sprintf("https://%s/item-%d", f.baseHost, i), "mimeType": "image/jpeg",
					"filename": fmt.Sprintf("IMG_%d.jpg", i),
					"mediaFileMetadata": map[string]any{"width": 4032, "height": "3024", "cameraMake": "Make", "cameraModel": "Model",
						"photoMetadata": map[string]any{"focalLength": 4.2}}}})
		}
		answer := map[string]any{"mediaItems": items}
		if start+f.page < f.items {
			answer["nextPageToken"] = strconv.Itoa(start + f.page)
		}
		enc.Encode(answer)
	default:
		http.NotFound(w, r)
	}
}

// connected sets up Google against the fake, with a clock the test moves,
// and connects alice.
func connected(t *testing.T) (*Google, *fakeGoogle, *time.Time) {
	t.Helper()
	f := newFakeGoogle()
	g := NewGoogle("id", "secret", "https://wiki.example/cb", serve(t, f))
	now := epoch
	g.now = func() time.Time { return now }
	_, state, err := g.Begin("alice")
	if err != nil {
		t.Fatal(err)
	}
	if user, err := g.Finish(context.Background(), state, "good"); err != nil || user != "alice" {
		t.Fatalf("Finish = %q, %v", user, err)
	}
	return g, f, &now
}

func TestNewGoogleNeedsClient(t *testing.T) {
	if NewGoogle("", "s", "r", nil) != nil || NewGoogle("i", "", "r", nil) != nil || NewGoogle("i", "s", "", nil) != nil {
		t.Fatal("set up without a client ID, secret or redirect address")
	}
	g := NewGoogle("i", "s", "r", nil)
	if g.client != http.DefaultClient || g.API != googlePicker || g.AuthURL != googleAuthURL || g.TokenURL != googleTokenURL {
		t.Fatalf("defaults: %+v", g)
	}
}

func TestBeginAsksOnlineAccessWithPKCE(t *testing.T) {
	g := NewGoogle("id", "secret", "https://wiki.example/cb", nil)
	authURL, state, err := g.Begin("alice")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(authURL)
	q := u.Query()
	if u.Host != "accounts.google.com" || q.Get("access_type") != "online" || q.Get("code_challenge_method") != "S256" ||
		q.Get("code_challenge") == "" || q.Get("state") != state || q.Get("scope") != pickerScope || q.Get("prompt") != "" {
		t.Fatalf("consent address %s", authURL)
	}
	if user, ok := g.Pending(state); !ok || user != "alice" {
		t.Fatalf("Pending = %q, %v", user, ok)
	}
	if _, ok := g.Pending("other"); ok {
		t.Fatal("an unknown state is pending")
	}
	g.Cancel(state)
	if _, ok := g.Pending(state); ok {
		t.Fatal("a cancelled state is pending")
	}
	if _, _, err := g.Begin(""); err == nil {
		t.Fatal("began without a person")
	}
}

func TestBeginIsBoundedAndFlowsExpire(t *testing.T) {
	g := NewGoogle("id", "secret", "https://wiki.example/cb", nil)
	now := epoch
	g.now = func() time.Time { return now }
	var first string
	for i := range maxPending {
		_, state, err := g.Begin("alice")
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = state
		}
	}
	_, _, err := g.Begin("alice")
	wantErr(t, err, ErrTooMany)
	now = now.Add(FlowLifetime)
	if _, ok := g.Pending(first); ok {
		t.Fatal("an expired flow is pending")
	}
	if _, _, err := g.Begin("alice"); err != nil {
		t.Fatalf("expired flows were not swept: %v", err)
	}
	_, err = g.Finish(context.Background(), first, "good")
	wantErr(t, err, ErrNotFound)
}

func TestFinishKeepsOnlyTheAccessToken(t *testing.T) {
	g, f, now := connected(t)
	if f.verifier == "" {
		t.Fatal("the code was exchanged without a verifier")
	}
	if !g.Connected("alice") || g.Connected("bob") {
		t.Fatal("Connected")
	}
	if g.access["alice"].token != "at-1" {
		t.Fatalf("token %+v", g.access["alice"])
	}
	// No refresh token anywhere: only the access token and its expiry are kept.
	if len(g.access) != 1 || g.access["alice"] != (accessToken{token: "at-1", expires: epoch.Add(time.Hour)}) {
		t.Fatalf("kept %+v", g.access)
	}
	*now = epoch.Add(59*time.Minute + time.Second)
	if g.Connected("alice") {
		t.Fatal("a token with less than a minute left counts as connected")
	}
	if _, ok := g.access["alice"]; ok {
		t.Fatal("a token that ran out was kept")
	}
}

func TestFinishRefusals(t *testing.T) {
	f := newFakeGoogle()
	g := NewGoogle("id", "secret", "https://wiki.example/cb", serve(t, f))
	ctx := context.Background()

	_, err := g.Finish(ctx, "nope", "good")
	wantErr(t, err, ErrNotFound)

	_, state, _ := g.Begin("alice")
	_, err = g.Finish(ctx, state, "bad")
	wantErr(t, err, ErrUnreadable)
	if !strings.Contains(err.Error(), "invalid_grant") || strings.Contains(err.Error(), "secret-ish") {
		t.Fatalf("message %q", err)
	}
	// A state is used once.
	_, err = g.Finish(ctx, state, "good")
	wantErr(t, err, ErrNotFound)

	f.scope = "openid email"
	_, state, _ = g.Begin("alice")
	_, err = g.Finish(ctx, state, "good")
	wantErr(t, err, ErrNotConnected)
	if g.Connected("alice") {
		t.Fatal("a grant without the scope connected")
	}

	f.scope, f.expires = "", 0
	now := epoch
	g.now = func() time.Time { return now }
	_, state, _ = g.Begin("alice")
	if _, err := g.Finish(ctx, state, "good"); err != nil {
		t.Fatal(err)
	}
	if got := g.access["alice"].expires; !got.Equal(epoch.Add(defaultTokenLife)) {
		t.Fatalf("expiry without expires_in %v", got)
	}

	f.status["POST /token"] = http.StatusInternalServerError
	_, state, _ = g.Begin("alice")
	_, err = g.Finish(ctx, state, "good")
	wantErr(t, err, ErrUnreadable)
}

func TestFinishWithoutAccessToken(t *testing.T) {
	client := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"token_type":"Bearer","access_token":""}`))
	}))
	g := NewGoogle("id", "secret", "https://wiki.example/cb", client)
	_, state, _ := g.Begin("alice")
	_, err := g.Finish(context.Background(), state, "good")
	if err == nil {
		t.Fatal("finished without an access token")
	}
}

func TestErrorCode(t *testing.T) {
	for s, want := range map[string]bool{"invalid_grant": true, "": false, "Bad": false, "a b": false, strings.Repeat("a", 41): false} {
		if errorCode(s) != want {
			t.Errorf("errorCode(%q) = %v", s, !want)
		}
	}
}

func TestPickingFromSessionToDownload(t *testing.T) {
	g, f, now := connected(t)
	ctx := context.Background()
	s, err := g.NewSession(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if s.ID != "sess-1" || s.PickerURI != f.pickURI || s.PollInterval != 5*time.Second || s.Timeout != 1800*time.Second || s.Picked {
		t.Fatalf("session %+v", s)
	}
	f.mu.Lock()
	f.picked = true
	f.mu.Unlock()
	s, err = g.Poll(ctx, "alice", s.ID)
	if err != nil || !s.Picked {
		t.Fatalf("Poll = %+v, %v", s, err)
	}
	items, err := g.Items(ctx, "alice", s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("%d items", len(items))
	}
	want := Media{ID: "item-1", Name: "IMG_1.jpg", Type: "image/jpeg", Taken: time.Date(2024, 5, 1, 10, 0, 0, 123e6, time.UTC),
		Width: 4032, Height: 3024, Make: "Make", Model: "Model"}
	if items[1] != want {
		t.Fatalf("item %+v, want %+v", items[1], want)
	}
	rc, m, err := g.Download(ctx, "alice", s.ID, "item-2", 100)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil || string(data) != "JPEGDATA" || m.ID != "item-2" {
		t.Fatalf("download %q %+v %v", data, m, err)
	}
	if f.auths[0] != "Bearer at-1" {
		t.Fatalf("download sent %q", f.auths[0])
	}
	// The session lasts until its expiry; the token runs out before.
	*now = epoch.Add(time.Hour + 50*time.Minute)
	_, _, err = g.Download(ctx, "alice", s.ID, "item-2", 100)
	wantErr(t, err, ErrNotConnected)
	*now = epoch.Add(2 * time.Hour)
	_, _, err = g.Download(ctx, "alice", s.ID, "item-2", 100)
	wantErr(t, err, ErrNotFound)
}

func TestDownloadLimitsAndRefusals(t *testing.T) {
	g, f, _ := connected(t)
	ctx := context.Background()
	f.video = true
	s, _ := g.NewSession(ctx, "alice")
	if _, err := g.Items(ctx, "alice", s.ID); err != nil {
		t.Fatal(err)
	}

	// A video is downloaded with "=dv", and its poster is the picture
	// Google makes of it (ADR-0192).
	if m, err := g.Picked("alice", s.ID, "item-0"); err != nil || !m.Video {
		t.Fatalf("picked %+v, %v", m, err)
	}
	_, err := g.Picked("bob", s.ID, "item-0")
	wantErr(t, err, ErrNotFound)
	_, err = g.Picked("alice", s.ID, "item-9")
	wantErr(t, err, ErrNotFound)
	rc, m, err := g.Download(ctx, "alice", s.ID, "item-0", 100)
	if err != nil || !m.Video {
		t.Fatalf("video %+v, %v", m, err)
	}
	if data, _ := io.ReadAll(rc); string(data) != "VIDEODATA" {
		t.Fatalf("video %q", data)
	}
	rc.Close()
	rc, err = g.Poster(ctx, "alice", s.ID, "item-0", 100)
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := io.ReadAll(rc); string(data) != "POSTER" {
		t.Fatalf("poster %q", data)
	}
	rc.Close()
	_, err = g.Poster(ctx, "alice", s.ID, "item-1", 100)
	wantErr(t, err, ErrInvalid)
	_, _, err = g.Download(ctx, "alice", s.ID, "item-1", 0)
	wantErr(t, err, ErrInvalid)
	_, _, err = g.Download(ctx, "alice", s.ID, "item-9", 100)
	wantErr(t, err, ErrNotFound)
	_, _, err = g.Download(ctx, "bob", s.ID, "item-1", 100)
	wantErr(t, err, ErrNotFound)

	// Longer than the limit fails while reading.
	rc, _, err = g.Download(ctx, "alice", s.ID, "item-1", 4)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	wantErr(t, err, ErrTooLarge)
	if len(data) != 4 {
		t.Fatalf("read %d bytes", len(data))
	}
	// Exactly the limit is fine.
	rc, _, _ = g.Download(ctx, "alice", s.ID, "item-1", 8)
	if data, err := io.ReadAll(rc); err != nil || len(data) != 8 {
		t.Fatalf("read %d, %v", len(data), err)
	}
	rc.Close()

	f.mu.Lock()
	f.status["GET /item-1=d"] = http.StatusForbidden
	f.mu.Unlock()
	_, _, err = g.Download(ctx, "alice", s.ID, "item-1", 100)
	wantErr(t, err, ErrUnreadable)
	if strings.Contains(err.Error(), "googleusercontent") || strings.Contains(err.Error(), "at-1") {
		t.Fatalf("message names the address or token: %v", err)
	}

	f.mu.Lock()
	f.status["GET /item-1=d"] = http.StatusUnauthorized
	f.mu.Unlock()
	_, _, err = g.Download(ctx, "alice", s.ID, "item-1", 100)
	wantErr(t, err, ErrNotConnected)
	if g.Connected("alice") {
		t.Fatal("401 kept the token")
	}
	_, _, err = g.Download(ctx, "alice", s.ID, "item-2", 100)
	wantErr(t, err, ErrNotConnected)
}

func TestDownloadRefusesAddressesOfOtherHosts(t *testing.T) {
	for _, host := range []string{"evil.example", "googleusercontent.com.evil.example", "lh3.googleusercontent.com:8443", "user@lh3.googleusercontent.com"} {
		g, f, _ := connected(t)
		ctx := context.Background()
		f.baseHost = host
		s, _ := g.NewSession(ctx, "alice")
		if _, err := g.Items(ctx, "alice", s.ID); err != nil {
			t.Fatal(err)
		}
		_, _, err := g.Download(ctx, "alice", s.ID, "item-1", 100)
		wantErr(t, err, ErrUnreadable)
		if len(f.auths) != 0 {
			t.Fatalf("%s: requested", host)
		}
	}
	// A base address over http is refused too.
	g, _, _ := connected(t)
	g.sessions["x"] = &session{user: "alice", expires: epoch.Add(time.Hour),
		items: map[string]pickedItem{"m": {baseURL: "http://lh3.googleusercontent.com/m"}, "q": {baseURL: "https://lh3.googleusercontent.com/m?x=1"}}}
	for _, m := range []string{"m", "q"} {
		_, _, err := g.Download(context.Background(), "alice", "x", m, 100)
		wantErr(t, err, ErrUnreadable)
	}
}

func TestDownloadFollowsRedirectsOnlyOnItsHost(t *testing.T) {
	g, f, _ := connected(t)
	ctx := context.Background()
	g.sessions["x"] = &session{user: "alice", expires: epoch.Add(time.Hour), items: map[string]pickedItem{
		"away": {baseURL: "https://lh3.googleusercontent.com/redirect-away"},
		"same": {baseURL: "https://lh3.googleusercontent.com/redirect-same"}}}
	_, _, err := g.Download(ctx, "alice", "x", "away", 100)
	wantErr(t, err, ErrUnreadable)
	if !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("message %v", err)
	}
	rc, _, err := g.Download(ctx, "alice", "x", "same", 100)
	if err != nil {
		t.Fatal(err)
	}
	rc.Close()
	if len(f.auths) != 3 || f.auths[2] != "Bearer at-1" {
		t.Fatalf("auths %q", f.auths)
	}
	// The caller's own redirect policy is still asked.
	asked := false
	g.client.CheckRedirect = func(*http.Request, []*http.Request) error { asked = true; return nil }
	rc, _, err = g.Download(ctx, "alice", "x", "same", 100)
	if err != nil || !asked {
		t.Fatalf("own policy asked %v, %v", asked, err)
	}
	rc.Close()
}

func TestSessionsAreBoundedAndOwn(t *testing.T) {
	g, f, now := connected(t)
	ctx := context.Background()
	var ids []string
	for range maxSessions {
		s, err := g.NewSession(ctx, "alice")
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, s.ID)
	}
	_, err := g.NewSession(ctx, "alice")
	wantErr(t, err, ErrTooMany)

	// bob, connected too, sees none of alice's sessions.
	_, state, _ := g.Begin("bob")
	if _, err := g.Finish(ctx, state, "good"); err != nil {
		t.Fatal(err)
	}
	_, err = g.Poll(ctx, "bob", ids[0])
	wantErr(t, err, ErrNotFound)
	_, err = g.Items(ctx, "bob", ids[0])
	wantErr(t, err, ErrNotFound)
	g.Close(ctx, "bob", ids[0])
	if len(f.deleted) != 0 {
		t.Fatal("bob closed alice's session")
	}
	if _, err := g.NewSession(ctx, "bob"); err != nil {
		t.Fatalf("alice's sessions count for bob: %v", err)
	}

	g.Close(ctx, "alice", ids[0])
	if len(f.deleted) != 1 || f.deleted[0] != ids[0] {
		t.Fatalf("deleted %v", f.deleted)
	}
	_, err = g.Poll(ctx, "alice", ids[0])
	wantErr(t, err, ErrNotFound)
	if _, err := g.NewSession(ctx, "alice"); err != nil {
		t.Fatalf("a closed session still counts: %v", err)
	}

	// Sessions are forgotten at their expiry.
	*now = epoch.Add(2 * time.Hour)
	_, err = g.Poll(ctx, "alice", ids[1])
	wantErr(t, err, ErrNotFound)

	g.Forget("bob")
	if g.Connected("bob") {
		t.Fatal("Forget kept the token")
	}
	for _, s := range g.sessions {
		if s.user == "bob" {
			t.Fatal("Forget kept a session")
		}
	}
}

func TestNewSessionRefusals(t *testing.T) {
	g, f, _ := connected(t)
	ctx := context.Background()
	_, err := g.NewSession(ctx, "bob")
	wantErr(t, err, ErrNotConnected)

	f.pickURI = "https://evil.example/picker"
	_, err = g.NewSession(ctx, "alice")
	wantErr(t, err, ErrUnreadable)
	f.pickURI = "https://photos.google.com/picker/abc"

	f.sameID = "dup"
	if _, err := g.NewSession(ctx, "alice"); err != nil {
		t.Fatal(err)
	}
	_, err = g.NewSession(ctx, "alice")
	wantErr(t, err, ErrUnreadable)
	f.sameID = ""

	f.status["POST /v1/sessions"] = http.StatusTooManyRequests
	_, err = g.NewSession(ctx, "alice")
	wantErr(t, err, ErrUnreadable)
	if !strings.Contains(err.Error(), "429") {
		t.Fatalf("message %v", err)
	}

	f.status["POST /v1/sessions"] = http.StatusUnauthorized
	_, err = g.NewSession(ctx, "alice")
	wantErr(t, err, ErrNotConnected)
	if g.Connected("alice") {
		t.Fatal("401 kept the token")
	}
}

func TestPollRefusals(t *testing.T) {
	g, f, _ := connected(t)
	ctx := context.Background()
	s, _ := g.NewSession(ctx, "alice")
	f.mu.Lock()
	delete(f.sessions, s.ID)
	f.mu.Unlock()
	_, err := g.Poll(ctx, "alice", s.ID)
	wantErr(t, err, ErrNotFound)

	f.mu.Lock()
	f.sessions[s.ID] = true
	f.pickURI = "ftp://photos.google.com/x"
	f.mu.Unlock()
	_, err = g.Poll(ctx, "alice", s.ID)
	wantErr(t, err, ErrUnreadable)
}

func TestPollAnsweringAnotherSession(t *testing.T) {
	client := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case hostOf(r) == "oauth2.googleapis.com":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"access_token":"t","expires_in":3600}`))
		case r.Method == http.MethodPost:
			w.Write([]byte(`{"id":"a"}`))
		case r.URL.Path == "/v1/sessions/a":
			w.Write([]byte(`{"id":"b"}`))
		default:
			w.Write([]byte(`not json`))
		}
	}))
	g := NewGoogle("id", "secret", "https://wiki.example/cb", client)
	ctx := context.Background()
	_, state, _ := g.Begin("alice")
	if _, err := g.Finish(ctx, state, "c"); err != nil {
		t.Fatal(err)
	}
	s, err := g.NewSession(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	// Without polling config or expiry: defaults.
	if s.PollInterval != time.Second || s.Timeout != 0 || !g.sessions["a"].expires.After(time.Now().Add(50*time.Minute)) {
		t.Fatalf("defaults %+v %v", s, g.sessions["a"].expires)
	}
	_, err = g.Poll(ctx, "alice", "a")
	wantErr(t, err, ErrUnreadable)
	_, err = g.Items(ctx, "alice", "a")
	wantErr(t, err, ErrUnreadable)
}

func TestNewSessionWithoutID(t *testing.T) {
	client := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hostOf(r) == "oauth2.googleapis.com" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"access_token":"t","expires_in":3600}`))
			return
		}
		w.Write([]byte(`{"pickerUri":"https://photos.google.com/p"}`))
	}))
	g := NewGoogle("id", "secret", "https://wiki.example/cb", client)
	_, state, _ := g.Begin("alice")
	g.Finish(context.Background(), state, "c")
	_, err := g.NewSession(context.Background(), "alice")
	wantErr(t, err, ErrUnreadable)
}

func TestItemsBounds(t *testing.T) {
	g, f, _ := connected(t)
	ctx := context.Background()
	s, _ := g.NewSession(ctx, "alice")

	f.items, f.page = maxItems+1, 100
	_, err := g.Items(ctx, "alice", s.ID)
	wantErr(t, err, ErrTooMany)

	f.items, f.page = maxItemPages+1, 1
	_, err = g.Items(ctx, "alice", s.ID)
	wantErr(t, err, ErrTooMany)

	f.items, f.page = maxItems, 100
	items, err := g.Items(ctx, "alice", s.ID)
	if err != nil || len(items) != maxItems {
		t.Fatalf("%d items, %v", len(items), err)
	}

	f.status["GET /v1/mediaItems"] = http.StatusBadRequest
	_, err = g.Items(ctx, "alice", s.ID)
	wantErr(t, err, ErrUnreadable)
}

func TestItemsSkipsDuplicatesAndItemsWithoutID(t *testing.T) {
	client := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case hostOf(r) == "oauth2.googleapis.com":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"access_token":"t","expires_in":3600}`))
		case r.Method == http.MethodPost:
			w.Write([]byte(`{"id":"a","expireTime":"2026-11-10T12:10:00Z"}`))
		default:
			w.Write([]byte(`{"mediaItems":[{"id":"x","mediaFile":{"mediaFileMetadata":{"width":"wide","height":-3}}},{"id":"x"},{"id":""}]}`))
		}
	}))
	g := NewGoogle("id", "secret", "https://wiki.example/cb", client)
	now := epoch
	g.now = func() time.Time { return now }
	ctx := context.Background()
	_, state, _ := g.Begin("alice")
	g.Finish(ctx, state, "c")
	s, _ := g.NewSession(ctx, "alice")
	items, err := g.Items(ctx, "alice", s.ID)
	if err != nil || len(items) != 1 || items[0].Width != 0 || items[0].Height != 0 || !items[0].Taken.IsZero() {
		t.Fatalf("items %+v, %v", items, err)
	}
	if got := g.sessions["a"].expires; !got.Equal(epoch.Add(baseURLLife)) {
		t.Fatalf("expiry %v", got)
	}
}

func TestItemsOfASessionForgottenMeanwhile(t *testing.T) {
	var g *Google
	client := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case hostOf(r) == "oauth2.googleapis.com":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"access_token":"t","expires_in":3600}`))
		case r.Method == http.MethodPost:
			w.Write([]byte(`{"id":"a"}`))
		default:
			g.Forget("alice")
			g.mu.Lock()
			g.access["alice"] = accessToken{token: "t", expires: time.Now().Add(time.Hour)}
			g.mu.Unlock()
			w.Write([]byte(`{"mediaItems":[]}`))
		}
	}))
	g = NewGoogle("id", "secret", "https://wiki.example/cb", client)
	ctx := context.Background()
	_, state, _ := g.Begin("alice")
	g.Finish(ctx, state, "c")
	s, _ := g.NewSession(ctx, "alice")
	_, err := g.Items(ctx, "alice", s.ID)
	wantErr(t, err, ErrNotFound)
}

func TestCallRefusals(t *testing.T) {
	g, _, _ := connected(t)
	ctx := context.Background()
	err := g.call(ctx, "alice", http.MethodGet, "/x", nil, func() {}, nil)
	wantErr(t, err, ErrInvalid)
	err = g.call(ctx, "alice", "BAD METHOD", "/x", nil, nil, nil)
	wantErr(t, err, ErrUnreadable)

	big := serve(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(make([]byte, maxAnswer+1))
	}))
	g.client = big
	err = g.call(ctx, "alice", http.MethodGet, "/big", nil, nil, nil)
	wantErr(t, err, ErrTooLarge)

	ctx2, cancel := context.WithCancel(ctx)
	cancel()
	err = g.call(ctx2, "alice", http.MethodGet, "/x", nil, nil, nil)
	wantErr(t, err, ErrUnreadable)
	g.sessions["x"] = &session{user: "alice", expires: epoch.Add(time.Hour), items: map[string]pickedItem{"m": {baseURL: "https://lh3.googleusercontent.com/m"}}}
	_, _, err = g.Download(ctx2, "alice", "x", "m", 10)
	wantErr(t, err, ErrUnreadable)
}

func TestDuration(t *testing.T) {
	for s, want := range map[string]time.Duration{"5s": 5 * time.Second, "3.5s": 3500 * time.Millisecond, "": time.Second,
		"-1s": time.Second, "9999s": time.Minute, "0.1s": time.Second} {
		if got := duration(s, time.Second, time.Minute); got != want {
			t.Errorf("duration(%q) = %v, want %v", s, got, want)
		}
	}
}
