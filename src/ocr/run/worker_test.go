package run_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shrigum/adamic/src/library"
	"github.com/shrigum/adamic/src/ocr"
	"github.com/shrigum/adamic/src/ocr/run"
	"github.com/shrigum/adamic/src/ocr/store"
	"github.com/shrigum/adamic/src/ocr/tesseract"
	"github.com/shrigum/adamic/src/reader"
)

// gatedRecognizer blocks each recognition until the test sends on gate (or
// the run's ctx is cancelled), so tests control exactly how far a run gets.
type gatedRecognizer struct {
	fakeRecognizer
	gate chan struct{}
}

func (g *gatedRecognizer) RecognizePage(ctx context.Context, img image.Image, size reader.PageSize) ([]ocr.RecognizedUnit, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("recognize page: cancelled: %w", ctx.Err())
	case <-g.gate:
	}
	return g.fakeRecognizer.RecognizePage(ctx, img, size)
}

// recordingStore is an in-memory ocr.Store that snapshots every Save (deep
// copies, so later upserts can't rewrite history) and can be seeded or made
// to fail.
type recordingStore struct {
	mu      sync.Mutex
	saves   []ocr.Result
	seed    *ocr.Result
	saveErr error
}

// Load behaves like the real store: the latest save for id wins, then the
// seeded pre-existing result, then "no OCR yet".
func (s *recordingStore) Load(id library.DocID) (ocr.Result, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.saves) - 1; i >= 0; i-- {
		if s.saves[i].ID == id {
			return snapshot(s.saves[i]), true, nil
		}
	}
	if s.seed != nil && s.seed.ID == id {
		return snapshot(*s.seed), true, nil
	}
	return ocr.Result{}, false, nil
}

func (s *recordingStore) Save(result ocr.Result) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveErr != nil {
		return s.saveErr
	}
	s.saves = append(s.saves, snapshot(result))
	return nil
}

func (s *recordingStore) last(t *testing.T) ocr.Result {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.saves) == 0 {
		t.Fatal("nothing was saved")
	}
	return s.saves[len(s.saves)-1]
}

func snapshot(r ocr.Result) ocr.Result {
	data, err := json.Marshal(r)
	if err != nil {
		panic(err)
	}
	var out ocr.Result
	if err := json.Unmarshal(data, &out); err != nil {
		panic(err)
	}
	return out
}

// doneResult waits for onDone's report, or fails the test after a timeout.
type doneResult struct {
	result ocr.Result
	err    error
}

func waitDone(t *testing.T, ch <-chan doneResult) doneResult {
	t.Helper()
	select {
	case d := <-ch:
		return d
	case <-time.After(60 * time.Second):
		t.Fatal("run did not finish in time")
		return doneResult{}
	}
}

func TestRunnerCompletesOffThreadAndPersistsIncrementally(t *testing.T) {
	e, doc := openFixture(t, scannedPDF)
	rec := &gatedRecognizer{gate: make(chan struct{}, scannedPages)}
	st := &recordingStore{}
	r := run.NewRunner(e, rec, st)
	defer r.Close()

	doneCh := make(chan doneResult, 1)
	if err := r.Start(doc, nil, func(res ocr.Result, err error) { doneCh <- doneResult{res, err} }); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Start returned while every page is still gated: the run is off-thread.
	select {
	case d := <-doneCh:
		t.Fatalf("run finished before any page was released: %+v", d)
	default:
	}
	for i := 0; i < scannedPages; i++ {
		rec.gate <- struct{}{}
	}

	d := waitDone(t, doneCh)
	if d.err != nil {
		t.Fatalf("run error = %v", d.err)
	}
	if len(d.result.Pages) != scannedPages {
		t.Fatalf("result has %d pages, want %d", len(d.result.Pages), scannedPages)
	}
	st.mu.Lock()
	saveCount := len(st.saves)
	st.mu.Unlock()
	if saveCount != scannedPages {
		t.Errorf("store saved %d times, want once per page (%d) — per-page persistence (AC7)", saveCount, scannedPages)
	}
	if got := st.last(t); len(got.Pages) != scannedPages || got.ID != d.result.ID {
		t.Errorf("final stored result = %d pages id %q, want %d pages id %q", len(got.Pages), got.ID, scannedPages, d.result.ID)
	}
}

