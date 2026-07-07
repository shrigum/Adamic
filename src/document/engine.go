package document

import (
	"errors"
	"fmt"
	"image"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	pdfium_errors "github.com/klippa-app/go-pdfium/errors"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"

	"github.com/shrigum/adamic/src/library"
	"github.com/shrigum/adamic/src/reader"
)

// Engine is the Document Engine: it satisfies reader.Reader by driving PDFium
// through go-pdfium's WebAssembly backend (ADR-0012), so no cgo and no C
// toolchain are required (proven by the C1 spike). One Engine owns one wasm
// pool; it is safe for concurrent commands (guarded by mu) but, per spec A8,
// the product opens one document at a time.
//
// Failure modes are owned in doc.go's package comment: soft, classified
// *reader.OpenError on open; ErrClosedDocument / ErrPageOutOfRange on
// misordered or out-of-range commands; never a panic on a bad file.
type Engine struct {
	pool     pdfium.Pool
	instance pdfium.Pdfium
	store    library.Store // reading-position persistence (T10); may be nil

	mu   sync.Mutex
	docs map[reader.DocumentID]openDoc
	seq  uint64
}

// openDoc tracks one open document: its PDFium handle plus the library identity
// under which its reading position is saved and restored.
type openDoc struct {
	ref references.FPDF_DOCUMENT
	id  library.DocID
}

var _ reader.Reader = (*Engine)(nil)

// NewEngine starts the PDFium wasm pool and returns an Engine backed by the
// file-based reading-position store. Call Shutdown to release it. The
// single-instance pool matches the one-document-at-a-time model (A8); a larger
// pool is a later performance lever (T11), not a correctness one.
func NewEngine() (*Engine, error) {
	return NewEngineWithStore(library.FileStore{})
}

// NewEngineWithStore is NewEngine with an explicit position store, for tests
// and for swapping in the SQLite-backed store later (ADR-0008). A nil store
// disables position persistence (Open always starts at page 1, Save is a no-op)
// — the reader still works, which is the soft-failure contract for persistence.
func NewEngineWithStore(store library.Store) (*Engine, error) {
	pool, err := webassembly.Init(webassembly.Config{MinIdle: 1, MaxIdle: 1, MaxTotal: 1})
	if err != nil {
		return nil, fmt.Errorf("start PDFium wasm pool: %w", err)
	}
	instance, err := pool.GetInstance(30 * time.Second)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("acquire PDFium instance: %w", err)
	}
	return &Engine{
		pool:     pool,
		instance: instance,
		store:    store,
		docs:     map[reader.DocumentID]openDoc{},
	}, nil
}

// Shutdown closes every open document and releases the wasm pool. After
// Shutdown the Engine must not be used.
func (e *Engine) Shutdown() error {
	e.mu.Lock()
	for id, od := range e.docs {
		e.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: od.ref})
		delete(e.docs, id)
	}
	e.mu.Unlock()
	if err := e.instance.Close(); err != nil {
		e.pool.Close()
		return fmt.Errorf("close PDFium instance: %w", err)
	}
	return e.pool.Close()
}

// Open loads a PDF and returns its handle, page geometry, and the reading
// position to restore — the saved position if this document was read before,
// or the zero position (page 1) if not (spec AC7). Soft failures are classified
// into *reader.OpenError so the frontend can show a specific message and stay
// up (spec AC9/AC10). A position-store failure is soft: the document still
// opens, it just starts at page 1.
func (e *Engine) Open(path string) (*reader.Document, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	// Distinguish "missing" up front: PDFium's ErrFile lumps missing and
	// unreadable together, but AC9 wants a distinct not-found message.
	data, err := os.ReadFile(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &reader.OpenError{Path: abs, Kind: reader.OpenNotFound, Err: err}
		}
		return nil, &reader.OpenError{Path: abs, Kind: reader.OpenCorrupt, Err: err}
	}

	doc, err := e.instance.OpenDocument(&requests.OpenDocument{File: &data})
	if err != nil {
		return nil, classifyOpen(abs, data, err)
	}

	pc, err := e.instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		e.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})
		return nil, &reader.OpenError{Path: abs, Kind: reader.OpenCorrupt, Err: err}
	}

	sizes, err := e.pageSizes(doc.Document, pc.PageCount)
	if err != nil {
		e.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})
		return nil, &reader.OpenError{Path: abs, Kind: reader.OpenCorrupt, Err: err}
	}

	// Stable identity for position persistence (A4). A failure to hash is soft:
	// the document opens without a persisted position rather than not at all.
	docKey, keyErr := library.Identify(abs)

	pos := e.restore(docKey, keyErr)

	e.mu.Lock()
	e.seq++
	id := reader.DocumentID(fmt.Sprintf("doc-%d", e.seq))
	e.docs[id] = openDoc{ref: doc.Document, id: docKey}
	e.mu.Unlock()

	return &reader.Document{
		ID:       id,
		Path:     abs,
		PageInfo: reader.PageInfo{Count: pc.PageCount, Sizes: sizes},
		Position: pos,
	}, nil
}

