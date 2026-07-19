package ocr

import "github.com/shrigum/adamic/src/library"

// Store persists one document's OCR result keyed by its library identity
// (task T6; spec A4, AC4, AC12). It is deliberately narrow (design-review
// condition C3): a Save/Load pair, not a general document-metadata
// repository. The file-backed implementation lives in the store subpackage;
// the SQLite store (ADR-0008) later implements this same interface with no
// caller change. Failure modes are soft for readers: a load failure means "no
// OCR yet", never a crash (spec error summary).
type Store interface {
	// Load returns the stored OCR result for id and whether one existed. A
	// document that was never OCR'd returns ok=false and a nil error.
	Load(id library.DocID) (result Result, ok bool, err error)

	// Save writes result under result.ID, replacing any previous result for
	// that document.
	Save(result Result) error
}
