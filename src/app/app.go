// Package app is the binding layer between the web frontend and the Go core: it
// wraps a reader.Reader (the Document Engine) in methods whose arguments and
// results are JSON-serializable, so a frontend calling over the Wails boundary
// (ADR-0005) never sees Go-only types like image.Image. Page images cross the
// boundary as PNG data URLs; page geometry and reading position cross as plain
// structs.
//
// This is task T6's core (the bindable surface). It holds no PDF or persistence
// logic of its own — it translates between the frontend's JSON world and the
// reader command interface. The concrete Wails app registers an *App's methods
// as bound methods; that registration (and the window) is the thin,
// interactively-verified wiring left for the desktop build, kept out of here so
// the translation logic is unit-testable without a running window.
//
// Failure modes: an open failure is translated to an OpenResult carrying a
// machine-readable Kind and a human message (the frontend shows the message and
// stays up, spec AC9/AC10); other errors (closed document, out-of-range page)
// are returned as plain errors the frontend surfaces as programmer/logic faults.
package app

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"

	"github.com/shrigum/adamic/src/reader"
)

// App is the bound object the frontend calls. Construct it with New over a
// concrete reader.Reader (the document engine in production, a stub in tests).
type App struct {
	reader reader.Reader
}

// New returns an App bound to r.
func New(r reader.Reader) *App { return &App{reader: r} }

// OpenResult is the JSON shape returned by Open. On success Ok is true and Doc
// is populated; on a soft failure Ok is false and Error describes it for display.
type OpenResult struct {
	Ok    bool         `json:"ok"`
	Doc   *DocumentDTO `json:"doc,omitempty"`
	Error *OpenErrDTO  `json:"error,omitempty"`
}

// DocumentDTO is the frontend view of an opened document: its handle, page
// geometry (for laying out the scroll canvas before any page is rendered), and
// the reading position to restore.
type DocumentDTO struct {
	ID       string        `json:"id"`
	Path     string        `json:"path"`
	Pages    []PageSizeDTO `json:"pages"`
	Position PositionDTO   `json:"position"`
}

// PageSizeDTO is a page's intrinsic size in points (frontend computes fit modes
// and canvas size from these).
type PageSizeDTO struct {
	WidthPt  float64 `json:"widthPt"`
	HeightPt float64 `json:"heightPt"`
}

// PositionDTO is a reading position: zero-based page and within-page offset
// (fraction of page height).
type PositionDTO struct {
	Page    int     `json:"page"`
	OffsetY float64 `json:"offsetY"`
}

// OpenErrDTO is a soft open failure the frontend renders. Kind is a stable
// machine string ("not-found", "not-pdf", "corrupt", "password"); Message is
// human-facing.
type OpenErrDTO struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

// Open opens a PDF by path. It never returns a Go error for the expected soft
// failures (missing/non-PDF/corrupt/encrypted) — those come back as
// OpenResult{Ok:false} so the frontend shows a message and stays running.
func (a *App) Open(path string) OpenResult {
	doc, err := a.reader.Open(path)
	if err != nil {
		var oe *reader.OpenError
		if errors.As(err, &oe) {
			return OpenResult{Ok: false, Error: &OpenErrDTO{
				Kind:    openKindString(oe.Kind),
				Message: userMessage(oe),
			}}
		}
		// An unexpected error still must not crash the UI; surface it generically.
		return OpenResult{Ok: false, Error: &OpenErrDTO{Kind: "error", Message: err.Error()}}
	}

	pages := make([]PageSizeDTO, len(doc.PageInfo.Sizes))
	for i, s := range doc.PageInfo.Sizes {
		pages[i] = PageSizeDTO{WidthPt: s.WidthPt, HeightPt: s.HeightPt}
	}
	return OpenResult{Ok: true, Doc: &DocumentDTO{
		ID:       string(doc.ID),
		Path:     doc.Path,
		Pages:    pages,
		Position: PositionDTO{Page: doc.Position.Page, OffsetY: doc.Position.OffsetY},
	}}
}

// RenderPage returns page (zero-based) as a PNG data URL at the given zoom
// (1.0 = 100%). For fit modes the frontend passes the resolved zoom it computed
// from page geometry and its viewport, keeping this method's argument list
// JSON-simple. An out-of-range page or closed document is a returned error.
func (a *App) RenderPage(id string, page int, zoom float64) (string, error) {
	img, err := a.reader.RenderPage(reader.DocumentID(id), page, reader.Scale{Zoom: zoom})
	if err != nil {
		return "", err
	}
	return pngDataURL(img)
}

// RenderPageFit returns page as a PNG data URL sized to fit the given viewport
// (device pixels). fitPage=false fits width only; fitPage=true fits the whole
// page. This is the fit-mode counterpart to RenderPage (spec AC4), kept as a
// separate method so each has a flat, JSON-friendly signature.
func (a *App) RenderPageFit(id string, page, viewportW, viewportH int, fitPage bool) (string, error) {
	scale := reader.Scale{
		FitWidth: !fitPage,
		FitPage:  fitPage,
		Viewport: reader.Viewport{WidthPx: viewportW, HeightPx: viewportH},
	}
	img, err := a.reader.RenderPage(reader.DocumentID(id), page, scale)
	if err != nil {
		return "", err
	}
	return pngDataURL(img)
}

// Thumbnail returns page as a small fixed-width PNG data URL for the panel.
func (a *App) Thumbnail(id string, page int) (string, error) {
	img, err := a.reader.Thumbnail(reader.DocumentID(id), page)
	if err != nil {
		return "", err
	}
	return pngDataURL(img)
}

// SetPosition saves the reading position for a document.
func (a *App) SetPosition(id string, page int, offsetY float64) error {
	return a.reader.SetPosition(reader.DocumentID(id), reader.Position{Page: page, OffsetY: offsetY})
}

// GetPosition returns the saved reading position for a document.
func (a *App) GetPosition(id string) (PositionDTO, error) {
	pos, err := a.reader.GetPosition(reader.DocumentID(id))
	if err != nil {
		return PositionDTO{}, err
	}
	return PositionDTO{Page: pos.Page, OffsetY: pos.OffsetY}, nil
}

// Close releases a document.
func (a *App) Close(id string) error {
	return a.reader.Close(reader.DocumentID(id))
}

// pngDataURL encodes an image as a base64 PNG data URL for an <img> src.
func pngDataURL(img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("encode page png: %w", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func openKindString(k reader.OpenKind) string {
	switch k {
	case reader.OpenNotFound:
		return "not-found"
	case reader.OpenNotPDF:
		return "not-pdf"
	case reader.OpenCorrupt:
		return "corrupt"
	case reader.OpenPasswordReqd:
		return "password"
	default:
		return "error"
	}
}

// userMessage is the human-facing text for a soft open failure.
func userMessage(oe *reader.OpenError) string {
	switch oe.Kind {
	case reader.OpenNotFound:
		return "That file could not be found."
	case reader.OpenNotPDF:
		return "That file is not a PDF."
	case reader.OpenCorrupt:
		return "This PDF appears to be damaged or incomplete and can't be opened."
	case reader.OpenPasswordReqd:
		return "This PDF is password-protected. Opening protected files isn't supported yet."
	default:
		return "The document could not be opened."
	}
}
