package run_test

import (
	"context"
	"errors"
	"fmt"
	"image"
	"testing"

	"github.com/shrigum/adamic/src/document"
	"github.com/shrigum/adamic/src/library"
	"github.com/shrigum/adamic/src/ocr"
	"github.com/shrigum/adamic/src/ocr/run"
	"github.com/shrigum/adamic/src/ocr/tesseract"
	"github.com/shrigum/adamic/src/reader"
)

const (
	scannedPDF     = "../../document/testdata/taalcompleet-a1-sample.pdf"
	bornDigitalPDF = "../../document/testdata/born-digital.pdf"
	scannedPages   = 4
)

// fakeRecognizer is a canned ocr.Recognizer: call n yields one valid unit
// "unit-n", or errOn[n]'s error. Like the real engine, it reports ctx
// cancellation as an error wrapping ctx.Err().
type fakeRecognizer struct {
	calls int
	errOn map[int]error // keyed by 0-based call ordinal
}

func (f *fakeRecognizer) RecognizePage(ctx context.Context, img image.Image, size reader.PageSize) ([]ocr.RecognizedUnit, error) {
	call := f.calls
	f.calls++
	if ctx.Err() != nil {
		return nil, fmt.Errorf("recognize page: cancelled: %w", ctx.Err())
	}
	if err := f.errOn[call]; err != nil {
		return nil, err
	}
	return []ocr.RecognizedUnit{{
		Text:       fmt.Sprintf("unit-%d", call),
		Box:        ocr.Box{X: 1, Y: 1, W: size.WidthPt / 2, H: size.HeightPt / 4},
		Confidence: 0.9,
	}}, nil
}

// openFixture starts a real engine and opens one fixture; both are cleaned up
// with the test.
func openFixture(t *testing.T, path string) (*document.Engine, *reader.Document) {
	t.Helper()
	t.Setenv(library.EnvConfigDir, t.TempDir())
	e, err := document.NewEngine()
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	t.Cleanup(func() {
		if err := e.Shutdown(); err != nil {
			t.Errorf("Shutdown: %v", err)
		}
	})
	doc, err := e.Open(path)
	if err != nil {
		t.Fatalf("Open %s: %v", path, err)
	}
	t.Cleanup(func() { e.Close(doc.ID) })
	return e, doc
}

func TestDocumentRecognizesAllCandidatePages(t *testing.T) {
	e, doc := openFixture(t, scannedPDF)
	rec := &fakeRecognizer{}

	var seen []int
	result, err := run.Document(context.Background(), e, rec, doc, func(pr ocr.PageResult) {
		seen = append(seen, pr.Page)
	})
	if err != nil {
		t.Fatalf("Document: %v", err)
	}

	wantID, err := library.Identify(doc.Path)
	if err != nil {
		t.Fatalf("Identify: %v", err)
	}
	if result.ID != wantID {
		t.Errorf("result.ID = %q, want the library identity %q (AC12)", result.ID, wantID)
	}
	if len(result.Pages) != scannedPages {
		t.Fatalf("got %d page results, want %d (every scanned page is a candidate)", len(result.Pages), scannedPages)
	}
	for i, pr := range result.Pages {
		if pr.Page != i {
			t.Errorf("Pages[%d].Page = %d, want ascending page order", i, pr.Page)
		}
		if pr.Failure != nil {
			t.Errorf("page %d unexpectedly failed: %+v", pr.Page, pr.Failure)
		}
		if len(pr.Units) == 0 {
			t.Errorf("page %d has no units", pr.Page)
		}
	}
	if len(seen) != scannedPages {
		t.Errorf("onPage called %d times, want once per candidate page (%d)", len(seen), scannedPages)
	}
}

func TestDocumentSkipsBornDigitalPages(t *testing.T) {
	e, doc := openFixture(t, bornDigitalPDF)
	rec := &fakeRecognizer{}

	result, err := run.Document(context.Background(), e, rec, doc, nil)
	if err != nil {
		t.Fatalf("Document: %v", err)
	}
	if len(result.Pages) != 0 {
		t.Errorf("got %d page results on a born-digital document, want 0 (spec A3)", len(result.Pages))
	}
	if rec.calls != 0 {
		t.Errorf("recognizer called %d times on a born-digital document, want 0 (no OCR run)", rec.calls)
	}
}

