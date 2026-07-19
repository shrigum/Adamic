package app

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shrigum/adamic/src/document"
	"github.com/shrigum/adamic/src/library"
	"github.com/shrigum/adamic/src/ocr"
	"github.com/shrigum/adamic/src/ocr/run"
	"github.com/shrigum/adamic/src/reader"
)

const (
	scannedPDF     = "../document/testdata/taalcompleet-a1-sample.pdf"
	bornDigitalPDF = "../document/testdata/born-digital.pdf"
	scannedPages   = 4
)

// stubRecognizer returns one valid unit per call, without any engine.
type stubRecognizer struct{ calls int }

func (s *stubRecognizer) RecognizePage(ctx context.Context, img image.Image, size reader.PageSize) ([]ocr.RecognizedUnit, error) {
	s.calls++
	return []ocr.RecognizedUnit{{
		Text:       fmt.Sprintf("unit-%d", s.calls),
		Box:        ocr.Box{X: 1, Y: 1, W: size.WidthPt / 2, H: size.HeightPt / 4},
		Confidence: 0.9,
	}}, nil
}

// memStore is a minimal in-memory ocr.Store.
type memStore struct {
	mu sync.Mutex
	m  map[library.DocID]string // JSON snapshots: honest copies
}

func (s *memStore) Load(id library.DocID) (ocr.Result, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.m[id]
	if !ok {
		return ocr.Result{}, false, nil
	}
	var out ocr.Result
	if err := json.Unmarshal([]byte(data), &out); err != nil {
		return ocr.Result{}, false, err
	}
	return out, true, nil
}

func (s *memStore) Save(result ocr.Result) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[library.DocID]string{}
	}
	s.m[result.ID] = string(data)
	return nil
}

// eventCollector captures emitted events and lets tests wait for one.
type eventCollector struct {
	mu     sync.Mutex
	events map[string][]any
	waitCh chan string
}

func newEventCollector() *eventCollector {
	return &eventCollector{events: map[string][]any{}, waitCh: make(chan string, 64)}
}

func (c *eventCollector) emit(name string, data any) {
	c.mu.Lock()
	c.events[name] = append(c.events[name], data)
	c.mu.Unlock()
	c.waitCh <- name
}

func (c *eventCollector) waitFor(t *testing.T, name string) any {
	t.Helper()
	deadline := time.After(60 * time.Second)
	for {
		select {
		case got := <-c.waitCh:
			if got == name {
				c.mu.Lock()
				defer c.mu.Unlock()
				return c.events[name][len(c.events[name])-1]
			}
		case <-deadline:
			t.Fatalf("event %q never emitted", name)
			return nil
		}
	}
}

func (c *eventCollector) count(name string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.events[name])
}

// newOCRApp builds an App over a real engine with OCR enabled on a stub
// recognizer and in-memory store, and opens the given fixture.
func newOCRApp(t *testing.T, fixture string) (*App, *stubRecognizer, *eventCollector, string) {
	t.Helper()
	t.Setenv(library.EnvConfigDir, t.TempDir())
	engine, err := document.NewEngine()
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	t.Cleanup(func() {
		if err := engine.Shutdown(); err != nil {
			t.Errorf("Shutdown: %v", err)
		}
	})

	a := New(engine)
	rec := &stubRecognizer{}
	runner := run.NewRunner(engine, rec, &memStore{})
	t.Cleanup(runner.Close)
	events := newEventCollector()
	a.EnableOCR(engine, runner, events.emit)

	res := a.Open(fixture)
	if !res.Ok {
		t.Fatalf("Open %s: %+v", fixture, res.Error)
	}
	return a, rec, events, res.Doc.ID
}

func TestOCRUnavailableIsSoft(t *testing.T) {
	a := New(reader.NewStubReader(3))
	res := a.Open("/x.pdf")

	if _, err := a.OCRCandidates(res.Doc.ID); err == nil || !strings.Contains(err.Error(), "not available") {
		t.Errorf("OCRCandidates without OCR: err = %v, want the soft unavailable error", err)
	}
	if err := a.OCRStart(res.Doc.ID); err == nil || !strings.Contains(err.Error(), "not available") {
		t.Errorf("OCRStart without OCR: err = %v, want the soft unavailable error", err)
	}
	if _, err := a.OCRResult(res.Doc.ID); err == nil {
		t.Error("OCRResult without OCR succeeded, want the soft unavailable error")
	}
	if err := a.OCRCancel(); err == nil {
		t.Error("OCRCancel without OCR succeeded, want the soft unavailable error")
	}
}