func TestRunnerCancelPersistsFinishedPagesAndKeepsOldOnes(t *testing.T) {
	e, doc := openFixture(t, scannedPDF)
	id, err := library.Identify(doc.Path)
	if err != nil {
		t.Fatalf("Identify: %v", err)
	}

	// An earlier full run is on record; this run replaces page 0, is
	// cancelled, and must leave pages 1–3 of the old result untouched.
	old := ocr.Result{ID: id}
	for p := 0; p < scannedPages; p++ {
		old.Pages = append(old.Pages, ocr.PageResult{
			Page:  p,
			Units: []ocr.RecognizedUnit{{Text: fmt.Sprintf("old-%d", p), Box: ocr.Box{X: 1, Y: 1, W: 2, H: 2}, Confidence: 0.4}},
		})
	}
	// User corrections on the old result: page 0's must die with its re-OCR,
	// page 2's must survive untouched (T8 model over the T7 merge).
	old.Corrections = []ocr.Correction{{Page: 0, Unit: 0, Text: "corrected-0"}, {Page: 2, Unit: 0, Text: "corrected-2"}}
	rec := &gatedRecognizer{gate: make(chan struct{})}
	st := &recordingStore{seed: &old}
	r := run.NewRunner(e, rec, st)
	defer r.Close()

	progressCh := make(chan run.Progress, scannedPages)
	doneCh := make(chan doneResult, 1)
	if err := r.Start(doc, func(p run.Progress) { progressCh <- p }, func(res ocr.Result, err error) { doneCh <- doneResult{res, err} }); err != nil {
		t.Fatalf("Start: %v", err)
	}
	rec.gate <- struct{}{} // release page 0 only
	select {
	case p := <-progressCh:
		if p.Page != 0 || p.PageCount != scannedPages || p.Failed {
			t.Errorf("first progress = %+v, want page 0 of %d, not failed", p, scannedPages)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("no progress event for page 0")
	}
	r.Cancel()

	d := waitDone(t, doneCh)
	if !errors.Is(d.err, context.Canceled) {
		t.Fatalf("cancelled run error = %v, want wrapping context.Canceled", d.err)
	}
	if len(d.result.Pages) != 1 || d.result.Pages[0].Page != 0 {
		t.Fatalf("partial result = %+v, want exactly the finished page 0", d.result.Pages)
	}

	stored := st.last(t)
	if len(stored.Pages) != scannedPages {
		t.Fatalf("stored result has %d pages after cancel, want %d (new page 0 + preserved old pages)", len(stored.Pages), scannedPages)
	}
	if got := stored.Pages[0].Units[0].Text; got == "old-0" {
		t.Error("page 0 still holds the old run's unit; the new result should have replaced it")
	}
	for p := 1; p < scannedPages; p++ {
		if got := stored.Pages[p].Units[0].Text; got != fmt.Sprintf("old-%d", p) {
			t.Errorf("page %d = %q, want the earlier run's result preserved (never clobbered)", p, got)
		}
	}
	if _, ok := stored.CorrectionFor(0, 0); ok {
		t.Error("page 0 correction survived that page's re-OCR")
	}
	if text, ok := stored.CorrectionFor(2, 0); !ok || text != "corrected-2" {
		t.Errorf("page 2 correction = %q, %v; want preserved across the partial run (AC6)", text, ok)
	}
}

func TestRunnerOneRunAtATime(t *testing.T) {
	e, doc := openFixture(t, scannedPDF)
	rec := &gatedRecognizer{gate: make(chan struct{}, scannedPages)}
	st := &recordingStore{}
	r := run.NewRunner(e, rec, st)
	defer r.Close()

	doneCh := make(chan doneResult, 1)
	if err := r.Start(doc, nil, func(res ocr.Result, err error) { doneCh <- doneResult{res, err} }); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := r.Start(doc, nil, nil); !errors.Is(err, run.ErrRunInProgress) {
		t.Fatalf("second Start error = %v, want ErrRunInProgress", err)
	}

	r.Cancel()
	waitDone(t, doneCh)

	// After the first run ended, a new one may start.
	done2 := make(chan doneResult, 1)
	if err := r.Start(doc, nil, func(res ocr.Result, err error) { done2 <- doneResult{res, err} }); err != nil {
		t.Fatalf("Start after finished run: %v", err)
	}
	for i := 0; i < scannedPages; i++ {
		rec.gate <- struct{}{}
	}
	if d := waitDone(t, done2); d.err != nil {
		t.Errorf("second run error = %v", d.err)
	}
}

func TestRunnerReportsFailedPagesInProgress(t *testing.T) {
	e, doc := openFixture(t, scannedPDF)
	rec := &fakeRecognizer{errOn: map[int]error{1: errors.New("engine exploded")}}
	st := &recordingStore{}
	r := run.NewRunner(e, rec, st)
	defer r.Close()

	var events []run.Progress
	doneCh := make(chan doneResult, 1)
	err := r.Start(doc,
		func(p run.Progress) { events = append(events, p) },
		func(res ocr.Result, err error) { doneCh <- doneResult{res, err} })
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	d := waitDone(t, doneCh)
	if d.err != nil {
		t.Fatalf("run error = %v (a page failure must not fail the run, AC8)", d.err)
	}
	if len(events) != scannedPages {
		t.Fatalf("got %d progress events, want %d", len(events), scannedPages)
	}
	for _, p := range events {
		if wantFailed := p.Page == 1; p.Failed != wantFailed {
			t.Errorf("progress for page %d: Failed = %v, want %v", p.Page, p.Failed, wantFailed)
		}
	}
}

func TestRunnerStoreFailureIsSoft(t *testing.T) {
	e, doc := openFixture(t, scannedPDF)
	rec := &fakeRecognizer{}
	st := &recordingStore{saveErr: errors.New("disk full")}
	r := run.NewRunner(e, rec, st)
	defer r.Close()

	doneCh := make(chan doneResult, 1)
	if err := r.Start(doc, nil, func(res ocr.Result, err error) { doneCh <- doneResult{res, err} }); err != nil {
		t.Fatalf("Start: %v", err)
	}
	d := waitDone(t, doneCh)
	if len(d.result.Pages) != scannedPages {
		t.Fatalf("result has %d pages, want %d — recognition must survive a failing store", len(d.result.Pages), scannedPages)
	}
	if d.err == nil || !strings.Contains(d.err.Error(), "persist") {
		t.Fatalf("run error = %v, want the persistence failure reported", d.err)
	}
	if errors.Is(d.err, context.Canceled) {
		t.Error("store failure reported as cancellation")
	}
}

func TestRunnerCloseCancelsAndWaits(t *testing.T) {
	e, doc := openFixture(t, scannedPDF)
	rec := &gatedRecognizer{gate: make(chan struct{})}
	st := &recordingStore{}
	r := run.NewRunner(e, rec, st)

	doneCh := make(chan doneResult, 1)
	if err := r.Start(doc, nil, func(res ocr.Result, err error) { doneCh <- doneResult{res, err} }); err != nil {
		t.Fatalf("Start: %v", err)
	}
	r.Close()
	// Close returned: the goroutine (including its onDone) must be finished.
	select {
	case d := <-doneCh:
		if !errors.Is(d.err, context.Canceled) {
			t.Errorf("run error after Close = %v, want wrapping context.Canceled", d.err)
		}
	default:
		t.Fatal("Close returned before the run goroutine finished")
	}
}

func TestRunnerStartRefusesUnidentifiableDocument(t *testing.T) {
	e, _ := openFixture(t, scannedPDF)
	r := run.NewRunner(e, &fakeRecognizer{}, &recordingStore{})
	defer r.Close()

	err := r.Start(&reader.Document{Path: "does-not-exist.pdf"}, nil, func(ocr.Result, error) {
		t.Error("onDone fired for a run that never started")
	})
	if err == nil {
		t.Fatal("Start succeeded on an unidentifiable document, want an error (nothing could be persisted)")
	}
}

// TestRunnerRealEngine drives the whole T7 stack — worker, real Tesseract,
// real file store — over the scanned fixture. Skips without a local engine.
func TestRunnerRealEngine(t *testing.T) {
	rec, err := tesseract.Find("nld")
	if err != nil {
		t.Skipf("no usable Tesseract engine, skipping real-engine test: %v", err)
	}
	e, doc := openFixture(t, scannedPDF) // sets EnvConfigDir to a temp dir
	r := run.NewRunner(e, rec, store.FileStore{})
	defer r.Close()

	var events []run.Progress
	doneCh := make(chan doneResult, 1)
	err = r.Start(doc,
		func(p run.Progress) { events = append(events, p) },
		func(res ocr.Result, err error) { doneCh <- doneResult{res, err} })
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	d := waitDone(t, doneCh)
	if d.err != nil {
		t.Fatalf("run error = %v", d.err)
	}
	if len(events) != scannedPages {
		t.Errorf("got %d progress events, want %d", len(events), scannedPages)
	}
	stored, ok, err := store.FileStore{}.Load(d.result.ID)
	if err != nil || !ok {
		t.Fatalf("reload persisted result: ok=%v err=%v", ok, err)
	}
	if len(stored.Pages) != scannedPages {
		t.Fatalf("persisted result has %d pages, want %d", len(stored.Pages), scannedPages)
	}
}
