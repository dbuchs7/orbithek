package site

// Places is the administrators' setting of the place search the album
// suggests places from when a place is set by hand (ADR-0181). The search
// itself is the operator's choice (OT_GEOCODER); administrators turn its
// use off and on. It is on unless turned off, so the zero value is on.
type Places struct {
	SearchOff bool `json:"searchOff"`
}

// Places answers the setting of the place search.
func (s *Settings) Places() Places {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.places
}

// SetPlaces changes the setting of the place search.
func (s *Settings) SetPlaces(actor string, p Places) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.places == p {
		return nil
	}
	return s.record(event{Kind: placesSet, Actor: actor, Places: &p})
}