func TestDocumentReportsPerPageFailureAndContinues(t *testing.T) {
	e, doc := openFixture(t, scannedPDF)
	rec := &fakeRecognizer{errOn: map[int]error{1: errors.New("engine exploded")}}

	result, err := run.Document(context.Background(), e, rec, doc, nil)
	if err != nil {
		t.Fatalf("Document: %v (a page failure must not fail the run, AC8)", err)
	}
	if len(result.Pages) != scannedPages {
		t.Fatalf("got %d page results, want %d (failed page still reported)", len(result.Pages), scannedPages)
	}
	for _, pr := range result.Pages {
		if pr.Page == 1 {
			if pr.Failure == nil {
				t.Fatal("page 1 has no Failure, want the typed per-page failure (AC8)")
			}
			if pr.Failure.Kind != ocr.FailureEngine {
				t.Errorf("page 1 failure kind = %q, want %q", pr.Failure.Kind, ocr.FailureEngine)
			}
			if pr.Failure.Message == "" {
				t.Error("page 1 failure has an empty user-facing message")
			}
			if pr.Units != nil {
				t.Errorf("page 1 has both units and a failure: %+v", pr.Units)
			}
			continue
		}
		if pr.Failure != nil {
			t.Errorf("page %d failed too: %+v (only page 1 should)", pr.Page, pr.Failure)
		}
	}
}

func TestDocumentCancelPreservesFinishedPages(t *testing.T) {
	e, doc := openFixture(t, scannedPDF)
	rec := &fakeRecognizer{}

	ctx, cancel := context.WithCancel(context.Background())
	result, err := run.Document(ctx, e, rec, doc, func(ocr.PageResult) { cancel() })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Document error = %v, want wrapping context.Canceled", err)
	}
	if len(result.Pages) != 1 {
		t.Fatalf("partial result has %d pages, want the 1 finished before cancel (spec A8)", len(result.Pages))
	}
	if result.Pages[0].Page != 0 || result.Pages[0].Failure != nil {
		t.Errorf("preserved page = %+v, want page 0 recognized", result.Pages[0])
	}
}

func TestDocumentClosedMidRunStopsWithError(t *testing.T) {
	e, doc := openFixture(t, scannedPDF)
	rec := &fakeRecognizer{}

	result, err := run.Document(context.Background(), e, rec, doc, func(ocr.PageResult) { e.Close(doc.ID) })
	if !errors.Is(err, reader.ErrClosedDocument) {
		t.Fatalf("Document error = %v, want wrapping ErrClosedDocument", err)
	}
	if len(result.Pages) != 1 {
		t.Errorf("partial result has %d pages, want the 1 finished before the close", len(result.Pages))
	}
}

// TestDocumentRealEngineDutchFixture is the document-level run end to end with
// the real Tesseract engine: every page of the scanned fixture is recognized
// to contract-valid units (AC1/AC2 at document scope). Skips without a local
// engine, like the T2/T4 real-engine tests.
func TestDocumentRealEngineDutchFixture(t *testing.T) {
	rec, err := tesseract.Find("nld")
	if err != nil {
		t.Skipf("no usable Tesseract engine, skipping real-engine test: %v", err)
	}
	e, doc := openFixture(t, scannedPDF)

	result, err := run.Document(context.Background(), e, rec, doc, nil)
	if err != nil {
		t.Fatalf("Document: %v", err)
	}
	if len(result.Pages) != scannedPages {
		t.Fatalf("got %d page results, want %d", len(result.Pages), scannedPages)
	}
	for _, pr := range result.Pages {
		if pr.Failure != nil {
			t.Errorf("page %d failed: %+v", pr.Page, pr.Failure)
			continue
		}
		if len(pr.Units) == 0 {
			t.Errorf("page %d recognized no units on a page full of text", pr.Page)
		}
		for _, u := range pr.Units {
			if err := u.Validate(doc.PageInfo.Sizes[pr.Page]); err != nil {
				t.Errorf("page %d contract violation: %v", pr.Page, err)
			}
		}
	}
}
