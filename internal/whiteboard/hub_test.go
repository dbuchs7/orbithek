package whiteboard

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/item"
)

// fakeStore is a backend of one or more drawings.
type fakeStore struct {
	mu       sync.Mutex
	pictures map[Key][]byte
	editors  map[string]bool
	readers  map[string]bool
	saves    []save
	failSave error
}

type save struct {
	key       Key
	principal string
	png       []byte
}

func (f *fakeStore) Access(k Key, p string) (bool, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.pictures[k]; !ok {
		return false, false
	}
	return f.readers[p] || f.editors[p], f.editors[p]
}

func (f *fakeStore) Load(k Key) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.pictures[k]
	if !ok {
		return nil, item.ErrNotFound
	}
	return p, nil
}

func (f *fakeStore) Save(k Key, p string, png []byte) (item.ID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failSave != nil {
		return "", f.failSave
	}
	f.pictures[k] = png
	f.saves = append(f.saves, save{k, p, png})
	return item.ID(fmt.Sprintf("%032d", len(f.saves))), nil
}

func (f *fakeStore) lastScene(t *testing.T) (Scene, bool) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.saves) == 0 {
		t.Fatal("nothing was saved")
	}
	sc, stale, err := ReadScene(f.saves[len(f.saves)-1].png, 1<<30)
	if err != nil {
		t.Fatal(err)
	}
	return sc, stale
}

var key = Key{Item: "item1", First: "0123456789abcdef0123456789abcdef"}

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func el(id, kind string, version int, extra string) json.RawMessage {
	if extra != "" {
		extra = "," + extra
	}
	return json.RawMessage(fmt.Sprintf(`{"id":%q,"type":%q,"version":%d,"x":0,"y":0,"width":100,"height":50%s}`, id, kind, version, extra))
}

func newTest(t *testing.T, elements ...json.RawMessage) (*Hub, *fakeStore, *clock) {
	t.Helper()
	p, err := WriteScene(Placeholder(), Scene{Elements: elements}, false)
	if err != nil {
		t.Fatal(err)
	}
	st := &fakeStore{pictures: map[Key][]byte{key: p}, editors: map[string]bool{"ann": true, "bob": true, "ai": true}, readers: map[string]bool{"rea": true}}
	clk := &clock{t: time.Unix(1_800_000_000, 0)}
	h := NewHub(st, WithClock(clk.now))
	h.after = func(time.Duration, func()) {} // cursors are sent by the test
	return h, st, clk
}

// next returns the next event of a kind, skipping others.
func next(t *testing.T, sub *Sub, kind string) Event {
	t.Helper()
	for {
		select {
		case e, ok := <-sub.C:
			if !ok {
				t.Fatalf("stream closed waiting for %s", kind)
			}
			if e.Kind == kind {
				return e
			}
		default:
			t.Fatalf("no %s event", kind)
		}
	}
}

func drain(sub *Sub) []Event {
	var out []Event
	for {
		select {
		case e, ok := <-sub.C:
			if !ok {
				return out
			}
			out = append(out, e)
		default:
			return out
		}
	}
}

func kinds(evs []Event) string {
	var k []string
	for _, e := range evs {
		k = append(k, e.Kind)
	}
	return strings.Join(k, ",")
}

func TestJoinSnapshotAndOrder(t *testing.T) {
	// A scene without fractional indices is ordered once, as loaded.
	h, _, _ := newTest(t, el("a", "rectangle", 1, ""), el("b", "ellipse", 2, `"isDeleted":true`), el("bad id!", "rectangle", 1, ""))
	ann, err := h.Join(key, "ann", "Ann")
	if err != nil {
		t.Fatal(err)
	}
	snap := next(t, ann, Snapshot)
	if !snap.Editable || !snap.Keeper || len(snap.Elements) != 1 || len(snap.Participants) != 1 || snap.Client != ann.Client {
		t.Fatalf("snapshot = %+v", snap)
	}
	var m meta
	_ = json.Unmarshal(snap.Elements[0], &m)
	if m.Index == nil || *m.Index != "a0" {
		t.Fatalf("index = %v", m.Index)
	}
	rea, err := h.Join(key, "rea", "Rea")
	if err != nil {
		t.Fatal(err)
	}
	if s := next(t, rea, Snapshot); s.Editable || s.Keeper || len(s.Participants) != 2 {
		t.Fatalf("reader's snapshot = %+v", s)
	}
	if p := next(t, ann, Presence); !p.Keeper || len(p.Participants) != 2 {
		t.Fatalf("presence = %+v", p)
	}
	if _, err := h.Join(key, "eve", "Eve"); !errors.Is(err, item.ErrNotFound) {
		t.Fatalf("a stranger joined: %v", err)
	}
	if _, err := h.Join(Key{Item: "x", First: key.First}, "ann", "Ann"); !errors.Is(err, item.ErrNotFound) {
		t.Fatalf("a missing drawing: %v", err)
	}
	if n, c := h.Stats(); n != 1 || c != 2 {
		t.Fatalf("stats = %d %d", n, c)
	}
	if infos := h.Sessions(); len(infos) != 1 || infos[0].Participants != 2 {
		t.Fatalf("sessions = %+v", infos)
	}
	rea.Leave()
	ann.Leave()
	ann.Leave() // twice is harmless
	if n, _ := h.Stats(); n != 0 {
		t.Fatal("the session stayed open")
	}
}

