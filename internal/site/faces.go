package site

// Faces is the administrators' setting of face recognition in the album
// (ADR-0194). Finding faces and naming them is always offered; recognising
// a person again keeps what the browser computed of each face, biometric
// data, and is off unless administrators turn it on. The zero value is
// off.
type Faces struct {
	Recognition bool `json:"recognition"`
}

// Faces answers the setting of face recognition.
func (s *Settings) Faces() Faces {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.faces
}

// SetFaces changes the setting of face recognition.
func (s *Settings) SetFaces(actor string, f Faces) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.faces == f {
		return nil
	}
	return s.record(event{Kind: facesSet, Actor: actor, Faces: &f})
}
