package site

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/dbuchs7/orbithek/internal/share"
)

// Share binds an album folder to a file share, which the folder shows as
// previews (ADR-0197): a directory of the server, such as a mounted Samba
// share, or an S3 bucket. The share is the master; the wiki only reads
// it. Its ID names the share in every entry it makes, so it does not
// change.
type Share struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Address is an absolute directory of the server or
	// s3://bucket/prefix?endpoint=https://…&region=….
	Address string `json:"address"`
	// Folder is the ID of the album folder the share fills (D2).
	Folder string `json:"folder"`
	// Link is a template with {path} that opens an original where it
	// lies, https: or smb:; empty gives no link.
	Link          string `json:"link,omitempty"`
	Enabled       bool   `json:"enabled"`
	IntervalHours int    `json:"intervalHours"`
	// Device is the device a directory lay on when it was bound, so that
	// an unmounted share is not read as empty; 0 for S3 or where the
	// system does not tell.
	Device uint64 `json:"device,omitempty"`
	// SignedLinks lets readers of the folder open an S3 share's originals
	// through links the server signs for minutes, when they follow them
	// (ADR-0223); off by default.
	SignedLinks bool `json:"signedLinks,omitempty"`
}

// maxShares bounds how many shares a site keeps.
const maxShares = 20

// Share intervals, in hours.
const (
	ShareIntervalMin     = 1
	ShareIntervalMax     = 7 * 24
	ShareIntervalDefault = 6
)

// folderIDRE is the form of an item ID (item.ValidItemID).
var folderIDRE = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Kind answers how the share is reached, "" for an address that does not
// parse.
func (sh Share) Kind() share.Kind {
	a, err := share.ParseAddress(sh.Address)
	if err != nil {
		return ""
	}
	return a.Kind
}

// Normalize trims the share's text and fills its default interval.
func (sh Share) Normalize() Share {
	sh.ID = strings.TrimSpace(sh.ID)
	sh.Name = strings.TrimSpace(sh.Name)
	sh.Address = strings.TrimSpace(sh.Address)
	sh.Folder = strings.TrimSpace(sh.Folder)
	sh.Link = strings.TrimSpace(sh.Link)
	if sh.IntervalHours == 0 {
		sh.IntervalHours = ShareIntervalDefault
	}
	if sh.Kind() != share.Dir {
		sh.Device = 0
	}
	return sh
}

// Validate checks what the settings alone can tell. Whether a directory
// lies where the operator allows, and whether the folder exists and holds
// no pictures of its own, the server checks when a share is set.
func (sh Share) Validate() error {
	if !connectorIDRE.MatchString(sh.ID) {
		return fmt.Errorf("%w: a share's ID has lower-case letters, digits, dots and hyphens, at most 40", ErrInvalid)
	}
	if sh.Name == "" || len([]rune(sh.Name)) > 80 || strings.ContainsAny(sh.Name, "<>\n\r\t") {
		return fmt.Errorf("%w: a share's name is a single line of 1 to 80 characters", ErrInvalid)
	}
	if _, err := share.ParseAddress(sh.Address); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if !folderIDRE.MatchString(sh.Folder) {
		return fmt.Errorf("%w: a share fills an album folder, named by its ID", ErrInvalid)
	}
	if sh.Link != "" && !share.LinkTemplateOK(sh.Link) {
		return fmt.Errorf("%w: a link is an https: or smb: address with {path}, such as smb://nas/photos/{path}", ErrInvalid)
	}
	if sh.SignedLinks && sh.Kind() != share.S3 {
		return fmt.Errorf("%w: only an S3 share signs links to its originals", ErrInvalid)
	}
	if sh.IntervalHours < ShareIntervalMin || sh.IntervalHours > ShareIntervalMax {
		return fmt.Errorf("%w: a share is read every %d to %d hours", ErrInvalid, ShareIntervalMin, ShareIntervalMax)
	}
	return nil
}

// Shares returns the shares by name.
func (s *Settings) Shares() []Share {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Share, 0, len(s.shares))
	for _, sh := range s.shares {
		out = append(out, sh)
	}
	slices.SortFunc(out, func(a, b Share) int { return cmp.Or(strings.Compare(a.Name, b.Name), strings.Compare(a.ID, b.ID)) })
	return out
}

// Share returns one share.
func (s *Settings) Share(id string) (Share, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sh, ok := s.shares[id]
	return sh, ok
}

// ShareOfFolder returns the share a folder is bound to.
func (s *Settings) ShareOfFolder(folder string) (Share, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sh := range s.shares {
		if sh.Folder == folder {
			return sh, true
		}
	}
	return Share{}, false
}

// SetShare adds a share or replaces the one of its ID, and returns it as
// kept. A folder shows one share only, and a share keeps its folder: its
// entries are files of that folder.
func (s *Settings) SetShare(actor string, sh Share) (Share, error) {
	sh = sh.Normalize()
	if err := sh.Validate(); err != nil {
		return sh, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, exists := s.shares[sh.ID]
	if !exists && len(s.shares) >= maxShares {
		return sh, fmt.Errorf("%w: at most %d shares", ErrInvalid, maxShares)
	}
	if exists && old.Folder != sh.Folder {
		return sh, fmt.Errorf("%w: a share keeps its folder; remove it and add it again to fill another", ErrInvalid)
	}
	for _, other := range s.shares {
		if other.ID != sh.ID && other.Folder == sh.Folder {
			return sh, fmt.Errorf("%w: the folder shows the share %s already", ErrInvalid, other.Name)
		}
	}
	return sh, s.record(event{Kind: shareSet, Actor: actor, Share: &sh})
}

// RemoveShare removes a share. The server removes its entries first.
func (s *Settings) RemoveShare(actor, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.shares[id]; !ok {
		return fmt.Errorf("%w: no share %q", ErrInvalid, id)
	}
	return s.record(event{Kind: shareRemoved, Actor: actor, Asset: id})
}
