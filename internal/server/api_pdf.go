package server

import (
	"fmt"
	"net/http"

	"github.com/dbuchs7/orbithek/internal/item"
)

// pdfRoutes serve the PDF reader (ADR-0208). The reader draws a PDF in the
// browser from the file's ordinary download; its highlights are comments
// anchored in the PDF (commentRoutes).
func pdfRoutes(h itemHandlers) []Route {
	return []Route{
		{Method: http.MethodGet, Pattern: "/api/items/{ref}/pdfs/{file}", Summary: "A PDF file of the item, named by any of its versions, for the PDF reader (ADR-0208): its first version, by which highlights name it, its newest version and its versions, oldest first; a file that is no PDF is not found", Handler: http.HandlerFunc(h.pdf)},
	}
}

// pdfJSON is a PDF with its versions.
type pdfJSON struct {
	First    item.ID          `json:"first"`
	Newest   attachmentJSON   `json:"newest"`
	Versions []attachmentJSON `json:"versions"`
}

func (h itemHandlers) pdf(w http.ResponseWriter, r *http.Request) {
	h.read(w, r, func(x *item.Session) (any, error) {
		it, err := resolveItem(x, r.PathValue("ref"))
		if err != nil {
			return nil, err
		}
		// A protected file is no one's but its readers' (ADR-0160): x.Drawing
		// answers it as not found.
		d, err := x.Drawing(it.ID, item.ID(r.PathValue("file")))
		if err != nil {
			return nil, err
		}
		if !item.IsPDF(d.Newest) {
			return nil, fmt.Errorf("%w: %s is no PDF", item.ErrNotFound, r.PathValue("file"))
		}
		out := pdfJSON{First: d.First, Newest: toAttachmentJSON(d.Newest), Versions: []attachmentJSON{}}
		files, err := x.Attachments(it.ID)
		if err != nil {
			return nil, err
		}
		for _, a := range files {
			for _, v := range d.Versions {
				if a.ID == v {
					out.Versions = append(out.Versions, toAttachmentJSON(a))
				}
			}
		}
		return out, nil
	})
}
