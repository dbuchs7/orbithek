package extcal

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

// Limits of reading: how much one calendar may answer, how long a read
// may take, how long what was read is kept, and how long a failure is.
const (
	maxBody       = 8 << 20
	readTimeout   = 20 * time.Second
	keepFor       = 15 * time.Minute
	keepFailure   = 2 * time.Minute
	readersAtOnce = 4
)

// ErrUnreadable is a calendar that cannot be read now: its service is
// away, refuses, or answers something that is no calendar. The message
// says why, without the address.
var ErrUnreadable = errors.New("extcal: the calendar cannot be read")

// Reader reads people's calendars from their services and keeps what it
// read for a while, so a calendar shown again is not read again at once.
// It is safe for concurrent use.
type Reader struct {
	store  *Store
	client *http.Client
	google *Google
	now    func() time.Time

	mu    sync.Mutex
	cache map[string]cached
}

// cached is what was read of a calendar, or why it could not be.
type cached struct {
	at     time.Time
	cal    *Calendar
	events []Event
	err    error
}

// NewReader reads with client, which must keep to the addresses the
// server may reach (connector.PublicOnly, ADR-0174); google may be nil
// when Google is not set up.
func NewReader(store *Store, client *http.Client, google *Google) *Reader {
	r := &Reader{store: store, client: client, google: google, now: time.Now, cache: map[string]cached{}}
	if google != nil {
		google.reader = r
	}
	return r
}

// Store is the store the reader reads people's calendars from.
func (r *Reader) Store() *Store { return r.store }

// Google is Google's set-up, nil if there is none.
func (r *Reader) Google() *Google { return r.google }

// Shown is one occurrence of one of a person's calendars.
type Shown struct {
	Source string
	Event
}

// Problem is a calendar that could not be read.
type Problem struct {
	Source  string
	Message string
}

// Events reads the person's shown calendars and answers their occurrences
// in the window [from, to), sorted by their start, with the calendars
// that could not be read. reader is the person's zone, for times a
// calendar gives without one.
func (r *Reader) Events(ctx context.Context, user string, from, to time.Time, reader *time.Location) ([]Shown, []Problem) {
	var (
		mu       sync.Mutex
		out      []Shown
		problems []Problem
		wg       sync.WaitGroup
	)
	slots := make(chan struct{}, readersAtOnce)
	for _, s := range r.store.Sources(user) {
		if s.Hidden {
			continue
		}
		wg.Add(1)
		go func(s Source) {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			evs, err := r.read(ctx, user, s, from, to, reader)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				problems = append(problems, Problem{Source: s.ID, Message: err.Error()})
				return
			}
			for _, e := range evs {
				out = append(out, Shown{Source: s.ID, Event: e})
			}
		}(s)
	}
	wg.Wait()
	slices.SortStableFunc(out, func(a, b Shown) int {
		return cmp.Or(a.Start.Compare(b.Start), cmp.Compare(a.Source, b.Source), cmp.Compare(a.Title, b.Title))
	})
	slices.SortFunc(problems, func(a, b Problem) int { return cmp.Compare(a.Source, b.Source) })
	return out, problems
}

// span is the part of time read from a service that is asked by time —
// Google and CalDAV —: the window's months with a week on either side, so
// that moving within a month reads nothing again.
func span(from, to time.Time) (time.Time, time.Time) {
	f, t := from.UTC(), to.UTC()
	start := time.Date(f.Year(), f.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -7)
	end := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 7)
	return start, end
}

// read answers one calendar's occurrences in the window, from what was
// read before while it is fresh.
func (r *Reader) read(ctx context.Context, user string, s Source, from, to time.Time, reader *time.Location) ([]Event, error) {
	start, end := span(from, to)
	key := user + "\x00" + s.ID
	if s.Kind != KindICS {
		key += "\x00" + start.Format(time.DateOnly) + "\x00" + end.Format(time.DateOnly)
	}
	r.mu.Lock()
	c, ok := r.cache[key]
	r.mu.Unlock()
	age := r.now().Sub(c.at)
	if !ok || c.err == nil && age >= keepFor || c.err != nil && age >= keepFailure {
		c = cached{at: r.now()}
		ctx, cancel := context.WithTimeout(ctx, readTimeout)
		switch s.Kind {
		case KindICS:
			c.cal, c.err = r.readICS(ctx, user, s)
		case KindCalDAV:
			c.cal, c.err = r.readCalDAV(ctx, user, s, start, end)
		case KindGoogle:
			c.events, c.err = r.readGoogle(ctx, user, s, start, end)
		default:
			c.err = fmt.Errorf("%w: a calendar of the kind %q", ErrUnreadable, s.Kind)
		}
		cancel()
		r.mu.Lock()
		r.cache[key] = c
		r.mu.Unlock()
	}
	if c.err != nil {
		return nil, c.err
	}
	if c.cal != nil {
		return c.cal.Events(from, to, reader), nil
	}
	var out []Event
	for _, e := range c.events {
		if touches(e, from, to, cmp.Or(reader, time.UTC)) {
			out = append(out, e)
		}
	}
	return out, nil
}

// Forget drops what was read of a person's calendar, as when it is
// removed or changed.
func (r *Reader) Forget(user, id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	prefix := user + "\x00" + id
	for k := range r.cache {
		if k == prefix || strings.HasPrefix(k, prefix+"\x00") {
			delete(r.cache, k)
		}
	}
}

// readICS reads a calendar's iCalendar address.
func (r *Reader) readICS(ctx context.Context, user string, s Source) (*Calendar, error) {
	addr, err := r.store.Secret(user, s.ID)
	if err != nil {
		return nil, fmt.Errorf("%w: its address is no longer kept", ErrUnreadable)
	}
	return r.Fetch(ctx, addr)
}

// Fetch reads an iCalendar address, as when a person adds it and it is
// tried first.
func (r *Reader) Fetch(ctx context.Context, addr string) (*Calendar, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	req.Header.Set("Accept", "text/calendar, */*;q=0.5")
	req.Header.Set("User-Agent", "orbithek-calendar")
	data, err := r.do(req)
	if err != nil {
		return nil, err
	}
	cal, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%w: the address answers no calendar", ErrUnreadable)
	}
	return cal, nil
}

// do sends a request and reads its answer, at most maxBody; an answer
// that is no success is an error that names its status, never the
// address, which may be a credential.
func (r *Reader) do(req *http.Request) ([]byte, error) {
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnreadable, reason(err))
	}
	return body(resp)
}

// body reads an answer, at most maxBody, and closes it.
func body(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return nil, &StatusError{Status: resp.StatusCode}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnreadable, reason(err))
	}
	if len(data) > maxBody {
		return nil, fmt.Errorf("%w: the calendar is larger than %d MB", ErrUnreadable, maxBody>>20)
	}
	return data, nil
}

// StatusError is an answer that is no success.
type StatusError struct{ Status int }

func (e *StatusError) Error() string {
	switch e.Status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Sprintf("%v: the service refuses access (%d)", ErrUnreadable, e.Status)
	case http.StatusNotFound, http.StatusGone:
		return fmt.Sprintf("%v: the service knows no such calendar (%d)", ErrUnreadable, e.Status)
	}
	return fmt.Sprintf("%v: the service answers %d", ErrUnreadable, e.Status)
}

func (e *StatusError) Unwrap() error { return ErrUnreadable }

// reason says why a request failed without the address it went to: an
// *url.Error names it, and an iCalendar address is a credential.
func reason(err error) string {
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "the service did not answer in time"
	}
	if msg := err.Error(); !strings.Contains(msg, "://") {
		return msg
	}
	return "the service cannot be reached"
}
