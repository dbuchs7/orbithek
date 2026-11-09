// Package whiteboard holds live whiteboards (ADR-0167): drawings
// (ADR-0053) edited by several people and agents at once.
//
// A session is ephemeral server state for one drawing: its elements, each
// kept as Excalidraw wrote it with a lock and the version Excalidraw
// counts, its image files, its participants and what a workshop needs
// for the moment (ADR-0168): a vote, a timer, a presentation. Browsers
// receive every change as a server-sent event and send operations; agents
// read and change it with plain requests.
//
// The session is saved in checkpoints: the server asks one browser, the
// keeper, to draw the scene as it was at a given event, and writes its
// own copy of that scene into the picture, which becomes a new version of
// the drawing. Without a browser it writes the scene into the newest
// picture and marks it stale.
package whiteboard

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dbuchs7/orbithek/internal/budget"
	"github.com/dbuchs7/orbithek/internal/item"
)

// The timing of a session.
const (
	// LockTTL is how long a lock lasts without an operation by its holder.
	LockTTL = 2 * time.Minute
	// AgentTTL is how long an agent counts as a participant after its
	// last change.
	AgentTTL = 2 * time.Minute
	// QuietSave is how long a changed scene rests before it is saved, and
	// LongSave how long it may change on end before it is saved anyway.
	QuietSave = 10 * time.Second
	LongSave  = 2 * time.Minute
	// AloneSave is how long a scene changed without a browser in the
	// session rests before the server saves it, its picture stale.
	AloneSave = 30 * time.Second
	// CheckpointWait is how long the keeper has to send a picture.
	CheckpointWait = time.Minute
	// pointerEvery is how often cursor positions are sent.
	pointerEvery = 100 * time.Millisecond
	// buffer is the events a browser may fall behind before it is dropped.
	buffer = 512
	// colours is the number of participant colours.
	colours = 20
)

// Errors of a session.
var (
	ErrNotJoined = errors.New("not in the whiteboard's session")
	ErrForbidden = errors.New("forbidden")
	ErrBusy      = errors.New("too many live whiteboards, participants or elements")
	ErrInvalid   = errors.New("invalid operation")
	ErrLocked    = errors.New("locked")
	// ErrStale means a checkpoint was not, or no longer, asked for.
	ErrStale = errors.New("no checkpoint was asked for")
)

// Key names a whiteboard: its item and its drawing's first version.
type Key struct {
	Item  item.ID
	First item.ID
}

// Backend is the item store as the hub needs it. Access and Load are
// called with the hub's lock held, Save without it; none calls the hub.
type Backend interface {
	// Access reports what principal may do with the whiteboard: read
	// (join and follow) and edit. A gone item or drawing is not readable.
	Access(k Key, principal string) (read, edit bool)
	// Load returns the newest version of the drawing.
	Load(k Key) ([]byte, error)
	// Save stores a picture as the drawing's new version by principal, and
	// returns its ID.
	Save(k Key, principal string, png []byte) (item.ID, error)
}

// Participant is a client of a session, as others see it.
type Participant struct {
	Client    string `json:"client"`
	Principal string `json:"principal"`
	Name      string `json:"name"`
	Colour    int    `json:"colour"`
	Agent     bool   `json:"agent,omitempty"`
	Editable  bool   `json:"editable,omitempty"`
}

// LockState is who holds an element; Client is empty when it is free.
type LockState struct {
	ID     string `json:"id"`
	Client string `json:"client"`
}