func TestPutLockAndRefusals(t *testing.T) {
	h, _, _ := newTest(t, el("a", "rectangle", 1, `"index":"a0"`))
	ann, _ := h.Join(key, "ann", "Ann")
	bob, _ := h.Join(key, "bob", "Bob")
	drain(ann)
	drain(bob)

	// Ann selects a: she holds it, everyone is told.
	if res, err := h.Apply(key, ann.Client, "ann", []Op{{Op: "lock", IDs: []string{"a", "unknown"}}}); err != nil || len(res.Refused) != 0 {
		t.Fatal(res, err)
	}
	if l := next(t, bob, Locks); len(l.Locks) != 1 || l.Locks[0].Client != ann.Client {
		t.Fatalf("locks = %+v", l)
	}
	// Bob may neither lock nor change it, and is told how it is.
	res, err := h.Apply(key, bob.Client, "bob", []Op{{Op: "lock", IDs: []string{"a"}}, {Op: "put", Element: el("a", "rectangle", 5, "")}})
	if err != nil || len(res.Refused) != 2 || res.Refused[0].By != "Ann" || res.Refused[1].Current == nil {
		t.Fatalf("refused = %+v %v", res, err)
	}
	// Ann changes it; Bob sees it, Ann does not get her own change back.
	if res, err := h.Apply(key, ann.Client, "ann", []Op{{Op: "put", Element: el("a", "rectangle", 2, `"index":"a0"`)}}); err != nil || len(res.Refused) != 0 {
		t.Fatal(res, err)
	}
	if e := next(t, bob, Elements); len(e.Elements) != 1 || e.By != ann.Client {
		t.Fatalf("elements = %+v", e)
	}
	if evs := drain(ann); strings.Contains(kinds(evs), Elements) {
		t.Fatal("Ann got her own change back")
	}
	// A stale version is refused.
	if res, _ := h.Apply(key, ann.Client, "ann", []Op{{Op: "put", Element: el("a", "rectangle", 2, "")}}); len(res.Refused) != 1 {
		t.Fatal("a stale version was taken")
	}
	// Bob adds an element of his own and an image file.
	file := json.RawMessage(`{"id":"f1","mimeType":"image/png","dataURL":"data:image/png;base64,AA=="}`)
	if _, err := h.Apply(key, bob.Client, "bob", []Op{{Op: "file", File: file}, {Op: "put", Element: el("b", "image", 1, `"fileId":"f1","index":"a1"`)}}); err != nil {
		t.Fatal(err)
	}
	evs := drain(ann)
	if kinds(evs) != "locks,files,elements" && kinds(evs) != "files,elements" {
		t.Fatalf("events = %s", kinds(evs))
	}
	// Ann releases; the lock is free again.
	_, _ = h.Apply(key, ann.Client, "ann", []Op{{Op: "release", IDs: []string{"a"}}})
	if l := next(t, bob, Locks); l.Locks[0].Client != "" {
		t.Fatalf("release = %+v", l)
	}
	// Leaving frees what one holds.
	_, _ = h.Apply(key, bob.Client, "bob", []Op{{Op: "lock", IDs: []string{"a"}}})
	drain(ann)
	bob.Leave()
	if l := next(t, ann, Locks); l.Locks[0].Client != "" {
		t.Fatalf("leave = %+v", l)
	}
}

