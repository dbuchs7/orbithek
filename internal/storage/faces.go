package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dbuchs7/orbithek/internal/item"
)

// FaceVectorLen is the length of what the browser computes of a face to
// recognise it (ADR-0194): 128 numbers, as the recognition network of the
// vendored face-api gives them.
const FaceVectorLen = 128

// maxFaceVectorBytes bounds a file of face vectors: every face of a
// picture, each number written with a few digits.
const maxFaceVectorBytes = item.MaxFaces * FaceVectorLen * 12

// StoreFaceVectors keeps what was computed of the faces of a picture,
// in the order of its faces, sealed with the item's key beside the file
// (ADR-0194); a face nothing was computed of has an empty vector, and
// nil or only empty ones remove them. The caller has checked that the
// picture is a recorded file of the item.
func (db *DB) StoreFaceVectors(id, picture item.ID, vectors [][]float32) error {
	if !item.ValidAttachmentID(string(picture)) {
		return fmt.Errorf("storage: attachment ID %q", picture)
	}
	if !slices.ContainsFunc(vectors, func(v []float32) bool { return len(v) > 0 }) {
		db.DiscardFaceVectors(id, picture)
		return nil
	}
	if len(vectors) > item.MaxFaces {
		return fmt.Errorf("storage: more than %d faces", item.MaxFaces)
	}
	rounded := make([][]float64, len(vectors))
	for i, v := range vectors {
		if len(v) != 0 && len(v) != FaceVectorLen {
			return fmt.Errorf("storage: a face vector has %d numbers, not %d", len(v), FaceVectorLen)
		}
		rounded[i] = make([]float64, len(v))
		for j, x := range v {
			if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
				return errors.New("storage: a face vector holds no number")
			}
			rounded[i][j] = math.Round(float64(x)*1e5) / 1e5
		}
	}
	data, err := json.Marshal(rounded)
	if err != nil {
		return err
	}
	_, err = db.sealAttachment(id, picture+FacesSuffix, bytes.NewReader(data), maxFaceVectorBytes)
	return err
}

// FaceVectors reads what was computed of the faces of a picture, or an
// error that matches os.ErrNotExist when nothing was.
func (db *DB) FaceVectors(id, picture item.ID) ([][]float32, error) {
	if !item.ValidAttachmentID(string(picture)) {
		return nil, fmt.Errorf("storage: attachment %q: %w", picture, os.ErrNotExist)
	}
	f, err := db.openFile(id, picture+FacesSuffix)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxFaceVectorBytes+1))
	if err != nil {
		return nil, err
	}
	var out [][]float32
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("storage: face vectors of %s: %w", picture, err)
	}
	return out, nil
}

// DiscardFaceVectors removes what was computed of the faces of a picture.
func (db *DB) DiscardFaceVectors(id, picture item.ID) {
	if item.ValidAttachmentID(string(picture)) {
		os.Remove(attachmentPath(db.dir, id, picture+FacesSuffix))
	}
}

// ForgetFaceVectors removes what was computed of every face of every
// picture, when recognition is turned off (ADR-0194). It answers how many
// pictures' vectors it removed.
func (db *DB) ForgetFaceVectors() (int, error) {
	root := filepath.Join(db.dir, AttachmentsDir)
	removed := 0
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), FacesSuffix) {
			if err := os.Remove(path); err != nil {
				return err
			}
			removed++
		}
		return nil
	})
	return removed, err
}
