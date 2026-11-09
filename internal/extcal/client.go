package extcal

import (
	"errors"
	"net/http"
)

// AllowPrivateVar, set to "on", lets the server read calendars in its own
// network, such as a Nextcloud of the organisation. It is the operator's
// decision, as for connectors (ADR-0127), so it comes from the
// environment, not from the setting administrators change in the UI.
const AllowPrivateVar = "OT_CALENDAR_ALLOW_PRIVATE"

// ReadTimeout is how long reading a calendar may take.
const ReadTimeout = readTimeout

// Redirects is the redirect policy of the client calendars are read with:
// at most maxRedirects, and only to https.
func Redirects(req *http.Request, via []*http.Request) error {
	switch {
	case len(via) > maxRedirects:
		return errors.New("too many redirects")
	case req.URL.Scheme != "https":
		return errors.New("a redirect away from https")
	}
	return nil
}
