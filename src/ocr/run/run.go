// Package run drives OCR over a whole open document (task T5 of
// docs/planning/ocr/critical-path.md): it detects which pages need OCR
// (ocr.NeedsOCR over document.PageContent), renders each candidate at
// recognition scale, recognizes it through the ocr.Recognizer seam, and
// assembles the document's ocr.Result. T7 wraps this in a cancellable worker;
// the per-page callback exists so that worker can persist and report each
// page as it completes.
//
// Failure modes (docs/CODING_STANDARDS.md, "Own your failure modes"): OCR is
// additive and soft. A page that cannot be inspected, rendered, or recognized
// becomes a PageResult carrying a typed PageFailure — it never stops the
// other pages and never crashes the run (spec AC8). Document reads: a run
// returns a non-nil error only when the run as a whole cannot proceed or
// continue: the document is not/no longer open, its identity cannot be
// established (nothing could be persisted, spec AC12), or ctx was cancelled —
// and on cancellation the partial Result holds every page finished before the
// cut, so already-recognized work is preserved (spec A8). The run mutates
// nothing: it does not persist (T6/T9) and does not re-run detection
// decisions downstream.
package run

import (
	"context"
	"errors"
	"fmt"

	"github.com/shrigum/adamic/src/document"
	"github.com/shrigum/adamic/src/library"
	"github.com/shrigum/adamic/src/ocr"
	"github.com/shrigum/adamic/src/reader"
)

// recognitionDPI is the render resolution recognition runs at (spec A7:
// ~300 DPI equivalent, independent of on-screen zoom) — the resolution the T2
// spike measured and T4's real-engine tests use.
const recognitionDPI = 300

// Document OCRs every candidate page of an open document and returns the
// document's result, keyed by the same identity the reading-position store
// uses (spec A4/AC12). doc must be the open document as e.Open returned it.
// Pages that are not OCR candidates (born-digital text pages, spec A3) do not
// appear in the result; candidate pages appear in ascending order, each with
// its units or its typed failure.
//
// onPage, if non-nil, is called with each candidate page's PageResult as it
// completes (successes and failures alike), before the next page starts — the
// seam T7 uses to persist partial progress. It is called on the calling
// goroutine.
func Document(ctx context.Context, e *document.Engine, rec ocr.Recognizer, doc *reader.Document, onPage func(ocr.PageResult)) (ocr.Result, error) {
	id, err := library.Identify(doc.Path)
	if err != nil {
		return ocr.Result{}, fmt.Errorf("identify document for OCR: %w", err)
	}
	result := ocr.Result{ID: id}

	for page := 0; page < doc.PageInfo.Count; page++ {
		if ctx.Err() != nil {
			return result, fmt.Errorf("OCR run cancelled at page %d: %w", page, ctx.Err())
		}

		pr, candidate, err := recognizePage(ctx, e, rec, doc, page)
		if err != nil {
			return result, err
		}
		if !candidate {
			continue
		}
		result.Pages = append(result.Pages, pr)
		if onPage != nil {
			onPage(pr)
		}
	}
	return result, nil
}

// recognizePage runs detection and recognition for one page. candidate is
// false when the page has a usable native text layer and OCR is skipped
// (spec A3). Soft per-page problems come back as a PageResult with a typed
// Failure; the returned error is non-nil only for run-level conditions (the
// document handle is gone, or ctx was cancelled mid-recognition).
func recognizePage(ctx context.Context, e *document.Engine, rec ocr.Recognizer, doc *reader.Document, page int) (pr ocr.PageResult, candidate bool, err error) {
	pr = ocr.PageResult{Page: page}

	content, err := e.PageContent(doc.ID, page)
	if err != nil {
		if errors.Is(err, reader.ErrClosedDocument) {
			return pr, false, fmt.Errorf("OCR run: document closed at page %d: %w", page, err)
		}
		// A page we cannot inspect cannot be cleared as born-digital either;
		// reporting it as a failed candidate keeps the problem visible (AC8)
		// rather than silently skipping the page.
		pr.Failure = &ocr.PageFailure{
			Kind:    ocr.FailureUnreadable,
			Message: fmt.Sprintf("this page could not be inspected for text recognition (%v); if the document is intact, re-run OCR on this page", err),
		}
		return pr, true, nil
	}
	if !ocr.NeedsOCR(content.Text, content.ImageCoverage) {
		return pr, false, nil
	}

	img, err := e.RenderPage(doc.ID, page, reader.Scale{Zoom: recognitionDPI / 72.0})
	if err != nil {
		if errors.Is(err, reader.ErrClosedDocument) {
			return pr, false, fmt.Errorf("OCR run: document closed at page %d: %w", page, err)
		}
		pr.Failure = &ocr.PageFailure{
			Kind:    ocr.FailureUnreadable,
			Message: fmt.Sprintf("this page's image could not be rendered for text recognition (%v); if the document is intact, re-run OCR on this page", err),
		}
		return pr, true, nil
	}

	units, err := rec.RecognizePage(ctx, img, doc.PageInfo.Sizes[page])
	if err != nil {
		if ctx.Err() != nil {
			return pr, false, fmt.Errorf("OCR run cancelled at page %d: %w", page, ctx.Err())
		}
		pr.Failure = &ocr.PageFailure{
			Kind:    ocr.FailureEngine,
			Message: fmt.Sprintf("text recognition failed on this page (%v); check the OCR engine installation and re-run OCR on this page", err),
		}
		return pr, true, nil
	}
	pr.Units = units
	return pr, true, nil
}
