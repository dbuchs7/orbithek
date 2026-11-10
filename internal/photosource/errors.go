// Package photosource brings photos from other services into the album of
// the wiki (ADR-0177): from Google Photos, through the Picker API, in
// which a person chooses the photos themselves, and from public iCloud
// shared albums. It only talks to the services; the server stores what
// it downloads.
//
// Every request goes through the *http.Client the caller passes in, which
// keeps to the addresses the server may reach. Everything a service
// answers is untrusted: every read is bounded, and every address a
// service hands back is checked — https only, on a host of that service
// — before it is requested.
package photosource

import "errors"

// Errors of the photo sources. A message never carries a token or an
// address a service handed back.
var (
	// ErrInvalid is an address, a token or an argument that is not one a
	// source accepts.
	ErrInvalid = errors.New("photosource: invalid")
	// ErrNotFound is a session, an item, an album or a connection in
	// progress that does not exist, has expired or is not the caller's.
	ErrNotFound = errors.New("photosource: not found")
	// ErrNotConnected is a person who holds no valid access to Google
	// Photos: they never connected, the access token ran out, or Google
	// no longer accepts it.
	ErrNotConnected = errors.New("photosource: not connected")
	// ErrTooMany is more than a source is allowed to hold or answer: open
	// sessions, connections in progress, picked items or an album's
	// photos.
	ErrTooMany = errors.New("photosource: too many")
	// ErrTooLarge is an answer or a download longer than its limit.
	ErrTooLarge = errors.New("photosource: too large")
	// ErrUnreadable is a service that cannot be read now: it is away,
	// refuses, or answers something unexpected. The message says why with
	// the status at most.
	ErrUnreadable = errors.New("photosource: the service cannot be read")
)