// restore returns the saved reading position for a document, or the zero
// position when there is no store, no identity, no saved record, or the store
// read fails — all soft: a document must always open (spec AC7 / persistence
// soft-failure contract). A stored page that is now out of range (the file
// changed) is clamped to page 0 by returning zero.
func (e *Engine) restore(key library.DocID, keyErr error) reader.Position {
	if e.store == nil || keyErr != nil || key == "" {
		return reader.Position{}
	}
	rec, ok, err := e.store.Load(key)
	if err != nil || !ok {
		return reader.Position{}
	}
	if rec.Page < 0 {
		return reader.Position{}
	}
	return reader.Position{Page: rec.Page, OffsetY: rec.OffsetY}
}

// classifyOpen maps a PDFium OpenDocument error to the soft OpenError kind the
// frontend renders. PDFium reports both "not a PDF at all" and "a PDF whose
// body is broken" as ErrFormat, so we split them by the file header: a file
// that starts with the "%PDF-" marker but fails to open is corrupt/truncated;
// one without the marker is simply not a PDF (spec AC9). Anything unrecognized
// defaults to corrupt — a safe, non-crashing catch-all.
func classifyOpen(path string, data []byte, err error) error {
	kind := reader.OpenCorrupt
	switch {
	case errors.Is(err, pdfium_errors.ErrPassword):
		kind = reader.OpenPasswordReqd
	case errors.Is(err, pdfium_errors.ErrFormat):
		if hasPDFHeader(data) {
			kind = reader.OpenCorrupt
		} else {
			kind = reader.OpenNotPDF
		}
	case errors.Is(err, pdfium_errors.ErrFile):
		kind = reader.OpenCorrupt
	}
	return &reader.OpenError{Path: path, Kind: kind, Err: err}
}

// hasPDFHeader reports whether data begins with the PDF marker. Per the PDF
// spec the "%PDF-" marker must appear at the start of the file (some readers
// tolerate leading bytes, but its presence at the head is a reliable "this was
// meant to be a PDF" signal for error classification).
func hasPDFHeader(data []byte) bool {
	const marker = "%PDF-"
	if len(data) < len(marker) {
		return false
	}
	return string(data[:len(marker)]) == marker
}

func (e *Engine) pageSizes(doc references.FPDF_DOCUMENT, count int) ([]reader.PageSize, error) {
	sizes := make([]reader.PageSize, count)
	for i := 0; i < count; i++ {
		sz, err := e.instance.GetPageSize(&requests.GetPageSize{
			Page: requests.Page{ByIndex: &requests.PageByIndex{Document: doc, Index: i}},
		})
		if err != nil {
			return nil, fmt.Errorf("page %d size: %w", i, err)
		}
		sizes[i] = reader.PageSize{WidthPt: sz.Width, HeightPt: sz.Height}
	}
	return sizes, nil
}

// PageCount reports the page count of an open document.
func (e *Engine) PageCount(id reader.DocumentID) (int, error) {
	ref, err := e.ref(id)
	if err != nil {
		return 0, err
	}
	pc, err := e.instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: ref})
	if err != nil {
		return 0, fmt.Errorf("page count: %w", err)
	}
	return pc.PageCount, nil
}

