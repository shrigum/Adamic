package ocr

import (
	"context"
	"image"

	"github.com/shrigum/adamic/src/reader"
)

// Recognizer is the seam between the OCR feature and its engine (task T4;
// design-review condition C2): one rendered page image in, recognized units
// out — nothing else. No engine selection, capabilities, or configuration
// crosses this boundary. The MVP ships exactly one implementation (Tesseract,
// ADR-0014, src/ocr/tesseract); the ADR names the optional VLM backend as the
// foreseen second implementation behind this same seam.
type Recognizer interface {
	// RecognizePage recognizes the text of one page from its rendered image.
	// img is the page rasterized by the Document Engine at a
	// recognition-appropriate scale (spec A7); size is that page's size in
	// points, exactly as the reader reports it (condition C4). The returned
	// units are in page-point coordinates and each satisfies Validate for
	// size. ctx aborts an in-flight recognition (T7 cancels per page this
	// way); a cancelled call returns an error wrapping ctx.Err().
	//
	// Errors are soft, engine-level failures (engine unusable, image
	// unprocessable, cancelled) for the caller to normalize into the typed
	// per-page PageFailure (AC8, T13); RecognizePage never panics on a bad
	// image. A blank page is not an error: it returns zero units and nil.
	RecognizePage(ctx context.Context, img image.Image, size reader.PageSize) ([]RecognizedUnit, error)
}
