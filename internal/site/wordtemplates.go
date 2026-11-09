package site

import (
	"cmp"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// WordTemplate is a Word template types write their pages' documents with
// (ADR-0167). Every version is kept: a page remembers the version it was
// made with, and exports with that one until someone moves it to a newer.
type WordTemplate struct {
	ID       string                `json:"id"`
	Name     string                `json:"name"`
	Versions []WordTemplateVersion `json:"versions"`
}

// WordTemplateVersion is one uploaded file of a template.
type WordTemplateVersion struct {
	Version int       `json:"version"`
	File    string    `json:"file"`
	At      time.Time `json:"at"`
	Actor   string    `json:"actor"`
	Size    int       `json:"size"`
	// Tag names the content, for caching.
	Tag  string `json:"tag"`
	data []byte
}

// wordTemplateEvent is what WordTemplateSet records besides the file,
// which is the event's Data.
type wordTemplateEvent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	File string `json:"file,omitempty"`
}

// ErrNotFound marks a template that is not there.
var ErrNotFound = errors.New("not found")

const (
	wordTemplateSet     = "WordTemplateSet"
	wordTemplateRemoved = "WordTemplateRemoved"
)

// maxWordTemplates bounds the templates an installation keeps: the shape
// of the site's settings, like the number of connectors.
const maxWordTemplates = 100

var wordTemplateID = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)

// ValidWordTemplateID reports whether an ID is one a template may have:
// lower-case letters, digits and dashes, starting with a letter.
func ValidWordTemplateID(id string) bool { return wordTemplateID.MatchString(id) }

func checkWordTemplateName(name string) error {
	if strings.TrimSpace(name) == "" || strings.ContainsAny(name, "<>\n\r") || len([]rune(name)) > 80 {
		return fmt.Errorf("%w: a template's name is a single line of 1 to 80 characters", ErrInvalid)
	}
	return nil
}

// cleanFileName keeps the last part of an uploaded file's name, on one
// line, as it is shown.
func cleanFileName(name string) string {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(`<>"`, r) {
			return -1
		}
		return r
	}, name)
	if r := []rune(name); len(r) > 120 {
		name = string(r[:120])
	}
	return cmp.Or(strings.TrimSpace(name), "template.dotx")
}

func (s *Settings) applyWordTemplate(e event) error {
	switch e.Kind {
	case wordTemplateSet:
		if e.WordTemplate == nil {
			return errors.New("a word template event without a template")
		}
		w := e.WordTemplate
		t := s.wordTemplates[w.ID]
		if t == nil {
			if len(e.Data) == 0 {
				return fmt.Errorf("word template %q named before it has a file", w.ID)
			}
			t = &WordTemplate{ID: w.ID}
			s.wordTemplates[w.ID] = t
		}
		t.Name = w.Name
		if len(e.Data) > 0 {
			t.Versions = append(t.Versions, WordTemplateVersion{Version: len(t.Versions) + 1, File: w.File, At: e.At, Actor: e.Actor, Size: len(e.Data), Tag: tag(e.Data), data: e.Data})
		}
	case wordTemplateRemoved:
		delete(s.wordTemplates, e.Asset)
	}
	return nil
}

// WordTemplates lists the templates by name, their versions without
// their files.
func (s *Settings) WordTemplates() []WordTemplate {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]WordTemplate, 0, len(s.wordTemplates))
	for _, t := range s.wordTemplates {
		out = append(out, t.public())
	}
	slices.SortFunc(out, func(a, b WordTemplate) int {
		return cmp.Or(strings.Compare(a.Name, b.Name), strings.Compare(a.ID, b.ID))
	})
	return out
}

func (t *WordTemplate) public() WordTemplate {
	c := WordTemplate{ID: t.ID, Name: t.Name, Versions: make([]WordTemplateVersion, len(t.Versions))}
	for i, v := range t.Versions {
		v.data = nil
		c.Versions[i] = v
	}
	return c
}

// WordTemplate returns a template, its versions without their files.
func (s *Settings) WordTemplate(id string) (WordTemplate, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.wordTemplates[id]
	if !ok {
		return WordTemplate{}, false
	}
	return t.public(), true
}

// NewestWordTemplate is the newest version of a template, 0 when there is
// no such template.
func (s *Settings) NewestWordTemplate(id string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.wordTemplates[id]; ok {
		return len(t.Versions)
	}
	return 0
}

// WordTemplateFile returns a version of a template with its file; version
// 0 is the newest.
func (s *Settings) WordTemplateFile(id string, version int) (WordTemplateVersion, []byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.wordTemplates[id]
	if !ok || version < 0 || version > len(t.Versions) || len(t.Versions) == 0 {
		return WordTemplateVersion{}, nil, false
	}
	if version == 0 {
		version = len(t.Versions)
	}
	v := t.Versions[version-1]
	data := v.data
	v.data = nil
	return v, data, true
}

// AddWordTemplate stores a new version of a template, creating it when
// the ID is new; it returns the version. Checking the file is the
// caller's concern.
func (s *Settings) AddWordTemplate(actor, id, name, file string, data []byte) (int, error) {
	if !ValidWordTemplateID(id) {
		return 0, fmt.Errorf("%w: a template's ID has lower-case letters, digits and dashes, starting with a letter, at most 40", ErrInvalid)
	}
	if err := checkWordTemplateName(name); err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, fmt.Errorf("%w: the file is empty", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.wordTemplates[id]; !ok && len(s.wordTemplates) >= maxWordTemplates {
		return 0, fmt.Errorf("%w: at most %d templates", ErrInvalid, maxWordTemplates)
	}
	err := s.record(event{Kind: wordTemplateSet, Actor: actor, WordTemplate: &wordTemplateEvent{ID: id, Name: strings.TrimSpace(name), File: cleanFileName(file)}, Data: slices.Clone(data)})
	if err != nil {
		return 0, err
	}
	return len(s.wordTemplates[id].Versions), nil
}

// RenameWordTemplate changes a template's name.
func (s *Settings) RenameWordTemplate(actor, id, name string) error {
	if err := checkWordTemplateName(name); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.wordTemplates[id]; !ok {
		return fmt.Errorf("%w: no template %q", ErrNotFound, id)
	}
	return s.record(event{Kind: wordTemplateSet, Actor: actor, WordTemplate: &wordTemplateEvent{ID: id, Name: strings.TrimSpace(name)}})
}

// RemoveWordTemplate removes a template with every version; whether
// nothing uses it is the caller's concern.
func (s *Settings) RemoveWordTemplate(actor, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.wordTemplates[id]; !ok {
		return fmt.Errorf("%w: no template %q", ErrNotFound, id)
	}
	return s.record(event{Kind: wordTemplateRemoved, Actor: actor, Asset: id})
}
