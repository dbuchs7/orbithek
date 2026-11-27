package item

// A search field in a text (ADR-0264) is a
// block of the kind search that names what it searches when it is shown:
// a room, or a page, the page with the pages below it, or the pages below
// it only. The room is named by Space and the page by Ref, each none for
// the item the text is on, so a page moved or copied takes its search
// along. Label and ItemType narrow it further to the items that carry a
// label and are of a type, as the search page's filters do; the reader
// does not change them. The block holds no item and no copy of one (D4): every reader's
// browser asks the full-text search (ADR-0033) as that reader, so a hit
// is an item the reader reads (D10). The reader may choose another scope
// while reading; nothing of that is kept.

// The scopes a search field in a text searches. None searches a room.
const (
	// ScopePage searches the page alone.
	ScopePage = "page"
	// ScopeTree searches the page and every page below it.
	ScopeTree = "tree"
	// ScopeBelow searches every page below the page, not the page.
	ScopeBelow = "below"
)

// SearchScopes are the scopes a search field names besides a room, in the
// order a person is offered them.
var SearchScopes = []string{ScopePage, ScopeTree, ScopeBelow}

// Searched names the page a search field in a text searches within, when
// it names one other than the item it is on. The page is referenced as a
// mention is: it must exist and be readable when the block is written,
// and it is not deleted while the block names it (D8).
func (b Block) Searched() (ID, bool) {
	if b.Kind == SearchBlock && b.Ref != "" {
		return b.Ref, true
	}
	return "", false
}

// checkSearchBlock validates a search field in a text.
func checkSearchBlock(b Block, self ID) error {
	switch {
	case b.Text != "" || len(b.Marks) > 0 || len(b.Mentions) > 0:
		return invalid("a search field has no text: it names what it searches")
	case b.Scope != "" && !validScope(b.Scope):
		return invalid("scope %q: a search field searches a room (none), or %s, %s or %s of a page", b.Scope, ScopePage, ScopeTree, ScopeBelow)
	case b.Scope == "" && b.Ref != "":
		return invalid("a search field of a room names the room, not a page")
	case b.Scope != "" && b.Space != "":
		return invalid("a search field of a page names the page, not a room")
	case b.Space != "" && !spaceKeyRE.MatchString(b.Space):
		return invalid("room %q: a search field names a room by its key, or none for the room of its item", b.Space)
	case b.Ref != "" && b.Ref == self:
		return invalid("a search field names its own item as none")
	case b.Label != "" && !validStoredLabel(b.Label):
		return invalid("label %q: a search field names one label, as stored (ADR-0044), or none", b.Label)
	case b.ItemType != "" && !validViewName(b.ItemType):
		return invalid("type %q: a search field names a type by its name, or none for every type", b.ItemType)
	}
	return nil
}

func validScope(s string) bool {
	for _, v := range SearchScopes {
		if v == s {
			return true
		}
	}
	return false
}
