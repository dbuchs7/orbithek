// Package certs holds the certificates the server presents when it
// terminates TLS itself (ADR-0242): pairs of PEM files,
// a certificate chain and its key. The name a client asks for (SNI)
// chooses the pair, and a pair is read again when its files change, so a
// renewal needs no restart.
package certs

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

// Pair names the files of one certificate: its chain, the server's
// certificate first, and the private key that belongs to it.
type Pair struct{ Cert, Key string }

// Pairs matches certificate files with key files given in the same order.
// Certificates without their keys are an error rather than a server that
// answers in the clear on a port somebody believed secured.
func Pairs(certFiles, keyFiles []string) ([]Pair, error) {
	if len(certFiles) != len(keyFiles) {
		return nil, fmt.Errorf("%d certificate files but %d key files: name a key for every certificate, in the same order", len(certFiles), len(keyFiles))
	}
	var pairs []Pair
	for i := range certFiles {
		pairs = append(pairs, Pair{Cert: certFiles[i], Key: keyFiles[i]})
	}
	return pairs, nil
}

// Store serves the certificates of its pairs.
type Store struct {
	log *slog.Logger

	mu      sync.Mutex
	entries []*entry
}

type entry struct {
	pair Pair
	cert *tls.Certificate
	// The modification times the pair was last read at, or failed to be
	// read at: a change that cannot be read is tried again when the files
	// change once more, not on every handshake.
	certMod, keyMod time.Time
}

// Open reads every pair, so a wrong path or a key that does not belong to
// its certificate stops the server at its start rather than at the first
// handshake, and logs what each certificate is for.
func Open(pairs []Pair, log *slog.Logger) (*Store, error) {
	if len(pairs) == 0 {
		return nil, errors.New("no certificate named")
	}
	s := &Store{log: log}
	for _, p := range pairs {
		certMod, keyMod, err := modTimes(p)
		if err != nil {
			return nil, err
		}
		cert, err := load(p)
		if err != nil {
			return nil, err
		}
		s.entries = append(s.entries, &entry{pair: p, cert: cert, certMod: certMod, keyMod: keyMod})
		log.Info("serving a certificate", "cert", p.Cert, "names", names(cert.Leaf), "not-after", cert.Leaf.NotAfter.UTC().Format(time.RFC3339))
	}
	return s, nil
}

// Config is the server's whole TLS configuration: TLS 1.3 alone, and the
// certificate chosen per handshake. TLS 1.3 fixes its ciphers in the
// protocol, so there is no list to set and nothing to weaken; a client
// that cannot speak it is refused rather than served an older version.
func (s *Store) Config() *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, GetCertificate: s.GetCertificate}
}

// GetCertificate returns the first certificate made for the name the
// client asks for that the client can also verify; a name no certificate
// is made for, and a client that names none, get the first pair's.
// Changed files are read first: two file lookups per pair and handshake
// are what a renewal without a restart costs.
func (s *Store) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.entries {
		s.refresh(e)
	}
	for _, e := range s.entries {
		if hello.SupportsCertificate(e.cert) == nil {
			return e.cert, nil
		}
	}
	return s.entries[0].cert, nil
}

// refresh reads e's pair again when its files changed. A change that
// cannot be read is a warning, not an outage: the usual cause is a renewal
// caught half written, and the certificate loaded is still the valid one.
func (s *Store) refresh(e *entry) {
	certMod, keyMod, err := modTimes(e.pair)
	if err == nil && certMod.Equal(e.certMod) && keyMod.Equal(e.keyMod) {
		return
	}
	if err == nil {
		var cert *tls.Certificate
		if cert, err = load(e.pair); err == nil {
			e.cert, e.certMod, e.keyMod = cert, certMod, keyMod
			s.log.Info("read the changed certificate", "cert", e.pair.Cert, "names", names(cert.Leaf), "not-after", cert.Leaf.NotAfter.UTC().Format(time.RFC3339))
			return
		}
	}
	if certMod.Equal(e.certMod) && keyMod.Equal(e.keyMod) {
		return // warned about this state before
	}
	e.certMod, e.keyMod = certMod, keyMod
	s.log.Warn("cannot read the changed certificate; serving the one loaded before", "cert", e.pair.Cert, "err", err)
}

// modTimes reports when the pair's files last changed; a file that cannot
// be looked up reads as the zero time beside the error.
func modTimes(p Pair) (certMod, keyMod time.Time, err error) {
	cs, certErr := os.Stat(p.Cert)
	if certErr == nil {
		certMod = cs.ModTime()
	}
	ks, keyErr := os.Stat(p.Key)
	if keyErr == nil {
		keyMod = ks.ModTime()
	}
	return certMod, keyMod, errors.Join(certErr, keyErr)
}

func load(p Pair) (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(p.Cert, p.Key)
	if err != nil {
		return nil, fmt.Errorf("certificate %s with key %s: %w", p.Cert, p.Key, err)
	}
	return &cert, nil
}

// names lists what a certificate is made for.
func names(leaf *x509.Certificate) string {
	all := append([]string(nil), leaf.DNSNames...)
	for _, ip := range leaf.IPAddresses {
		all = append(all, ip.String())
	}
	return strings.Join(all, ",")
}

// Leaves reads the server certificate, the first of the chain, of every
// certificate file.
func Leaves(certFiles []string) ([]*x509.Certificate, error) {
	var leaves []*x509.Certificate
	for _, f := range certFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var block *pem.Block
		for block, data = pem.Decode(data); block != nil && block.Type != "CERTIFICATE"; block, data = pem.Decode(data) {
		}
		if block == nil {
			return nil, fmt.Errorf("%s holds no certificate", f)
		}
		leaf, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		leaves = append(leaves, leaf)
	}
	return leaves, nil
}

// Pinned is the configuration of a client that trusts exactly the server
// certificates in certFiles, by whatever name or address it reaches the
// server: the liveness probe asks the server on the loopback address,
// which no certificate names. It checks neither names nor dates, so it
// suits only a caller that was given the server's own files.
func Pinned(certFiles []string) (*tls.Config, error) {
	leaves, err := Leaves(certFiles)
	if err != nil {
		return nil, err
	}
	if len(leaves) == 0 {
		return nil, errors.New("no certificate named")
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS13,
		// The standard verification by name is replaced, not dropped:
		// VerifyConnection accepts the pinned certificates and nothing else.
		InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			for _, leaf := range leaves {
				if len(cs.PeerCertificates) > 0 && bytes.Equal(cs.PeerCertificates[0].Raw, leaf.Raw) {
					return nil
				}
			}
			return errors.New("the server presented a certificate that is not one of those configured")
		},
	}, nil
}

// Roots are the certificate authorities a command that calls a server
// verifies it against: the host's, with those of the PEM file caFile
// added (ADR-draft-ca-bundle). Without a file it returns nil, which
// crypto/tls reads as the host's authorities.
//
// Added, never replacing: an organisation's own authority is why this
// exists, and a caller that also reaches servers with publicly issued
// certificates must not lose them by naming its own. It is no way around
// verification, and there is none: a switch that skips it would be reached
// for when a certificate is wrong, which is when it must not exist.
func Roots(caFile string) (*x509.CertPool, error) {
	caFile = strings.TrimSpace(caFile)
	if caFile == "" {
		return nil, nil
	}
	bundle, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("the host's certificate authorities: %w", err)
	}
	if !pool.AppendCertsFromPEM(bundle) {
		return nil, fmt.Errorf("%s holds no certificate", caFile)
	}
	return pool, nil
}
