// Package ocr defines the OCR result contract: the types a recognizer
// produces, the store persists, and REQ-2 (text extraction/mapping) will
// consume (task T1 of docs/planning/ocr/critical-path.md), plus the Recognizer
// seam an engine implements (task T4, recognizer.go). Pure types, interfaces,
// and their invariants — no engine binding, no persistence, no I/O; the one
// shipped engine lives in the tesseract subpackage.
//
// Coordinate space (spec A2, AC2, AC12; design-review condition C4): every box
// is in page points (1/72 inch), the same space as reader.PageSize — origin at
// the page's top-left corner, x increasing right, y increasing down, matching
// the orientation of the raster the Document Engine renders and the frontend
// displays. A box is therefore valid at any zoom: multiply by the render scale.
// The pixel→point transform from a particular render lives with the recognizer,
// never in these types.
//
// Document identity (spec A4, AC12): a document's OCR result is keyed by
// library.DocID (absolute path + content hash, library.Identify) — the same key
// the reading-position store uses, so reopening the same file finds its OCR.
//
// Failure modes (docs/CODING_STANDARDS.md, "Own your failure modes"): OCR is
// additive and soft (spec error summary). This package defines how failure is
// *represented*, not produced: a page that could not be recognized carries a
// typed PageFailure in its PageResult (AC8) while other pages carry their
// units; the document itself stays fully readable regardless. Validate reports
// contract violations (empty text, confidence out of range, box outside the
// page) as loud errors, because a recognizer emitting them is a bug, not a bad
// scan.
package ocr

import (
	"fmt"

	"github.com/shrigum/adamic/src/library"
	"github.com/shrigum/adamic/src/reader"
)

// RecognizedUnit is one piece of recognized text tied to the on-page rectangle
// it came from (glossary-to-be: "recognized unit", spec A2). Granularity (word
// vs. line) follows what the engine provides; REQ-2 may re-segment. The unit
// carries no reading order — ordering across units is REQ-2's job, not OCR's.
type RecognizedUnit struct {
	// Text is the recognized text, never empty for a valid unit. Corrections
	// are not stored here: a user override lives alongside the engine result
	// (spec A6, task T8) and takes precedence on read, so the engine's original
	// text is retained.
	Text string `json:"text"`

	// Box is where Text sits on the page, in page points (see package doc for
	// origin and orientation).
	Box Box `json:"box"`

	// Confidence is the engine's confidence in Text, normalized to [0, 1]
	// (0 = none, 1 = certain). Engines reporting other scales (Tesseract:
	// 0–100) are normalized by the recognizer before units leave it.
	Confidence float64 `json:"confidence"`

	// Group is the engine's opaque line/block grouping id for this unit, or ""
	// if the engine provides none (spec A2). It is a hint REQ-2 may use when
	// segmenting; this feature never interprets it.
	Group string `json:"group,omitempty"`
}

// Box is an axis-aligned rectangle in page points: (X, Y) is the top-left
// corner, W and H extend right and down. Valid boxes have positive W and H.
type Box struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// In reports whether the box lies entirely within a page of the given size
// (boundary contact allowed).
func (b Box) In(size reader.PageSize) bool {
	return b.X >= 0 && b.Y >= 0 &&
		b.X+b.W <= size.WidthPt && b.Y+b.H <= size.HeightPt
}

// Validate checks the unit against the contract's invariants (spec AC2) for a
// page of the given size: non-empty text, confidence in [0, 1], and a
// positive-sized box inside the page bounds. A violation is a programmer error
// in whatever produced the unit — recognizers must validate before emitting,
// and tests assert it on real fixtures from the first recognizer commit.
func (u RecognizedUnit) Validate(size reader.PageSize) error {
	if u.Text == "" {
		return fmt.Errorf("recognized unit at %+v has empty text", u.Box)
	}
	if u.Confidence < 0 || u.Confidence > 1 {
		return fmt.Errorf("recognized unit %q has confidence %v outside [0, 1]", u.Text, u.Confidence)
	}
	if u.Box.W <= 0 || u.Box.H <= 0 {
		return fmt.Errorf("recognized unit %q has non-positive box size %vx%v", u.Text, u.Box.W, u.Box.H)
	}
	if !u.Box.In(size) {
		return fmt.Errorf("recognized unit %q box %+v lies outside the %vx%v pt page", u.Text, u.Box, size.WidthPt, size.HeightPt)
	}
	return nil
}

// PageResult is the outcome of recognizing one page: its units on success, or
// a typed failure (spec AC8 — a bad page is reported, never a crash, and never
// stops other pages). Exactly one of Units and Failure is meaningful: a failed
// page has a nil Units slice, a recognized page has a nil Failure. A page with
// no recognizable text (blank page) is a success with zero units, not a
// failure.
type PageResult struct {
	// Page is the zero-based page index, matching the reader's page indexing.
	Page int `json:"page"`

	// Units is the recognized text of the page. Order carries no meaning
	// (reading order is REQ-2).
	Units []RecognizedUnit `json:"units,omitempty"`

	// Failure is set when recognition of this page failed (nil on success).
	Failure *PageFailure `json:"failure,omitempty"`
}

// PageFailure is the typed, per-page, user-facing recognition failure (spec
// AC8; task T13 normalizes raw engine errors into this shape). It is data, not
// a Go error: it persists with the result so a failed page is still reported
// after reopening, and can be retried via explicit re-OCR (spec A5).
type PageFailure struct {
	Kind FailureKind `json:"kind"`

	// Message says what happened and, where an action exists, what to do —
	// it is shown to the user (docs/CODING_STANDARDS.md, error handling).
	Message string `json:"message"`
}

// FailureKind classifies why a page could not be recognized, normalized from
// the engine's failure modes (design-review implementer notes: engine or model
// missing, page unreadable, timeout).
type FailureKind string

const (
	// FailureEngine: the OCR engine or its model was missing, failed to start,
	// or crashed. Affects every page it is reported on; nothing page-specific.
	FailureEngine FailureKind = "engine"

	// FailureUnreadable: this page's image could not be rendered or decoded
	// for recognition (corrupt or unsupported image data).
	FailureUnreadable FailureKind = "unreadable"

	// FailureTimeout: recognition exceeded the per-page time budget (spec A8;
	// budget constant set by task T15).
	FailureTimeout FailureKind = "timeout"
)

// Result is a document's OCR result: the pages recognition ran on, keyed to
// the document by the same identity the reading-position store uses (spec A4,
// AC12). Pages not in Pages were never OCR candidates or have not been run;
// only candidate pages appear (detection is per page, spec A3), in ascending
// Page order with no duplicates. This shape is what the store (T6) persists
// inside its versioned envelope and what REQ-2 consumes.
type Result struct {
	ID    library.DocID `json:"id"`
	Pages []PageResult  `json:"pages"`

	// Corrections are the user's text overrides, addressed into Pages and
	// applied on read (EffectiveUnits); the engine originals in Pages are
	// never rewritten (task T8, spec A6). Kept sorted by (Page, Unit) with no
	// duplicates. Part of the same persisted envelope — corrections survive
	// exactly as long as the engine result they annotate.
	Corrections []Correction `json:"corrections,omitempty"`
}