// RenderPage rasterizes one page at the requested scale (T4). page is
// zero-based; out of range is ErrPageOutOfRange.
func (e *Engine) RenderPage(id reader.DocumentID, page int, scale reader.Scale) (image.Image, error) {
	ref, err := e.ref(id)
	if err != nil {
		return nil, err
	}
	pc, err := e.instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: ref})
	if err != nil {
		return nil, fmt.Errorf("page count for render: %w", err)
	}
	if page < 0 || page >= pc.PageCount {
		return nil, reader.ErrPageOutOfRange
	}

	sz, err := e.instance.GetPageSize(&requests.GetPageSize{
		Page: requests.Page{ByIndex: &requests.PageByIndex{Document: ref, Index: page}},
	})
	if err != nil {
		return nil, fmt.Errorf("page %d size: %w", page, err)
	}
	w, h := scale.PixelSize(sz.Width, sz.Height)

	render, err := e.instance.RenderPageInPixels(&requests.RenderPageInPixels{
		Page:   requests.Page{ByIndex: &requests.PageByIndex{Document: ref, Index: page}},
		Width:  w,
		Height: h,
	})
	if err != nil {
		return nil, fmt.Errorf("render page %d: %w", page, err)
	}
	defer render.Cleanup()

	// Copy the pixels out: the render result is freed by Cleanup, so the image
	// we hand back must own its backing array.
	return cloneImage(render.Result.Image), nil
}

// Thumbnail renders a small image of one page for the panel (AC6). It is a
// low, fixed zoom; independent caching/prioritization is a T8/T11 concern.
func (e *Engine) Thumbnail(id reader.DocumentID, page int) (image.Image, error) {
	return e.RenderPage(id, page, reader.Scale{Zoom: 0.2})
}

// SetPosition saves the reading position for an open document so a later Open
// restores it (spec AC7/AC8). With no store configured it is a no-op success.
// A store write failure is returned for logging but the caller treats it as
// soft — losing a saved position must not break reading.
func (e *Engine) SetPosition(id reader.DocumentID, pos reader.Position) error {
	od, err := e.lookup(id)
	if err != nil {
		return err
	}
	if e.store == nil || od.id == "" {
		return nil
	}
	pc, _ := e.PageCount(id)
	return e.store.Save(library.Record{
		ID:         od.id,
		PageCount:  pc,
		Page:       pos.Page,
		OffsetY:    pos.OffsetY,
		LastOpened: time.Now().UTC(),
	})
}

// GetPosition returns the saved position for an open document, or the zero
// position if none is stored (or no store is configured).
func (e *Engine) GetPosition(id reader.DocumentID) (reader.Position, error) {
	od, err := e.lookup(id)
	if err != nil {
		return reader.Position{}, err
	}
	return e.restore(od.id, nil), nil
}

// Close releases one document. Closing an unknown/closed handle is a no-op.
func (e *Engine) Close(id reader.DocumentID) error {
	e.mu.Lock()
	od, ok := e.docs[id]
	if ok {
		delete(e.docs, id)
	}
	e.mu.Unlock()
	if !ok {
		return nil
	}
	if _, err := e.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: od.ref}); err != nil {
		return fmt.Errorf("close document: %w", err)
	}
	return nil
}

// ref returns the PDFium handle for an open document, or ErrClosedDocument.
func (e *Engine) ref(id reader.DocumentID) (references.FPDF_DOCUMENT, error) {
	od, err := e.lookup(id)
	if err != nil {
		return references.FPDF_DOCUMENT(""), err
	}
	return od.ref, nil
}

// lookup returns the full open-document record for id, or ErrClosedDocument.
func (e *Engine) lookup(id reader.DocumentID) (openDoc, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	od, ok := e.docs[id]
	if !ok {
		return openDoc{}, reader.ErrClosedDocument
	}
	return od, nil
}

// cloneImage returns a deep copy of img as an *image.RGBA, so the returned image
// does not alias engine memory freed by render.Cleanup.
func cloneImage(img image.Image) image.Image {
	b := img.Bounds()
	out := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			out.Set(x, y, img.At(x, y))
		}
	}
	return out
}
