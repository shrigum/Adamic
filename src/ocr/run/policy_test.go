package run_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/shrigum/adamic/src/library"
	"github.com/shrigum/adamic/src/ocr"
	"github.com/shrigum/adamic/src/ocr/run"
	"github.com/shrigum/adamic/src/reader"
)

// recognizedFixture runs full OCR of the scanned fixture once with a
// counting fake recognizer and returns the runner, its parts, and the
// document's identity. The fake's call count starts at scannedPages.
func recognizedFixture(t *testing.T) (*run.Runner, *fakeRecognizer, *recordingStore, *reader.Document, library.DocID) {
	t.Helper()
	e, doc := openFixture(t, scannedPDF)
	rec := &fakeRecognizer{}
	st := &recordingStore{}
	r := run.NewRunner(e, rec, st)
	t.Cleanup(r.Close)

	doneCh := make(chan doneResult, 1)
	if err := r.Start(doc, nil, func(res ocr.Result, err error) { doneCh <- doneResult{res, err} }); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if d := waitDone(t, doneCh); d.err != nil {
		t.Fatalf("initial run error = %v", d.err)
	}
	id, err := library.Identify(doc.Path)
	if err != nil {
		t.Fatalf("Identify: %v", err)
	}
	return r, rec, st, doc, id
}

func waitPage(t *testing.T, ch <-chan pageDone) pageDone {
	t.Helper()
	select {
	case d := <-ch:
		return d
	case <-time.After(60 * time.Second):
		t.Fatal("page re-OCR did not finish in time")
		return pageDone{}
	}
}

type pageDone struct {
	pr  ocr.PageResult
	err error
}

// TestResultReusedOnReopenWithoutRerun is AC4 at the policy level: after
// recognizing and "reopening" (a fresh Open of the same file), the stored
// result is served with zero further recognizer calls.
func TestResultReusedOnReopenWithoutRerun(t *testing.T) {
	r, rec, _, _, _ := recognizedFixture(t)
	if rec.calls != scannedPages {
		t.Fatalf("recognizer ran %d times during the initial run, want %d", rec.calls, scannedPages)
	}

	// Reopen: same file through a fresh engine (the app-restart case) —
	// identity is content-based, so the cached result is found again.
	_, doc2 := openFixture(t, scannedPDF)
	id2, err := library.Identify(doc2.Path)
	if err != nil {
		t.Fatalf("Identify: %v", err)
	}
	result, ok, err := r.Result(id2)
	if err != nil || !ok {
		t.Fatalf("Result after reopen: ok=%v err=%v", ok, err)
	}
	if len(result.Pages) != scannedPages {
		t.Errorf("cached result has %d pages, want %d", len(result.Pages), scannedPages)
	}
	if rec.calls != scannedPages {
		t.Errorf("recognizer ran %d times after reopen+read, want still %d — reads must never re-run OCR (AC4)", rec.calls, scannedPages)
	}

	if _, ok, err := r.Result("no-such-doc"); ok || err != nil {
		t.Errorf("Result(unknown) = ok=%v err=%v, want no result, no error", ok, err)
	}
}

// TestExplicitPageReOCRReplacesOnlyThatPage is AC5: one page is explicitly
// re-recognized and replaced; every other page — and its corrections — stays.
func TestExplicitPageReOCRReplacesOnlyThatPage(t *testing.T) {
	r, rec, st, doc, id := recognizedFixture(t)
	if err := r.Correct(id, 1, 0, "corrected-1"); err != nil {
		t.Fatalf("Correct page 1: %v", err)
	}
	if err := r.Correct(id, 2, 0, "corrected-2"); err != nil {
		t.Fatalf("Correct page 2: %v", err)
	}
	oldPage0 := st.last(t).Pages[0].Units[0].Text

	pageCh := make(chan pageDone, 1)
	if err := r.StartPage(doc, 1, func(pr ocr.PageResult, err error) { pageCh <- pageDone{pr, err} }); err != nil {
		t.Fatalf("StartPage: %v", err)
	}
	d := waitPage(t, pageCh)
	if d.err != nil {
		t.Fatalf("page re-OCR error = %v", d.err)
	}
	if d.pr.Page != 1 || d.pr.Failure != nil || len(d.pr.Units) == 0 {
		t.Fatalf("page re-OCR outcome = %+v, want page 1 recognized", d.pr)
	}
	if rec.calls != scannedPages+1 {
		t.Errorf("recognizer ran %d times, want %d — exactly one page re-recognized", rec.calls, scannedPages+1)
	}

	stored, ok, err := r.Result(id)
	if err != nil || !ok {
		t.Fatalf("Result: ok=%v err=%v", ok, err)
	}
	if got := stored.Pages[1].Units[0].Text; got != fmt.Sprintf("unit-%d", scannedPages) {
		t.Errorf("page 1 units = %q, want the re-run's fresh unit", got)
	}
	if got := stored.Pages[0].Units[0].Text; got != oldPage0 {
		t.Errorf("page 0 changed across a page-1 re-OCR: %q -> %q", oldPage0, got)
	}
	if _, ok := stored.CorrectionFor(1, 0); ok {
		t.Error("page 1 correction survived its re-OCR (spec A5: dropped with the replaced result)")
	}
	if text, ok := stored.CorrectionFor(2, 0); !ok || text != "corrected-2" {
		t.Errorf("page 2 correction = %q, %v; want untouched", text, ok)
	}
}

