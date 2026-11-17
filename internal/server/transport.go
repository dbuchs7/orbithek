package server

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// RedirectToHTTPS answers plain HTTP for a server that terminates TLS
// itself (ADR-draft-hsts-and-redirect): a GET or HEAD is sent to the same
// host and path over HTTPS, on httpsPort where that is not 443 (empty
// means 443). The host is the one the request names, so every name the
// server answers keeps its own.
//
// The redirect is temporary: browsers keep a permanent one without an
// end, which would outlast a server that stops terminating TLS; what makes
// HTTPS stick is the transport policy of WithHSTS, which has an end. Any
// other method is refused rather than redirected: its body and credentials
// have crossed the network in the clear already, and a client that is
// redirected would send them so again every time.
func RedirectToHTTPS(httpsPort string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
			if strings.Contains(h, ":") {
				host = "[" + h + "]"
			}
		}
		if !plainHost(host) || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
			http.Error(w, "this server answers over HTTPS only", http.StatusBadRequest)
			return
		}
		if httpsPort != "" && httpsPort != "443" {
			host = net.JoinHostPort(strings.Trim(host, "[]"), httpsPort)
		}
		to := url.URL{Scheme: "https", Host: host, Path: r.URL.Path, RawPath: r.URL.RawPath, RawQuery: r.URL.RawQuery}
		http.Redirect(w, r, to.String(), http.StatusFound)
	})
}

// plainHost reports whether host is a name or an address and nothing
// else, so that what a request calls its host cannot carry a user, a path
// or another host into the address the redirect names.
func plainHost(host string) bool {
	for i := 0; i < len(host); i++ {
		c := host[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '-', c == '.', c == '_', c == ':', c == '[', c == ']':
		default:
			return false
		}
	}
	return host != ""
}

// HSTSValue turns the setting OT_HSTS_MAX_AGE, a number of seconds, into
// the value of the Strict-Transport-Security header; empty means the
// header is not sent. Zero is valid: it tells browsers to forget the
// policy, which is how an installation leaves it. The policy never names
// subdomains and never asks to be preloaded: both reach beyond this
// server.
func HSTSValue(setting string) (string, error) {
	setting = strings.TrimSpace(setting)
	if setting == "" {
		return "", nil
	}
	seconds, err := strconv.ParseUint(setting, 10, 63)
	if err != nil {
		return "", fmt.Errorf("%q is not a number of seconds", setting)
	}
	return "max-age=" + strconv.FormatUint(seconds, 10), nil
}

// WithHSTS sends the transport policy value (RFC 6797) with every answer
// to a request that arrived over HTTPS, the server's own
// (ADR-0242) or a trusted proxy's (ADR-0037); empty sends nothing. Over
// plain HTTP the header is left out, as browsers ignore it there. It must
// stand inside BehindProxies, which tells what a proxy says.
func WithHSTS(value string, h http.Handler) http.Handler {
	if value == "" {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSecure(r) {
			w.Header().Set("Strict-Transport-Security", value)
		}
		h.ServeHTTP(w, r)
	})
}
