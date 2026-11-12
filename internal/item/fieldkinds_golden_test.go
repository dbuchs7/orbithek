package item

import "testing"

// fieldKindsLog defines a room type (ADR-0201) with a field of every kind
// of ADR-0202 and sets their values, so the golden files pin Space,
// Targets, references and lists.
func fieldKindsLog(t *testing.T) []Event {
	s := setup(t)
	noErr(t, s.DefineType("admin", TypeDef{Name: "incident", Space: "PLAT", Body: true, Fields: []FieldDef{
		{Name: "about", Kind: KindItem, Targets: []string{"page"}},
		{Name: "platforms", Kind: KindMulti, Values: []string{"web", "ios", "android"}},
		{Name: "rating", Kind: KindRating},
		{Name: "cover", Kind: KindFile},
	}}))
	page := mustItem(t)(s.Create("anna", NewItem{Space: "PLAT", Type: "page", Title: "Runbook"}))
	it := mustItem(t)(s.Create("anna", NewItem{Space: "PLAT", Type: "incident", Title: "Outage",
		Properties: Props{"about": page.ID, "platforms": []string{"ios", "web"}}}))
	noErr(t, s.SetProperty("anna", it.ID, "rating", 4))
	_, err := s.AddAttachment("anna", it.ID, testFile(fileA, "cover.png"))
	noErr(t, err)
	noErr(t, s.SetProperty("anna", it.ID, "cover", fileA))
	noErr(t, s.SetProperty("anna", it.ID, "platforms", []string{"android"}))
	return s.Log()
}