func TestPageReOCRRefusalsAreLoud(t *testing.T) {
	t.Run("born-digital page has nothing to recognize", func(t *testing.T) {
		e, doc := openFixture(t, bornDigitalPDF)
		st := &recordingStore{}
		r := run.NewRunner(e, &fakeRecognizer{}, st)
		defer r.Close()

		pageCh := make(chan pageDone, 1)
		if err := r.StartPage(doc, 0, func(pr ocr.PageResult, err error) { pageCh <- pageDone{pr, err} }); err != nil {
			t.Fatalf("StartPage: %v", err)
		}
		d := waitPage(t, pageCh)
		if d.err == nil || !strings.Contains(d.err.Error(), "text layer") {
			t.Fatalf("re-OCR of a text page: err = %v, want a nothing-to-recognize refusal", d.err)
		}
		st.mu.Lock()
		defer st.mu.Unlock()
		if len(st.saves) != 0 {
			t.Errorf("refused re-OCR still saved %d times", len(st.saves))
		}
	})

	t.Run("page out of range fails before any goroutine", func(t *testing.T) {
		e, doc := openFixture(t, scannedPDF)
		r := run.NewRunner(e, &fakeRecognizer{}, &recordingStore{})
		defer r.Close()
		err := r.StartPage(doc, doc.PageInfo.Count, func(ocr.PageResult, error) {
			t.Error("onDone fired for a run that never started")
		})
		if err == nil || !strings.Contains(err.Error(), "pages") {
			t.Fatalf("StartPage(out of range) error = %v, want a loud range error", err)
		}
	})

	t.Run("refused while a run is in flight", func(t *testing.T) {
		e, doc := openFixture(t, scannedPDF)
		rec := &gatedRecognizer{gate: make(chan struct{})}
		r := run.NewRunner(e, rec, &recordingStore{})
		defer r.Close()

		doneCh := make(chan doneResult, 1)
		if err := r.Start(doc, nil, func(res ocr.Result, err error) { doneCh <- doneResult{res, err} }); err != nil {
			t.Fatalf("Start: %v", err)
		}
		if err := r.StartPage(doc, 0, nil); !errors.Is(err, run.ErrRunInProgress) {
			t.Errorf("StartPage during a run: err = %v, want ErrRunInProgress", err)
		}
		if err := r.Correct("some-id", 0, 0, "x"); !errors.Is(err, run.ErrRunInProgress) {
			t.Errorf("Correct during a run: err = %v, want ErrRunInProgress", err)
		}
		r.Cancel()
		waitDone(t, doneCh)
	})
}

// TestFailedReOCRPreservesPreviousUnits: an explicit re-OCR that fails must
// report, not destroy, the page's previous good result.
func TestFailedReOCRPreservesPreviousUnits(t *testing.T) {
	r, rec, _, doc, id := recognizedFixture(t)
	rec.errOn = map[int]error{scannedPages: errors.New("engine exploded")}

	pageCh := make(chan pageDone, 1)
	if err := r.StartPage(doc, 1, func(pr ocr.PageResult, err error) { pageCh <- pageDone{pr, err} }); err != nil {
		t.Fatalf("StartPage: %v", err)
	}
	d := waitPage(t, pageCh)
	if d.err != nil {
		t.Fatalf("failed re-OCR run-level error = %v, want the failure in the PageResult instead", d.err)
	}
	if d.pr.Failure == nil || d.pr.Failure.Kind != ocr.FailureEngine {
		t.Fatalf("re-OCR outcome = %+v, want a typed engine failure", d.pr)
	}

	stored, ok, err := r.Result(id)
	if err != nil || !ok {
		t.Fatalf("Result: ok=%v err=%v", ok, err)
	}
	if stored.Pages[1].Failure != nil || len(stored.Pages[1].Units) == 0 {
		t.Errorf("stored page 1 = %+v; a failed re-OCR must not overwrite the previous good result", stored.Pages[1])
	}
}

// TestCorrectionsPersistThroughThePolicy is the AC6 read path end to end:
// correct, read back with precedence, revert, and the loud no-result case.
func TestCorrectionsPersistThroughThePolicy(t *testing.T) {
	r, _, _, _, id := recognizedFixture(t)

	if err := r.Correct(id, 0, 0, "Hoi"); err != nil {
		t.Fatalf("Correct: %v", err)
	}
	stored, ok, err := r.Result(id)
	if err != nil || !ok {
		t.Fatalf("Result: ok=%v err=%v", ok, err)
	}
	units, ok := stored.EffectiveUnits(0)
	if !ok || units[0].Text != "Hoi" {
		t.Errorf("effective text = %q (ok=%v), want the persisted correction applied on read", units[0].Text, ok)
	}
	if got := stored.Pages[0].Units[0].Text; got == "Hoi" {
		t.Error("engine original was overwritten by the correction")
	}

	if err := r.Revert(id, 0, 0); err != nil {
		t.Fatalf("Revert: %v", err)
	}
	stored, _, _ = r.Result(id)
	if _, ok := stored.CorrectionFor(0, 0); ok {
		t.Error("correction still stored after Revert")
	}

	if err := r.Correct("unknown-doc", 0, 0, "x"); err == nil || !strings.Contains(err.Error(), "no OCR result") {
		t.Errorf("Correct(unknown) error = %v, want a no-result error", err)
	}
	if err := r.Correct(id, 0, 99, "x"); err == nil {
		t.Error("Correct(bad unit) succeeded, want the model's loud error surfaced")
	}
}