func TestOCRCandidatesOverTheBoundary(t *testing.T) {
	t.Run("scanned document lists every page", func(t *testing.T) {
		a, _, _, id := newOCRApp(t, scannedPDF)
		pages, err := a.OCRCandidates(id)
		if err != nil {
			t.Fatalf("OCRCandidates: %v", err)
		}
		if len(pages) != scannedPages {
			t.Errorf("candidates = %v, want all %d scanned pages (AC3)", pages, scannedPages)
		}
	})
	t.Run("born-digital document lists none", func(t *testing.T) {
		a, _, _, id := newOCRApp(t, bornDigitalPDF)
		pages, err := a.OCRCandidates(id)
		if err != nil {
			t.Fatalf("OCRCandidates: %v", err)
		}
		if len(pages) != 0 {
			t.Errorf("candidates = %v, want none (AC3)", pages)
		}
	})
	t.Run("unknown handle is an error", func(t *testing.T) {
		a, _, _, _ := newOCRApp(t, scannedPDF)
		if _, err := a.OCRCandidates("no-such-doc"); err == nil {
			t.Error("OCRCandidates(unknown) succeeded, want an error")
		}
	})
}

func TestOCRRunEventsAndResult(t *testing.T) {
	a, _, events, id := newOCRApp(t, scannedPDF)

	if err := a.OCRStart(id); err != nil {
		t.Fatalf("OCRStart: %v", err)
	}
	done := events.waitFor(t, EventOCRDone).(OCRDoneDTO)
	if !done.Ok || done.Cancelled || done.Pages != scannedPages {
		t.Fatalf("done event = %+v, want ok with %d pages", done, scannedPages)
	}
	if got := events.count(EventOCRProgress); got != scannedPages {
		t.Errorf("progress events = %d, want one per candidate page (%d)", got, scannedPages)
	}

	result, err := a.OCRResult(id)
	if err != nil {
		t.Fatalf("OCRResult: %v", err)
	}
	if !result.HasResult || len(result.Pages) != scannedPages {
		t.Fatalf("result = hasResult=%v pages=%d, want the full stored result", result.HasResult, len(result.Pages))
	}
	u := result.Pages[0].Units[0]
	if u.Text != u.EngineText || u.Corrected {
		t.Errorf("uncorrected unit = %+v, want text == engineText, corrected=false", u)
	}
	if u.Box.W <= 0 || u.Box.H <= 0 || u.Confidence <= 0 {
		t.Errorf("unit lost its geometry/confidence over the boundary: %+v", u)
	}
}

func TestOCRResultBeforeAnyRun(t *testing.T) {
	a, rec, _, id := newOCRApp(t, scannedPDF)
	result, err := a.OCRResult(id)
	if err != nil {
		t.Fatalf("OCRResult: %v", err)
	}
	if result.HasResult || result.Error != "" {
		t.Errorf("result before any run = %+v, want empty and soft", result)
	}
	if rec.calls != 0 {
		t.Errorf("reading the result ran the recognizer %d times, want 0 (AC4)", rec.calls)
	}
}

func TestOCRCorrectAndRevertOverTheBoundary(t *testing.T) {
	a, _, events, id := newOCRApp(t, scannedPDF)
	if err := a.OCRStart(id); err != nil {
		t.Fatalf("OCRStart: %v", err)
	}
	events.waitFor(t, EventOCRDone)

	if err := a.OCRCorrect(id, 0, 0, "Goedemorgen"); err != nil {
		t.Fatalf("OCRCorrect: %v", err)
	}
	result, err := a.OCRResult(id)
	if err != nil {
		t.Fatalf("OCRResult: %v", err)
	}
	u := result.Pages[0].Units[0]
	if u.Text != "Goedemorgen" || !u.Corrected || u.EngineText == "Goedemorgen" {
		t.Errorf("corrected unit = %+v, want override applied with the engine original retained (AC6)", u)
	}

	if err := a.OCRRevert(id, 0, 0); err != nil {
		t.Fatalf("OCRRevert: %v", err)
	}
	result, _ = a.OCRResult(id)
	u = result.Pages[0].Units[0]
	if u.Corrected || u.Text != u.EngineText {
		t.Errorf("reverted unit = %+v, want the engine text back", u)
	}

	if err := a.OCRCorrect(id, 0, 999, "x"); err == nil {
		t.Error("OCRCorrect(bad unit) succeeded, want the model's loud error")
	}
}

func TestOCRRecognizePageOverTheBoundary(t *testing.T) {
	a, rec, events, id := newOCRApp(t, scannedPDF)
	if err := a.OCRStart(id); err != nil {
		t.Fatalf("OCRStart: %v", err)
	}
	events.waitFor(t, EventOCRDone)
	before := rec.calls

	if err := a.OCRRecognizePage(id, 1); err != nil {
		t.Fatalf("OCRRecognizePage: %v", err)
	}
	pageDone := events.waitFor(t, EventOCRPageDone).(OCRPageDoneDTO)
	if pageDone.Page != 1 || pageDone.Failed || pageDone.Error != "" {
		t.Fatalf("pageDone = %+v, want page 1 recognized cleanly (AC5)", pageDone)
	}
	if rec.calls != before+1 {
		t.Errorf("recognizer ran %d more times, want exactly 1 (AC5: one page)", rec.calls-before)
	}
	result, _ := a.OCRResult(id)
	if got := result.Pages[1].Units[0].EngineText; got != fmt.Sprintf("unit-%d", before+1) {
		t.Errorf("page 1 = %q, want the re-run's fresh unit", got)
	}
}