func TestApplyRefusesBatches(t *testing.T) {
	h, _, _ := newTest(t)
	ann, _ := h.Join(key, "ann", "Ann")
	rea, _ := h.Join(key, "rea", "Rea")
	for name, ops := range map[string][]Op{
		"none":            nil,
		"unknown":         {{Op: "fly"}},
		"bad element":     {{Op: "put", Element: json.RawMessage(`[1]`)}},
		"bad id":          {{Op: "put", Element: el("a b", "rectangle", 1, "")}},
		"bad kind":        {{Op: "put", Element: el("a", "spaceship", 1, "")}},
		"bad version":     {{Op: "put", Element: el("a", "rectangle", 0, "")}},
		"bad file":        {{Op: "file", File: json.RawMessage(`{"id":"f","mimeType":"text/html","dataURL":"data:text/html;base64,AA"}`)}},
		"bad lock":        {{Op: "lock", IDs: []string{"a b"}}},
		"bad background":  {{Op: "background", Background: "red; x"}},
		"bad vote action": {{Op: "vote", Action: "rig"}},
	} {
		if _, err := h.Apply(key, ann.Client, "ann", ops); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := h.Apply(key, rea.Client, "rea", []Op{{Op: "put", Element: el("a", "rectangle", 1, "")}}); !errors.Is(err, ErrForbidden) {
		t.Errorf("a reader changed: %v", err)
	}
	if _, err := h.Apply(key, rea.Client, "rea", []Op{{Op: "pointer", X: 1, Y: 2}}); err != nil {
		t.Errorf("a reader's cursor: %v", err)
	}
	if _, err := h.Apply(key, "nobody", "ann", []Op{{Op: "pointer"}}); !errors.Is(err, ErrNotJoined) {
		t.Errorf("unknown client: %v", err)
	}
	if _, err := h.Apply(key, ann.Client, "bob", []Op{{Op: "pointer"}}); !errors.Is(err, ErrNotJoined) {
		t.Errorf("someone else's client: %v", err)
	}
	if _, err := h.Apply(Key{Item: "x"}, ann.Client, "ann", []Op{{Op: "pointer"}}); !errors.Is(err, ErrNotJoined) {
		t.Errorf("no session: %v", err)
	}
	if _, err := h.Apply(key, ann.Client, "ann", []Op{{Op: "background", Background: "#eeeeee"}}); err != nil {
		t.Errorf("background: %v", err)
	}
}

func TestPointersAreSentTogether(t *testing.T) {
	h, _, _ := newTest(t)
	var flush func()
	h.after = func(_ time.Duration, f func()) { flush = f }
	ann, _ := h.Join(key, "ann", "Ann")
	bob, _ := h.Join(key, "bob", "Bob")
	drain(ann)
	_, _ = h.Apply(key, ann.Client, "ann", []Op{{Op: "pointer", X: 1, Y: 2}})
	_, _ = h.Apply(key, bob.Client, "bob", []Op{{Op: "pointer", X: 3, Y: 4}})
	_, _ = h.Apply(key, ann.Client, "ann", []Op{{Op: "pointer", X: 5, Y: 6}})
	if flush == nil {
		t.Fatal("nothing scheduled")
	}
	flush()
	p := next(t, ann, Pointers)
	if len(p.Pointers) != 2 {
		t.Fatalf("pointers = %+v", p)
	}
	for _, x := range p.Pointers {
		if x.Client == ann.Client && x.X != 5 {
			t.Fatalf("not the newest position: %+v", x)
		}
	}
	flush() // nothing moved since
	if evs := drain(ann); strings.Contains(kinds(evs), Pointers) {
		t.Fatal("cursors sent again")
	}
}

func TestClientsAreBudgeted(t *testing.T) {
	b := budget.Get()
	defer budget.Set(b)
	small := b
	small.WhiteboardClients = 1
	small.WhiteboardSessions = 1
	budget.Set(small)
	h, st, _ := newTest(t)
	if _, err := h.Join(key, "ann", "Ann"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Join(key, "bob", "Bob"); !errors.Is(err, ErrBusy) {
		t.Fatalf("over budget: %v", err)
	}
	if _, err := h.Change(key, "ai", "AI", []Change{{Op: "note", Text: "x"}}); !errors.Is(err, ErrBusy) {
		t.Fatalf("agent over budget: %v", err)
	}
	other := Key{Item: "item2", First: key.First}
	st.pictures[other] = st.pictures[key]
	if _, err := h.Join(other, "ann", "Ann"); !errors.Is(err, ErrBusy) {
		t.Fatalf("sessions over budget: %v", err)
	}
}

func TestElementsAreBudgeted(t *testing.T) {
	b := budget.Get()
	defer budget.Set(b)
	small := b
	small.WhiteboardElements = 1
	budget.Set(small)
	h, _, _ := newTest(t)
	ann, _ := h.Join(key, "ann", "Ann")
	res, _ := h.Apply(key, ann.Client, "ann", []Op{{Op: "put", Element: el("a", "rectangle", 1, "")}, {Op: "put", Element: el("b", "rectangle", 1, "")}})
	if len(res.Refused) != 1 || res.Refused[0].ID != "b" {
		t.Fatalf("refused = %+v", res)
	}
	if _, err := h.Change(key, "ai", "AI", []Change{{Op: "note"}}); !errors.Is(err, ErrBusy) {
		t.Fatalf("agent over budget: %v", err)
	}
	small.WhiteboardSceneBytes = 10
	budget.Set(small)
	file := json.RawMessage(`{"id":"f1","mimeType":"image/png","dataURL":"data:image/png;base64,AA=="}`)
	if _, err := h.Apply(key, ann.Client, "ann", []Op{{Op: "file", File: file}}); !errors.Is(err, ErrBusy) {
		t.Fatalf("file over budget: %v", err)
	}
}

func TestCheckpointByTheKeeper(t *testing.T) {
	h, st, clk := newTest(t)
	ann, _ := h.Join(key, "ann", "Ann")
	bob, _ := h.Join(key, "bob", "Bob")
	drain(ann)
	drain(bob)
	_, _ = h.Apply(key, bob.Client, "bob", []Op{{Op: "put", Element: el("a", "rectangle", 1, `"index":"a0"`)}})
	h.Tick() // not quiet yet
	if evs := drain(ann); strings.Contains(kinds(evs), CheckpointEv) {
		t.Fatal("asked too early")
	}
	clk.add(QuietSave)
	h.Tick()
	cp := next(t, ann, CheckpointEv)
	if cp.Token == "" {
		t.Fatal("no token")
	}
	if evs := drain(bob); strings.Contains(kinds(evs), CheckpointEv) {
		t.Fatal("Bob is not the keeper")
	}
	// A change after the request is not in this checkpoint.
	_, _ = h.Apply(key, bob.Client, "bob", []Op{{Op: "put", Element: el("b", "ellipse", 1, `"index":"a1"`)}})
	if _, err := h.Checkpoint(key, ann.Client, "ann", "wrong", Placeholder()); !errors.Is(err, ErrStale) {
		t.Fatalf("wrong token: %v", err)
	}
	if _, err := h.Checkpoint(key, bob.Client, "ann", cp.Token, Placeholder()); !errors.Is(err, ErrNotJoined) {
		t.Fatalf("someone else's client: %v", err)
	}
	// The picture the browser sends carries a scene of its own; the
	// server's copy replaces it.
	forged, _ := WriteScene(Placeholder(), Scene{Elements: []json.RawMessage{el("forged", "rectangle", 1, "")}}, false)
	sv, err := h.Checkpoint(key, ann.Client, "ann", cp.Token, forged)
	if err != nil || sv.By != "Ann" || sv.Stale {
		t.Fatal(sv, err)
	}
	sc, stale := st.lastScene(t)
	if stale || len(sc.Elements) != 1 || !strings.Contains(string(sc.Elements[0]), `"a"`) {
		t.Fatalf("saved scene = %s, stale %v", sc.Elements, stale)
	}
	if st.saves[0].principal != "ann" {
		t.Fatalf("saved as %s", st.saves[0].principal)
	}
	if s := next(t, bob, SavedEv); s.Saved == nil || s.Saved.File == "" {
		t.Fatalf("saved = %+v", s)
	}
	if _, err := h.Checkpoint(key, ann.Client, "ann", cp.Token, Placeholder()); !errors.Is(err, ErrStale) {
		t.Fatalf("a token twice: %v", err)
	}
	// The change made since is saved by the next checkpoint, asked for
	// at once when someone asks to save.
	_, _ = h.Apply(key, bob.Client, "bob", []Op{{Op: "save"}})
	h.Tick()
	cp = next(t, ann, CheckpointEv)
	if _, err := h.Checkpoint(key, ann.Client, "ann", cp.Token, Placeholder()); err != nil {
		t.Fatal(err)
	}
	if sc, _ := st.lastScene(t); len(sc.Elements) != 2 {
		t.Fatalf("second checkpoint holds %d elements", len(sc.Elements))
	}
	// Nothing changed: nothing is asked.
	clk.add(time.Hour)
	h.Tick()
	if evs := drain(ann); strings.Contains(kinds(evs), CheckpointEv) {
		t.Fatal("asked without a change")
	}
	// A keeper who does not answer is asked again after CheckpointWait.
	_, _ = h.Apply(key, bob.Client, "bob", []Op{{Op: "put", Element: el("c", "ellipse", 1, `"index":"a2"`)}})
	clk.add(QuietSave)
	h.Tick()
	first := next(t, ann, CheckpointEv)
	h.Tick()
	if evs := drain(ann); strings.Contains(kinds(evs), CheckpointEv) {
		t.Fatal("asked again too soon")
	}
	clk.add(CheckpointWait)
	h.Tick()
	if again := next(t, ann, CheckpointEv); again.Token == first.Token {
		t.Fatal("the same token again")
	}
	// A reader cannot store a checkpoint.
	rea, _ := h.Join(key, "rea", "Rea")
	if _, err := h.Checkpoint(key, rea.Client, "rea", "x", Placeholder()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("reader's checkpoint: %v", err)
	}
	// A failed save keeps the scene unsaved.
	st.failSave = errors.New("disk full")
	clk.add(CheckpointWait)
	h.Tick()
	cp = next(t, ann, CheckpointEv)
	if _, err := h.Checkpoint(key, ann.Client, "ann", cp.Token, Placeholder()); err == nil {
		t.Fatal("a failed save succeeded")
	}
	if _, err := h.Checkpoint(key, ann.Client, "ann", cp.Token, []byte("not a png")); err == nil {
		t.Fatal("not a PNG was stored")
	}
}

func TestStalePictureIsDrawnAgain(t *testing.T) {
	p, _ := WriteScene(Placeholder(), Scene{Elements: []json.RawMessage{el("a", "rectangle", 1, `"index":"a0"`)}}, true)
	h, st, _ := newTest(t)
	st.pictures[key] = p
	ann, _ := h.Join(key, "ann", "Ann")
	drain(ann)
	_, _ = h.Apply(key, ann.Client, "ann", []Op{{Op: "save"}})
	h.Tick()
	cp := next(t, ann, CheckpointEv)
	if _, err := h.Checkpoint(key, ann.Client, "ann", cp.Token, Placeholder()); err != nil {
		t.Fatal(err)
	}
	if _, stale := st.lastScene(t); stale {
		t.Fatal("still stale")
	}
}

func TestServerSavesWithoutABrowser(t *testing.T) {
	h, st, clk := newTest(t)
	ids, err := h.Change(key, "ai", "Assistant", []Change{{Op: "note", Text: "Idea"}})
	if err != nil || len(ids) != 1 {
		t.Fatal(ids, err)
	}
	h.Tick()
	if len(st.saves) != 0 {
		t.Fatal("saved before it was quiet")
	}
	clk.add(AloneSave)
	h.Tick()
	sc, stale := st.lastScene(t)
	if !stale || len(sc.Elements) != 2 || st.saves[0].principal != "ai" {
		t.Fatalf("server save: stale %v, %d elements, by %s", stale, len(sc.Elements), st.saves[0].principal)
	}
	// The agent lapses and the session closes.
	clk.add(AgentTTL)
	h.Tick()
	if n, _ := h.Stats(); n != 0 {
		t.Fatal("the session stayed open")
	}
}

func TestEveryoneLeftUnsaved(t *testing.T) {
	h, st, _ := newTest(t)
	ann, _ := h.Join(key, "ann", "Ann")
	_, _ = h.Apply(key, ann.Client, "ann", []Op{{Op: "put", Element: el("a", "rectangle", 1, "")}})
	ann.Leave()
	if n, _ := h.Stats(); n != 1 {
		t.Fatal("a session with unsaved changes closed")
	}
	h.Tick()
	if _, stale := st.lastScene(t); !stale {
		t.Fatal("not marked stale")
	}
	if n, _ := h.Stats(); n != 0 {
		t.Fatal("the session stayed open after saving")
	}
}

func TestShutdownSaves(t *testing.T) {
	h, st, _ := newTest(t)
	ann, _ := h.Join(key, "ann", "Ann")
	_, _ = h.Apply(key, ann.Client, "ann", []Op{{Op: "put", Element: el("a", "rectangle", 1, "")}})
	drain(ann)
	h.Shutdown()
	if e := next(t, ann, Closed); e.Reason == "" {
		t.Fatal("no reason")
	}
	if len(st.saves) != 1 {
		t.Fatal("not saved on shutdown")
	}
	if n, _ := h.Stats(); n != 0 {
		t.Fatal("sessions left")
	}
}

func TestLocksAndAgentsLapse(t *testing.T) {
	h, _, clk := newTest(t, el("a", "rectangle", 1, `"index":"a0"`))
	ann, _ := h.Join(key, "ann", "Ann")
	bob, _ := h.Join(key, "bob", "Bob")
	_, _ = h.Apply(key, ann.Client, "ann", []Op{{Op: "lock", IDs: []string{"a"}}})
	drain(bob)
	clk.add(LockTTL)
	h.Tick()
	if l := next(t, bob, Locks); l.Locks[0].Client != "" {
		t.Fatalf("lapse = %+v", l)
	}
	if _, err := h.Change(key, "ai", "AI", []Change{{Op: "move", ID: "a", X: ptr(5), Y: ptr(5)}}); err != nil {
		t.Fatal(err)
	}
	if p := next(t, bob, Presence); len(p.Participants) != 3 || !p.Participants[0].Agent {
		t.Fatalf("agent presence = %+v", p.Participants)
	}
	clk.add(AgentTTL)
	h.Tick()
	if p := next(t, bob, Presence); len(p.Participants) != 2 {
		t.Fatalf("agent stayed: %+v", p.Participants)
	}
}

func TestRevalidate(t *testing.T) {
	h, st, _ := newTest(t, el("a", "rectangle", 1, `"index":"a0"`))
	ann, _ := h.Join(key, "ann", "Ann")
	bob, _ := h.Join(key, "bob", "Bob")
	_, _ = h.Apply(key, bob.Client, "bob", []Op{{Op: "lock", IDs: []string{"a"}}})
	drain(ann)
	// Bob may only read now: his lock goes.
	st.mu.Lock()
	delete(st.editors, "bob")
	st.readers["bob"] = true
	st.mu.Unlock()
	h.Revalidate()
	if l := next(t, ann, Locks); l.Locks[0].Client != "" {
		t.Fatalf("locks = %+v", l)
	}
	if _, err := h.Apply(key, bob.Client, "bob", []Op{{Op: "put", Element: el("a", "rectangle", 9, "")}}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a reader changed: %v", err)
	}
	// Bob may not read any more: he is closed out.
	st.mu.Lock()
	delete(st.readers, "bob")
	st.mu.Unlock()
	h.Revalidate()
	if e := next(t, bob, Closed); e.Reason == "" {
		t.Fatal("no reason")
	}
	if _, err := h.Apply(key, bob.Client, "bob", []Op{{Op: "pointer"}}); !errors.Is(err, ErrNotJoined) {
		t.Fatal(err)
	}
	// The drawing is gone: the session closes.
	st.mu.Lock()
	delete(st.pictures, key)
	st.mu.Unlock()
	h.Revalidate()
	next(t, ann, Closed)
	if n, _ := h.Stats(); n != 0 {
		t.Fatal("session stayed")
	}
}

func TestApplyNoticesLostAccess(t *testing.T) {
	h, st, _ := newTest(t)
	bob, _ := h.Join(key, "bob", "Bob")
	st.mu.Lock()
	delete(st.editors, "bob")
	st.mu.Unlock()
	if _, err := h.Apply(key, bob.Client, "bob", []Op{{Op: "pointer"}}); !errors.Is(err, ErrNotJoined) {
		t.Fatalf("lost access: %v", err)
	}
}

func TestSlowBrowserIsDropped(t *testing.T) {
	h, _, _ := newTest(t)
	ann, _ := h.Join(key, "ann", "Ann")
	bob, _ := h.Join(key, "bob", "Bob")
	for i := range buffer + 10 {
		_, _ = h.Apply(key, ann.Client, "ann", []Op{{Op: "put", Element: el(fmt.Sprintf("e%d", i), "rectangle", 1, "")}})
	}
	n := len(drain(bob))
	if n > buffer {
		t.Fatalf("got %d events", n)
	}
	if _, err := h.Apply(key, bob.Client, "bob", []Op{{Op: "pointer"}}); !errors.Is(err, ErrNotJoined) {
		t.Fatalf("a dropped browser still applies: %v", err)
	}
}

func ptr(f float64) *float64 { return &f }