// Pointer is a client's cursor on the scene.
type Pointer struct {
	Client string  `json:"client"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
}

// TimerState is the session's countdown (ADR-0168). Ends is when it ends,
// in Unix milliseconds, while it runs; Left the milliseconds left while
// it is paused.
type TimerState struct {
	Running bool   `json:"running"`
	Paused  bool   `json:"paused"`
	Ends    int64  `json:"ends,omitempty"`
	Left    int64  `json:"left,omitempty"`
	Total   int64  `json:"total,omitempty"`
	By      string `json:"by,omitempty"`
}

// VoteState is a vote as one participant sees it (ADR-0168): their own
// votes while it is open, everyone's counts once it is closed.
type VoteState struct {
	Open    bool           `json:"open"`
	Closed  bool           `json:"closed"`
	Title   string         `json:"title,omitempty"`
	Per     int            `json:"per,omitempty"`
	Left    int            `json:"left,omitempty"`
	Mine    map[string]int `json:"mine,omitempty"`
	Voters  int            `json:"voters,omitempty"`
	Results map[string]int `json:"results,omitempty"`
	By      string         `json:"by,omitempty"`
}

// PresentationState is who presents and what (ADR-0168): a frame, or
// their view where the scene has no frames.
type PresentationState struct {
	Active bool    `json:"active"`
	Client string  `json:"client,omitempty"`
	Name   string  `json:"name,omitempty"`
	Frame  string  `json:"frame,omitempty"`
	X      float64 `json:"x,omitempty"`
	Y      float64 `json:"y,omitempty"`
	Zoom   float64 `json:"zoom,omitempty"`
}

// Saved tells of the newest checkpoint.
type Saved struct {
	At    int64   `json:"at"`
	By    string  `json:"by"`
	File  item.ID `json:"file"`
	Stale bool    `json:"stale,omitempty"`
}

// Event kinds, the names of the server-sent events.
const (
	Snapshot     = "snapshot"     // the first event of a client
	Elements     = "elements"     // elements changed
	Files        = "files"        // image files added
	Locks        = "locks"        // locks taken or released
	Presence     = "presence"     // participants
	Pointers     = "pointers"     // cursor positions
	Timer        = "timer"        // the countdown
	Vote         = "vote"         // the vote, as the receiver sees it
	Presentation = "presentation" // who presents what
	CheckpointEv = "checkpoint"   // to the keeper: draw the scene as it is now
	SavedEv      = "saved"        // a checkpoint was stored
	Comments     = "comments"     // a comment was written; read them again
	Closed       = "closed"       // the stream ends
)

// Event is one message to the browsers of a session.
type Event struct {
	Kind         string                     `json:"-"`
	Seq          int                        `json:"seq"`
	Now          int64                      `json:"now,omitempty"`
	Client       string                     `json:"client,omitempty"`
	Drawing      item.ID                    `json:"drawing,omitempty"`
	Editable     bool                       `json:"editable,omitempty"`
	Keeper       bool                       `json:"keeper,omitempty"`
	By           string                     `json:"by,omitempty"`
	Elements     []json.RawMessage          `json:"elements,omitempty"`
	Files        map[string]json.RawMessage `json:"files,omitempty"`
	Background   string                     `json:"background,omitempty"`
	Locks        []LockState                `json:"locks,omitempty"`
	Participants []Participant              `json:"participants,omitempty"`
	Pointers     []Pointer                  `json:"pointers,omitempty"`
	Timer        *TimerState                `json:"timer,omitempty"`
	Vote         *VoteState                 `json:"vote,omitempty"`
	Presentation *PresentationState         `json:"presentation,omitempty"`
	Saved        *Saved                     `json:"saved,omitempty"`
	Token        string                     `json:"token,omitempty"`
	Reason       string                     `json:"reason,omitempty"`
}

type element struct {
	raw     json.RawMessage
	m       meta
	holder  *client
	touched time.Time
}

type client struct {
	id, principal, name string
	ch                  chan Event // nil for an agent
	editable            bool
	joined              int
	last                time.Time
	pointer             *Pointer
}

func (c *client) agent() bool { return c.ch == nil }

type vote struct {
	open, closed bool
	title, by    string
	per          int
	votes        map[string]map[string]int // principal → element → votes
	results      map[string]int
}

type checkpoint struct {
	token string
	at    time.Time
	scene Scene
	seq   int // the change it shows
}

type session struct {
	key      Key
	order    []string
	elements map[string]*element
	files    map[string]json.RawMessage
	bg       string
	bytes    int64
	clients  map[string]*client
	agents   map[string]*client // by principal
	colours  map[string]int     // by principal
	joins    int
	seq      int

	// change counts the changes of the scene; saved is the count the
	// newest checkpoint holds.
	change, saved int
	firstDirty    time.Time
	lastChange    time.Time
	lastChanger   string
	stale         bool
	saveAsked     bool
	pending       *checkpoint
	saving        bool
	lastSaved     *Saved

	pointerDirty bool
	timer        TimerState
	vote         vote
	present      *PresentationState
	presenter    *client
}

// Hub holds every live whiteboard.
type Hub struct {
	mu       sync.Mutex
	b        Backend
	now      func() time.Time
	log      *slog.Logger
	sessions map[Key]*session
	// after schedules the sending of cursor positions; tests replace it.
	after func(time.Duration, func())
}

// Option configures a Hub.
type Option func(*Hub)

// WithClock replaces time.Now, for tests.
func WithClock(now func() time.Time) Option { return func(h *Hub) { h.now = now } }

// WithLog sets where failed saves are reported.
func WithLog(l *slog.Logger) Option { return func(h *Hub) { h.log = l } }

// NewHub returns a hub over a backend.
func NewHub(b Backend, opts ...Option) *Hub {
	h := &Hub{b: b, now: time.Now, log: slog.New(slog.DiscardHandler), sessions: map[Key]*session{},
		after: func(d time.Duration, f func()) { time.AfterFunc(d, f) }}
	for _, o := range opts {
		o(h)
	}
	return h
}

func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// Sub is a browser's subscription to a session.
type Sub struct {
	// Client identifies the browser in operations.
	Client string
	// C delivers the events, starting with a snapshot. It is closed when
	// the browser leaves, falls behind, loses access or the session ends.
	C   <-chan Event
	hub *Hub
	key Key
}

// Leave ends the subscription and releases the browser's locks.
func (s *Sub) Leave() { s.hub.leave(s.key, s.Client) }

// open returns the session of k, loading the drawing if none is open.
func (h *Hub) open(k Key) (*session, error) {
	if s := h.sessions[k]; s != nil {
		return s, nil
	}
	if len(h.sessions) >= budget.Get().WhiteboardSessions {
		return nil, ErrBusy
	}
	p, err := h.b.Load(k)
	if err != nil {
		return nil, err
	}
	sc, stale, err := ReadScene(p, budget.Get().WhiteboardSceneBytes)
	if err != nil {
		return nil, err
	}
	s := &session{key: k, elements: map[string]*element{}, files: map[string]json.RawMessage{}, bg: sc.Background,
		clients: map[string]*client{}, agents: map[string]*client{}, colours: map[string]int{}, stale: stale}
	ordered, last := true, ""
	for _, raw := range sc.Elements {
		m, err := parseElement(raw)
		if err != nil || s.elements[m.ID] != nil {
			continue // what the server cannot name it cannot lock; Excalidraw drops it too
		}
		if m.Index == nil || !validIndex(*m.Index) || *m.Index <= last {
			ordered = false
		} else {
			last = *m.Index
		}
		s.elements[m.ID] = &element{raw: raw, m: m}
		s.order = append(s.order, m.ID)
		s.bytes += int64(len(raw))
	}
	if !ordered {
		// A scene saved before fractional indices, or out of order: the
		// server orders it once, before anyone sees it.
		idx := ""
		for _, id := range s.order {
			e := s.elements[id]
			idx = indexAfter(idx)
			before := int64(len(e.raw))
			e.raw = setIndex(e.raw, idx)
			v := idx
			e.m.Index = &v
			s.bytes += int64(len(e.raw)) - before
		}
	}
	for id, f := range sc.Files {
		if elementIDRE.MatchString(id) {
			s.files[id] = f
			s.bytes += int64(len(f))
		}
	}
	h.sessions[k] = s
	return s, nil
}

// Join adds a browser of principal (shown as name) to the whiteboard's
// session, opening it if needed. Reading is enough to join; editing is
// checked per operation.
func (h *Hub) Join(k Key, principal, name string) (*Sub, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	read, edit := h.b.Access(k, principal)
	if !read {
		return nil, fmt.Errorf("%w: whiteboard %s/%s", item.ErrNotFound, k.Item, k.First)
	}
	s, err := h.open(k)
	if err != nil {
		return nil, err
	}
	if len(s.clients) >= budget.Get().WhiteboardClients {
		h.gc(s)
		return nil, ErrBusy
	}
	s.joins++
	c := &client{id: newID(), principal: principal, name: name, ch: make(chan Event, buffer), editable: edit, joined: s.joins, last: h.now()}
	h.add(s, c)
	h.send(s, c, h.snapshot(s, c))
	h.presenceAfterChange(s, c)
	return &Sub{Client: c.id, C: c.ch, hub: h, key: k}, nil
}

func (h *Hub) add(s *session, c *client) {
	s.clients[c.id] = c
	if _, ok := s.colours[c.principal]; !ok {
		used := map[int]bool{}
		for _, n := range s.colours {
			used[n] = true
		}
		n := 0
		for used[n] && n < colours {
			n++
		}
		s.colours[c.principal] = n % colours
	}
}

func (h *Hub) snapshot(s *session, c *client) Event {
	var els []json.RawMessage
	for _, id := range s.order {
		if e := s.elements[id]; !e.m.IsDeleted {
			els = append(els, e.raw)
		}
	}
	timer := s.timer
	v := h.voteFor(s, c.principal)
	p := h.presentation(s)
	return Event{Kind: Snapshot, Now: h.now().UnixMilli(), Client: c.id, Drawing: s.key.First, Editable: c.editable, Keeper: h.keeper(s) == c,
		Elements: els, Files: maps.Clone(s.files), Background: s.bg, Locks: s.locks(), Participants: h.participants(s),
		Timer: &timer, Vote: &v, Presentation: &p, Saved: s.lastSaved}
}

func (s *session) locks() []LockState {
	var out []LockState
	for _, id := range s.order {
		if e := s.elements[id]; e.holder != nil {
			out = append(out, LockState{ID: id, Client: e.holder.id})
		}
	}
	return out
}

func (h *Hub) participants(s *session) []Participant {
	out := make([]Participant, 0, len(s.clients))
	for _, c := range s.clients {
		out = append(out, Participant{Client: c.id, Principal: c.principal, Name: c.name, Colour: s.colours[c.principal], Agent: c.agent(), Editable: c.editable})
	}
	slices.SortFunc(out, func(a, b Participant) int {
		if c := strings.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		return strings.Compare(a.Client, b.Client)
	})
	return out
}

// keeper is the editing browser that joined first: it draws the
// checkpoints.
func (h *Hub) keeper(s *session) *client {
	var k *client
	for _, c := range s.clients {
		if !c.agent() && c.editable && (k == nil || c.joined < k.joined) {
			k = c
		}
	}
	return k
}

// send delivers an event to one browser; one that has fallen too far
// behind is dropped rather than slowing everyone down.
func (h *Hub) send(s *session, c *client, e Event) {
	if c.agent() {
		return
	}
	s.seq++
	e.Seq = s.seq
	select {
	case c.ch <- e:
	default:
		h.drop(s, c)
	}
}

func (h *Hub) broadcast(s *session, e Event, except *client) {
	for _, c := range s.clients {
		if c != except {
			h.send(s, c, e)
		}
	}
}

// drop removes a client and frees its locks; a presenter ends presenting.
func (h *Hub) drop(s *session, c *client) {
	if _, ok := s.clients[c.id]; !ok {
		return
	}
	delete(s.clients, c.id)
	if c.agent() {
		delete(s.agents, c.principal)
	} else {
		close(c.ch)
	}
	var freed []LockState
	for _, id := range s.order {
		if e := s.elements[id]; e.holder == c {
			e.holder = nil
			freed = append(freed, LockState{ID: id})
		}
	}
	if len(freed) > 0 {
		h.broadcast(s, Event{Kind: Locks, Locks: freed}, nil)
	}
	if s.presenter == c {
		s.presenter, s.present = nil, nil
		p := h.presentation(s)
		h.broadcast(s, Event{Kind: Presentation, Presentation: &p}, nil)
	}
	if s.pending != nil && h.keeper(s) == nil {
		s.pending = nil
	}
}

func (h *Hub) leave(k Key, id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.sessions[k]
	if s == nil {
		return
	}
	c := s.clients[id]
	if c == nil {
		return
	}
	h.drop(s, c)
	h.presenceAfterChange(s)
	h.gc(s)
}

// presenceAfterChange tells everyone who is in the session, and each
// browser whether it keeps the checkpoints now.
// A client that just had its snapshot (except) knows already.
func (h *Hub) presenceAfterChange(s *session, except ...*client) {
	keeper := h.keeper(s)
	parts := h.participants(s)
	for _, c := range s.clients {
		if !slices.Contains(except, c) {
			h.send(s, c, Event{Kind: Presence, Keeper: keeper == c, Participants: parts})
		}
	}
}

// gc closes a session nobody is in and nothing waits to be saved in.
func (h *Hub) gc(s *session) {
	if len(s.clients) == 0 && s.change == s.saved && !s.saving {
		delete(h.sessions, s.key)
	}
}

// Op is one operation of a browser.
type Op struct {
	Op string `json:"op"`
	// put: an element as Excalidraw holds it; file: an image file.
	Element json.RawMessage `json:"element,omitempty"`
	File    json.RawMessage `json:"file,omitempty"`
	// lock, release: elements.
	IDs []string `json:"ids,omitempty"`
	// pointer, view: a point of the scene; view: and a zoom.
	X    float64 `json:"x,omitempty"`
	Y    float64 `json:"y,omitempty"`
	Zoom float64 `json:"zoom,omitempty"`
	// timer, vote, present: what to do, and with what.
	Action  string `json:"action,omitempty"`
	Seconds int    `json:"seconds,omitempty"`
	Title   string `json:"title,omitempty"`
	Votes   int    `json:"votes,omitempty"`
	Target  string `json:"target,omitempty"`
	// background: the scene's background colour.
	Background string `json:"background,omitempty"`
}

// Refusal is an element change or lock the server did not take: Current
// is the element as the server holds it, By who holds it.
type Refusal struct {
	ID      string          `json:"id"`
	Current json.RawMessage `json:"current,omitempty"`
	By      string          `json:"by,omitempty"`
}

// Result is the answer to a batch of operations.
type Result struct {
	Refused []Refusal `json:"refused,omitempty"`
}

// Apply runs a browser's operations, in order. A malformed operation
// refuses the batch; an element held by another or changed since is
// refused alone, and the answer gives it as the server holds it.
func (h *Hub) Apply(k Key, clientID, principal string, ops []Op) (Result, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.sessions[k]
	if s == nil {
		return Result{}, ErrNotJoined
	}
	c := s.clients[clientID]
	if c == nil || c.principal != principal || c.agent() {
		return Result{}, ErrNotJoined
	}
	if len(ops) == 0 {
		return Result{}, fmt.Errorf("%w: no operations", ErrInvalid)
	}
	read, edit := h.b.Access(k, principal)
	if !read {
		h.drop(s, c)
		return Result{}, ErrNotJoined
	}
	if c.editable != edit {
		c.editable = edit
		h.presenceAfterChange(s)
	}
	c.last = h.now()
	// Everything but the cursor needs the right to edit.
	for i, op := range ops {
		if !edit && op.Op != "pointer" && op.Op != "vote" {
			return Result{}, ErrForbidden
		}
		if err := h.check(s, op); err != nil {
			return Result{}, fmt.Errorf("operation %d: %w", i+1, err)
		}
	}
	var res Result
	var changed []json.RawMessage
	var locks []LockState
	files := map[string]json.RawMessage{}
	for _, op := range ops {
		switch op.Op {
		case "put":
			m, _ := parseElement(op.Element)
			if r, ok := h.put(s, c, op.Element, m); !ok {
				res.Refused = append(res.Refused, r)
			} else {
				changed = append(changed, op.Element)
			}
		case "file":
			var f struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(op.File, &f)
			if _, ok := s.files[f.ID]; !ok {
				if s.bytes+int64(len(op.File)) > budget.Get().WhiteboardSceneBytes {
					return res, ErrBusy
				}
				s.files[f.ID] = op.File
				s.bytes += int64(len(op.File))
				files[f.ID] = op.File
			}
		case "lock":
			for _, id := range op.IDs {
				e := s.elements[id]
				switch {
				case e == nil:
				case e.holder != nil && e.holder != c:
					res.Refused = append(res.Refused, Refusal{ID: id, Current: e.raw, By: e.holder.name})
				default:
					if e.holder == nil {
						locks = append(locks, LockState{ID: id, Client: c.id})
					}
					e.holder, e.touched = c, c.last
				}
			}
		case "release":
			for _, id := range op.IDs {
				if e := s.elements[id]; e != nil && e.holder == c {
					e.holder = nil
					locks = append(locks, LockState{ID: id})
				}
			}
		case "pointer":
			c.pointer = &Pointer{Client: c.id, X: op.X, Y: op.Y}
			h.schedulePointers(s)
		case "background":
			s.bg = op.Background
			h.changed(s, c)
		case "timer":
			h.timerOp(s, c, op)
		case "vote":
			if err := h.voteOp(s, c, op); err != nil {
				return res, err
			}
		case "present", "frame", "view":
			if err := h.presentOp(s, c, op); err != nil {
				return res, err
			}
		case "comments":
			h.broadcast(s, Event{Kind: Comments, By: c.name}, c)
		case "save":
			if s.change != s.saved || s.stale {
				s.saveAsked = true
			}
		}
	}
	// Files first, so an image never arrives before its picture.
	if len(files) > 0 {
		h.broadcast(s, Event{Kind: Files, Files: files}, c)
	}
	if len(changed) > 0 {
		h.broadcast(s, Event{Kind: Elements, Elements: changed, By: c.id}, c)
	}
	if len(locks) > 0 {
		h.broadcast(s, Event{Kind: Locks, Locks: locks}, nil)
	}
	return res, nil
}

// check refuses a malformed operation before any of a batch runs.
func (h *Hub) check(s *session, op Op) error {
	switch op.Op {
	case "put":
		if _, err := parseElement(op.Element); err != nil {
			return err
		}
	case "file":
		var f struct {
			ID       string `json:"id"`
			MimeType string `json:"mimeType"`
			DataURL  string `json:"dataURL"`
		}
		if err := json.Unmarshal(op.File, &f); err != nil || !elementIDRE.MatchString(f.ID) || !imageTypes[f.MimeType] ||
			!strings.HasPrefix(f.DataURL, "data:"+f.MimeType+";base64,") {
			return fmt.Errorf("%w: a file is an image with its ID, type and data", ErrInvalid)
		}
	case "lock", "release":
		for _, id := range op.IDs {
			if !elementIDRE.MatchString(id) {
				return fmt.Errorf("%w: element ID %q", ErrInvalid, id)
			}
		}
	case "background":
		if _, err := colour(op.Background, ""); err != nil || op.Background == "" {
			return fmt.Errorf("%w: background %q", ErrInvalid, op.Background)
		}
	case "pointer", "timer", "vote", "present", "frame", "view", "comments", "save":
	default:
		return fmt.Errorf("%w: unknown operation %q", ErrInvalid, op.Op)
	}
	return nil
}

// imageTypes are the image files a scene may carry, as Excalidraw takes
// them.
var imageTypes = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/svg+xml": true, "image/gif": true, "image/webp": true, "image/bmp": true, "image/x-icon": true, "image/avif": true,
}

// put takes an element's new version, unless someone else holds it or the
// server holds a newer one.
func (h *Hub) put(s *session, c *client, raw json.RawMessage, m meta) (Refusal, bool) {
	e := s.elements[m.ID]
	switch {
	case e != nil && e.holder != nil && e.holder != c:
		return Refusal{ID: m.ID, Current: e.raw, By: e.holder.name}, false
	case e != nil && m.Version <= e.m.Version:
		return Refusal{ID: m.ID, Current: e.raw}, false
	case e == nil && len(s.elements) >= budget.Get().WhiteboardElements,
		s.bytes+int64(len(raw))-size(e) > budget.Get().WhiteboardSceneBytes:
		return Refusal{ID: m.ID, Current: currentRaw(e)}, false
	}
	if e == nil {
		e = &element{}
		s.elements[m.ID] = e
		s.order = append(s.order, m.ID)
	}
	s.bytes += int64(len(raw)) - size(e)
	e.raw, e.m = raw, m
	if e.holder == c {
		e.touched = h.now()
	}
	h.changed(s, c)
	return Refusal{}, true
}

func size(e *element) int64 {
	if e == nil {
		return 0
	}
	return int64(len(e.raw))
}

func currentRaw(e *element) json.RawMessage {
	if e == nil {
		return nil
	}
	return e.raw
}

// changed counts a change of the scene by c.
func (h *Hub) changed(s *session, c *client) {
	now := h.now()
	if s.change == s.saved {
		s.firstDirty = now
	}
	s.change++
	s.lastChange, s.lastChanger = now, c.principal
}

func (h *Hub) schedulePointers(s *session) {
	if s.pointerDirty {
		return
	}
	s.pointerDirty = true
	k := s.key
	h.after(pointerEvery, func() { h.flushPointers(k) })
}

// flushPointers sends the cursors that moved since the last time.
func (h *Hub) flushPointers(k Key) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.sessions[k]
	if s == nil || !s.pointerDirty {
		return
	}
	s.pointerDirty = false
	var ps []Pointer
	for _, c := range s.clients {
		if c.pointer != nil {
			ps = append(ps, *c.pointer)
			c.pointer = nil
		}
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].Client < ps[j].Client })
	if len(ps) > 0 {
		h.broadcast(s, Event{Kind: Pointers, Pointers: ps}, nil)
	}
}

// scene is a copy of the session's scene as a checkpoint holds it: the
// elements that are not deleted, by their index, and the files they show.
func (s *session) scene() Scene {
	type ordered struct {
		pos int
		e   *element
	}
	var els []ordered
	for i, id := range s.order {
		if e := s.elements[id]; !e.m.IsDeleted {
			els = append(els, ordered{i, e})
		}
	}
	sort.SliceStable(els, func(i, j int) bool {
		a, b := str(els[i].e.m.Index), str(els[j].e.m.Index)
		if a == b {
			return els[i].pos < els[j].pos
		}
		return a < b
	})
	sc := Scene{Files: map[string]json.RawMessage{}, Background: s.bg}
	for _, o := range els {
		sc.Elements = append(sc.Elements, o.e.raw)
		if o.e.m.Type == "image" && o.e.m.FileID != nil {
			if f, ok := s.files[*o.e.m.FileID]; ok {
				sc.Files[*o.e.m.FileID] = f
			}
		}
	}
	return sc
}

// Checkpoint stores the picture the keeper drew for the checkpoint token
// asked for. The server writes its own copy of the scene into it, so the
// version holds what the session held at that event.
func (h *Hub) Checkpoint(k Key, clientID, principal, token string, png []byte) (Saved, error) {
	h.mu.Lock()
	s := h.sessions[k]
	if s == nil || s.clients[clientID] == nil || s.clients[clientID].principal != principal {
		h.mu.Unlock()
		return Saved{}, ErrNotJoined
	}
	if _, edit := h.b.Access(k, principal); !edit {
		h.mu.Unlock()
		return Saved{}, ErrForbidden
	}
	cp := s.pending
	if cp == nil || cp.token != token || s.saving {
		h.mu.Unlock()
		return Saved{}, ErrStale
	}
	s.pending, s.saving = nil, true
	name := s.clients[clientID].name
	h.mu.Unlock()

	out, err := WriteScene(png, cp.scene, false)
	var file item.ID
	if err == nil {
		file, err = h.b.Save(k, principal, out)
	}
	return h.saved(k, cp.seq, name, file, false, err)
}

// saved records the end of a save.
func (h *Hub) saved(k Key, seq int, by string, file item.ID, stale bool, err error) (Saved, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.sessions[k]
	if s == nil {
		return Saved{}, err
	}
	s.saving = false
	if err != nil {
		h.log.Warn("a whiteboard was not saved", "item", k.Item, "drawing", k.First, "err", err)
		h.gc(s)
		return Saved{}, err
	}
	if seq > s.saved {
		s.saved = seq
	}
	if s.change != s.saved {
		s.firstDirty = h.now()
	}
	s.stale, s.saveAsked = stale, false
	sv := Saved{At: h.now().UnixMilli(), By: by, File: file, Stale: stale}
	s.lastSaved = &sv
	h.broadcast(s, Event{Kind: SavedEv, Saved: &sv}, nil)
	h.gc(s)
	return sv, nil
}

// serverSave writes the session's scene into the newest picture, marked
// stale, as the one who changed it last; it runs without the hub's lock.
func (h *Hub) serverSave(k Key, sc Scene, seq int, principal string) {
	p, err := h.b.Load(k)
	if err == nil {
		p, err = WriteScene(p, sc, true)
	}
	var file item.ID
	if err == nil {
		file, err = h.b.Save(k, principal, p)
	}
	_, _ = h.saved(k, seq, "", file, true, err)
}

type job struct {
	key       Key
	scene     Scene
	seq       int
	principal string
}

// Tick lapses idle locks and agents, asks keepers for checkpoints and
// saves scenes nobody can draw; the saves run before it returns.
func (h *Hub) Tick() {
	h.mu.Lock()
	now := h.now()
	var jobs []job
	for _, s := range h.sessions {
		h.lapse(s, now)
		if j, ok := h.plan(s, now); ok {
			jobs = append(jobs, j)
		}
		h.gc(s)
	}
	h.mu.Unlock()
	for _, j := range jobs {
		h.serverSave(j.key, j.scene, j.seq, j.principal)
	}
}

func (h *Hub) lapse(s *session, now time.Time) {
	var freed []LockState
	for _, id := range s.order {
		if e := s.elements[id]; e.holder != nil && now.Sub(e.touched) >= LockTTL && now.Sub(e.holder.last) >= LockTTL {
			e.holder = nil
			freed = append(freed, LockState{ID: id})
		}
	}
	if len(freed) > 0 {
		h.broadcast(s, Event{Kind: Locks, Locks: freed}, nil)
	}
	gone := false
	for _, c := range s.clients {
		if c.agent() && now.Sub(c.last) >= AgentTTL {
			h.drop(s, c)
			gone = true
		}
	}
	if gone {
		h.presenceAfterChange(s)
	}
}

// plan decides how the session is saved now: by asking its keeper, or,
// with none, by the server; ok says the server saves.
func (h *Hub) plan(s *session, now time.Time) (job, bool) {
	if s.saving {
		return job{}, false
	}
	dirty := s.change != s.saved
	keeper := h.keeper(s)
	if keeper != nil {
		if s.pending != nil && now.Sub(s.pending.at) < CheckpointWait {
			return job{}, false
		}
		live := false
		for _, e := range s.elements {
			if !e.m.IsDeleted {
				live = true
				break
			}
		}
		due := dirty && (s.saveAsked || now.Sub(s.lastChange) >= QuietSave || now.Sub(s.firstDirty) >= LongSave)
		if due || (s.stale && live && (s.saveAsked || now.Sub(s.lastChange) >= QuietSave)) {
			s.pending = &checkpoint{token: newID(), at: now, scene: s.scene(), seq: s.change}
			h.send(s, keeper, Event{Kind: CheckpointEv, Token: s.pending.token})
		}
		return job{}, false
	}
	s.pending = nil
	if dirty && s.lastChanger != "" && (len(s.clients) == 0 || now.Sub(s.lastChange) >= AloneSave) {
		s.saving = true
		return job{key: s.key, scene: s.scene(), seq: s.change, principal: s.lastChanger}, true
	}
	return job{}, false
}

// Revalidate checks every client's access again, after permissions or the
// item changed: those who may no longer read leave, and those who may no
// longer edit lose their locks. A session whose drawing is gone thus
// loses everyone and closes, unless it holds changes to save, which then
// fail and are reported.
func (h *Hub) Revalidate() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, s := range h.sessions {
		changed := false
		for _, c := range s.clients {
			read, edit := h.b.Access(s.key, c.principal)
			switch {
			case !read:
				h.send(s, c, Event{Kind: Closed, Reason: "you may no longer read this whiteboard"})
				h.drop(s, c)
				changed = true
			case edit != c.editable:
				c.editable, changed = edit, true
				if !edit {
					var freed []LockState
					for _, id := range s.order {
						if e := s.elements[id]; e.holder == c {
							e.holder = nil
							freed = append(freed, LockState{ID: id})
						}
					}
					if len(freed) > 0 {
						h.broadcast(s, Event{Kind: Locks, Locks: freed}, nil)
					}
				}
			}
		}
		if changed {
			h.presenceAfterChange(s)
		}
		h.gc(s)
	}
}

func (h *Hub) close(s *session, reason string) {
	for _, c := range s.clients {
		h.send(s, c, Event{Kind: Closed, Reason: reason})
		h.drop(s, c)
	}
	delete(h.sessions, s.key)
}

// Shutdown ends every session for a server that is stopping: changes not
// yet saved are written into the newest picture, marked stale.
func (h *Hub) Shutdown() {
	h.mu.Lock()
	var jobs []job
	for _, s := range h.sessions {
		if s.change != s.saved && s.lastChanger != "" && !s.saving {
			s.saving = true
			jobs = append(jobs, job{key: s.key, scene: s.scene(), seq: s.change, principal: s.lastChanger})
		}
		for _, c := range s.clients {
			h.send(s, c, Event{Kind: Closed, Reason: "the server is shutting down"})
			h.drop(s, c)
		}
	}
	h.mu.Unlock()
	for _, j := range jobs {
		h.serverSave(j.key, j.scene, j.seq, j.principal)
	}
	h.mu.Lock()
	clear(h.sessions)
	h.mu.Unlock()
}

// Info describes an open session, for the list of live whiteboards.
type Info struct {
	Key          Key
	Participants int
}

// Sessions lists the open sessions with their participants.
func (h *Hub) Sessions() []Info {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]Info, 0, len(h.sessions))
	for k, s := range h.sessions {
		people := map[string]bool{}
		for _, c := range s.clients {
			people[c.principal] = true
		}
		out = append(out, Info{Key: k, Participants: len(people)})
	}
	return out
}

// Stats reports the open sessions and their clients, for operational
// metrics (ADR-0016).
func (h *Hub) Stats() (sessions, clients int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, s := range h.sessions {
		clients += len(s.clients)
	}
	return len(h.sessions), clients
}
