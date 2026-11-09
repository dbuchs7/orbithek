package site

// Calendars is whether people may show their calendars of other services
// — iCalendar addresses, Google and CalDAV accounts — in the wiki's
// calendar (ADR-0174). It is off until an administrator turns it on, as
// the server then reads addresses people give it and keeps their
// credentials.
type Calendars struct {
	External bool `json:"external"`
}

// Calendars answers the setting of calendars of other services.
func (s *Settings) Calendars() Calendars {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calendars
}

// SetCalendars changes the setting of calendars of other services.
func (s *Settings) SetCalendars(actor string, c Calendars) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.calendars == c {
		return nil
	}
	return s.record(event{Kind: calendarsSet, Actor: actor, Calendars: &c})
}
