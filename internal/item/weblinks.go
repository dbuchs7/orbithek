package item

// WebLinkType is the name of the ready-made type of web links (ADR-0206):
// an item for a page of the web, with what the page says about itself in
// its fields and one's own notes in its body.
const WebLinkType = "weblink"

// webLinkTemplate is the ready-made type of web links. Its fields are the
// ones a page names in its metadata; none is required, so a link is saved
// while the server reads no pages (OT_LINK_PREVIEWS off).
func webLinkTemplate() TypeDef {
	return TypeDef{Name: WebLinkType, Body: true, Fields: []FieldDef{
		{Name: "url", Kind: KindURL},
		{Name: "site", Kind: KindText},
		{Name: "description", Kind: KindText},
		{Name: "author", Kind: KindText},
		{Name: "published", Kind: KindDate},
	}}
}

// TemplateType returns the ready-made type of a name, if there is one.
func TemplateType(name string) (TypeDef, bool) {
	for _, td := range TypeTemplates() {
		if td.Name == name {
			return td, true
		}
	}
	return TypeDef{}, false
}

// CreateFromTemplate creates an item where the principal may create, as
// Create does. When no type of the item's type name is defined and a
// ready-made type of that name exists, the type is defined from it in the
// same batch as the item (ADR-0206), as the task of an access request
// defines the task type (ADR-0162): one who may create in a room saves a
// web link before an administrator has added the type. A refused item
// leaves no type behind.
func (x *Session) CreateFromTemplate(n NewItem) (Item, error) {
	tmpl, ok := TemplateType(n.Type)
	if _, defined := x.s.latest(n.Type); defined || !ok {
		return x.Create(n)
	}
	if err := x.canCreateIn(n.Space, n.Parent); err != nil {
		return Item{}, err
	}
	if n.Body != nil {
		for _, r := range n.Body.refs() {
			if _, err := x.readable(r.To); err != nil {
				return Item{}, err
			}
		}
	}
	n.Watch = true
	return x.s.createDefining(x.who, n, tmpl)
}

// createDefining defines td and creates an item of it, as one batch.
func (s *Store) createDefining(actor string, n NewItem, td TypeDef) (Item, error) {
	if err := td.validate(); err != nil {
		return Item{}, err
	}
	td.Version = len(s.types[td.Name]) + 1
	e, err := s.prepareCreateOf(n, td)
	if err != nil {
		return Item{}, err
	}
	events := []Event{{Kind: TypeDefined, TypeDef: &td}, e}
	if n.Body != nil {
		body, err := s.firstBodyOf(td, e, *n.Body)
		if err != nil {
			return Item{}, err
		}
		events = append(events, body)
	}
	if err := s.recordBatch(actor, append(events, watchOnCreate(actor, n, e)...)...); err != nil {
		return Item{}, err
	}
	return s.items[e.Item].clone(), nil
}
