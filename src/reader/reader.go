// Package reader defines the command interface — the boundary across which the
// web frontend asks the Go core to open documents, render pages, and remember
// where the reader left off (ADR-0005). The frontend holds no PDF, rendering,
// or persistence logic; it calls these commands and displays what they return.
//
// This is task T1 of the pdf-reader-core feature
// (docs/planning/pdf-reader-core/critical-path.md). Per design-review condition
// C2 it exposes only the commands the acceptance criteria require — open, page
// count, render a page at a scale, thumbnails, get/set reading position — and
// nothing speculative. The interface may still change through T3–T7; it is not
// a frozen public API until the feature closes.
//
// The engine that satisfies Reader lives in package document (the Document
// Engine, ADR-0012); no other package imports the PDF binding. A stub
// implementation (StubReader) lets the frontend track (T6, the Wails shell) be
// built against this contract before the real engine lands.
//
// Error shape (docs/CODING_STANDARDS.md, "Own your failure modes"): every
// command returns a normal Go error. The soft, user-facing open failures the
// spec enumerates — missing file, non-PDF, corrupt/truncated, encrypted — are
// reported as sentinel-wrapped OpenError values so the frontend can show a
// specific message and stay running (spec AC9/AC10). Calling a command in the
// wrong order (rendering a document that was never opened, an out-of-range page)
// is a programmer error surfaced as ErrClosedDocument / ErrPageOutOfRange, not
// a crash.
package reader

import (
	"errors"
	"fmt"
	"image"
)

// Reader is the command interface (glossary: "Command interface"). Every method
// is a command the frontend may invoke over the Wails boundary. Implementations
// are safe for one document open at a time (spec A8); concurrency across
// documents is out of scope for REQ-1.
type Reader interface {
	// Open loads the PDF at path and returns a handle plus the facts the
	// frontend needs to lay out navigation (page count, and the reading
	// position to restore — page 1 for a never-opened document, spec AC7).
	// A soft open failure (missing, non-PDF, corrupt, encrypted) is returned
	// as an *OpenError; the reader stays usable (AC9/AC10).
	Open(path string) (*Document, error)

	// PageCount reports the number of pages in an open document. It fails with
	// ErrClosedDocument if the handle is not open.
	PageCount(doc DocumentID) (int, error)

	// RenderPage rasterizes one page to an image at the requested scale
	// (glossary: faithful fixed layout — the engine's raster, spec A1). page is
	// zero-based; an out-of-range page is ErrPageOutOfRange (a programmer error,
	// not a user error — the frontend clamps navigation, spec AC2/AC5).
	RenderPage(doc DocumentID, page int, scale Scale) (image.Image, error)

	// Thumbnail renders a low-resolution image of one page for the thumbnail
	// panel (spec AC6). It is RenderPage at a small fixed scale, separated so the
	// engine can cache and prioritize thumbnails independently (A6).
	Thumbnail(doc DocumentID, page int) (image.Image, error)

	// SetPosition records the reader's viewport for a document so a later Open
	// can restore it (spec AC7/AC8). A persistence failure is soft: it is
	// returned for logging but must not lose the open document.
	SetPosition(doc DocumentID, pos Position) error

	// GetPosition returns the last saved position for a document, or the
	// zero Position (page 0, top) if none was ever saved.
	GetPosition(doc DocumentID) (Position, error)

	// Close releases an open document's engine resources. Closing an
	// already-closed or unknown handle is a no-op, not an error.
	Close(doc DocumentID) error
}

// DocumentID is an opaque handle to an open document, minted by Open. The
// frontend passes it back on every subsequent command; it never inspects it.
type DocumentID string

// Document is what Open returns to the frontend: the handle plus the facts
// needed to render the initial view without a second round-trip.
type Document struct {
	ID       DocumentID // handle for subsequent commands
	Path     string     // absolute path that was opened
	PageInfo PageInfo   // per-page geometry, for layout before any page is rendered
	Position Position   // reading position to restore (zero value = page 1, top)
}

// PageInfo carries the page count and each page's intrinsic size in points
// (1/72 inch), which the frontend needs to size the scroll canvas and compute
// fit-to-width / fit-to-page before it has any rendered pixels (spec AC4).
type PageInfo struct {
	Count int
	Sizes []PageSize // len == Count; Sizes[i] is page i's unscaled size
}

// PageSize is a page's intrinsic size in points, before any Scale is applied.
type PageSize struct {
	WidthPt  float64
	HeightPt float64
}

// Scale describes how a page should be sized when rendered. Exactly one field
// is honored, checked in this order: FitWidth/FitPage take a viewport; Zoom is
// a direct multiplier (1.0 == 100%). The zero Scale means Zoom 1.0.
//
// Fit modes carry the viewport so the core computes the raster size (fit modes
// recompute on resize, spec AC4/A5); the frontend passes its current viewport
// rather than pre-computing a zoom factor the core would have to trust.
type Scale struct {
	Zoom     float64  // direct multiplier; used when FitWidth and FitPage are false
	FitWidth bool     // size so the page width fills Viewport.WidthPx
	FitPage  bool     // size so the whole page fits within Viewport
	Viewport Viewport // required when FitWidth or FitPage is set
}

// Viewport is the frontend's available drawing area in device pixels.
type Viewport struct {
	WidthPx  int
	HeightPx int
}

// Position is a persisted per-document reading position (glossary: "Reading
// position"). Page is zero-based. OffsetY is the within-page scroll offset as a
// fraction of page height (0.0 == top), which is resolution-independent so a
// restore is correct at any zoom (spec A2). A2 is low-confidence in the spec;
// this shape is the minimum that satisfies AC7 and is revisited if A2 changes.
type Position struct {
	Page    int
	OffsetY float64
}

// Sentinel errors for the command contract. Callers use errors.Is; the frontend
// maps each to a user-facing message.
var (
	// ErrClosedDocument is returned when a command names a DocumentID that is
	// not currently open (a programmer/order error, spec error summary).
	ErrClosedDocument = errors.New("reader: document is not open")

	// ErrPageOutOfRange is returned when a page index is negative or >= the
	// document's page count.
	ErrPageOutOfRange = errors.New("reader: page index out of range")
)

// OpenKind classifies why Open failed, so the frontend can show a specific
// message and decide whether the failure is expected (all of these are).
type OpenKind int

const (
	OpenNotFound     OpenKind = iota // no file at the path
	OpenNotPDF                       // exists but is not a PDF
	OpenCorrupt                      // a PDF, but malformed/truncated
	OpenPasswordReqd                 // encrypted; needs a password (spec AC10, A7)
)

func (k OpenKind) String() string {
	switch k {
	case OpenNotFound:
		return "not found"
	case OpenNotPDF:
		return "not a PDF"
	case OpenCorrupt:
		return "corrupt or truncated"
	case OpenPasswordReqd:
		return "password required"
	default:
		return "unknown"
	}
}

// OpenError is the soft, user-facing failure of Open (spec AC9/AC10). It names
// the path and classifies the cause; the app stays running. Match it with
// errors.As to render a specific message, or errors.Is against a Kind-bearing
// target is not supported — use As and inspect Kind.
type OpenError struct {
	Path string
	Kind OpenKind
	Err  error // underlying engine error, for logs (not shown to the user)
}

func (e *OpenError) Error() string {
	return fmt.Sprintf("cannot open %q: %s", e.Path, e.Kind)
}

func (e *OpenError) Unwrap() error { return e.Err }
