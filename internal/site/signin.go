package site

// SignIn is the administrators' setting of what a sign-in through single
// sign-on does when its identity is linked to no account (ADR-0229). A
// first sign-in creates an account without roles (ADR-0030) unless it is
// turned off, so the zero value keeps that behaviour. Off, only identities
// already linked to an account sign in.
type SignIn struct {
	NoNewAccounts bool `json:"noNewAccounts"`
}

// SignIn answers the setting of sign-in through single sign-on.
func (s *Settings) SignIn() SignIn {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.signIn
}

// SetSignIn changes the setting of sign-in through single sign-on.
func (s *Settings) SetSignIn(actor string, v SignIn) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.signIn == v {
		return nil
	}
	return s.record(event{Kind: signInSet, Actor: actor, SignIn: &v})
}
