package engine

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/dbuchs7/orbithek/internal/site"
)

// A person's shop at an engine (ADR-draft-a-shop-and-catalogues-from-an-engine)
// is read and used with the access token of the person's grant of it: the
// engine decides what the person sees and may do, as on its own shop page,
// and its answers are handed on as it gives them. This server names the
// routes it calls; it does not interpret a catalogue, an order or an
// approval beyond what it shows.

// ShopError is an engine's refusal of a person's shop request: its HTTP
// status, its reason in its own words, and its answer when that was a JSON
// object, such as the problems of a catalogue import.
type ShopError struct {
	Status  int
	Message string
	Body    json.RawMessage
}

func (e *ShopError) Error() string {
	return fmt.Sprintf("the engine answered %d: %s", e.Status, e.Message)
}

// shopErrorOf reads a refusal's answer, at most 64 KiB of it.
func shopErrorOf(res *http.Response) error {
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	se := &ShopError{Status: res.StatusCode, Message: engineMessage(bytes.NewReader(raw))}
	if t := bytes.TrimSpace(raw); len(t) > 0 && t[0] == '{' && json.Valid(t) {
		se.Body = json.RawMessage(t)
	}
	return se
}

// ShopAnswer is an engine's answer to a shop request: its type and body.
type ShopAnswer struct {
	ContentType string
	Body        []byte
}

// Shop makes a person's request of the engine's shop with the access
// token of their grant: method and path (with its query) name the
// engine's route, body is JSON or none, accept the types asked for, JSON
// when empty. The answer is at most budget EngineResponseBytes long.
func (r *Reader) Shop(ctx context.Context, e site.Engine, bearer, method, path string, body []byte, accept string) (ShopAnswer, error) {
	if bearer == "" {
		return ShopAnswer{}, fmt.Errorf("%w: no grant of the person", ErrNoCredential)
	}
	if !strings.HasPrefix(path, "/api/v1/") {
		return ShopAnswer{}, fmt.Errorf("%w: %s is no route of the engine's API", ErrUnavailable, path)
	}
	var a ShopAnswer
	err := r.call(ctx, e, request{method: method, path: path, accept: cmp.Or(accept, "application/json"), body: body, bearer: bearer, shop: true},
		func(contentType string, b io.Reader) error {
			raw, err := io.ReadAll(b)
			a = ShopAnswer{ContentType: contentType, Body: raw}
			return err
		})
	return a, err
}
