package run

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/shrigum/adamic/src/document"
	"github.com/shrigum/adamic/src/library"
	"github.com/shrigum/adamic/src/ocr"
	"github.com/shrigum/adamic/src/reader"
)

// ErrRunInProgress is returned by Runner.Start while a run is in flight —
// callers branch on it to offer cancel-and-restart instead (one run at a
// time, matching the one-document product model, spec A8).
var ErrRunInProgress = errors.New("an OCR run is already in progress; cancel it or wait for it to finish")

// Progress reports one candidate page finishing during a run (spec A8: the
// user sees OCR advancing). Skipped born-digital pages emit no event.
type Progress struct {
	// Page is the zero-based index of the page that just finished.
	Page int

	// PageCount is the document's total page count — an upper bound on the
	// remaining work (how many of those are candidates is only known as
	// detection reaches them).
	PageCount int

	// Failed reports that this page finished with a typed failure rather
	// than recognized units (the failure itself is in the run's Result).
	Failed bool
}

// Runner drives OCR runs off the caller's goroutine (task T7): it owns the
// worker goroutine, its cancellation, and the incremental persistence of
// results. One Runner runs at most one document at a time; it is the single
// owner of the run's lifecycle (design-review implementer note — no orphaned
// goroutines: Close cancels and waits).
//
// Persistence (spec A8, AC7): each candidate page's result is upserted into
// the document's stored OCR result as it completes — atomically, via the
// store — so cancelling mid-run leaves every finished page persisted and
// usable, and pages from an earlier run that this run has not (yet) reached
// are preserved, never clobbered.
//
// Failure modes: Start fails only on ErrRunInProgress or an unidentifiable
// document. Everything after Start is reported through onDone: per-page
// problems ride inside the Result as typed PageFailures (AC8); a cancelled
// run reports an error wrapping context.Canceled alongside the partial
// Result; a store that cannot save is soft — recognition continues, the
// in-memory Result is still delivered, and the persistence failure is
// joined into onDone's error.
type Runner struct {
	engine *document.Engine
	rec    ocr.Recognizer
	store  ocr.Store

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewRunner returns a Runner over the given engine, recognizer, and store.
func NewRunner(engine *document.Engine, rec ocr.Recognizer, store ocr.Store) *Runner {
	return &Runner{engine: engine, rec: rec, store: store}
}

// Start begins OCR of an open document on a worker goroutine and returns
// immediately. onProgress (if non-nil) is called after each candidate page,
// and onDone (if non-nil) exactly once when the run ends — with the (possibly
// partial) Result and the run's error, nil on full success. Both callbacks
// run on the worker goroutine; keep them fast and marshal to the UI outside.
func (r *Runner) Start(doc *reader.Document, onProgress func(Progress), onDone func(ocr.Result, error)) error {
	// Identify up front: with no identity nothing could be persisted (AC12),
	// better to refuse before burning minutes of recognition.
	id, err := library.Identify(doc.Path)
	if err != nil {
		return fmt.Errorf("start OCR run: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.done != nil {
		select {
		case <-r.done:
		default:
			return ErrRunInProgress
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	r.cancel, r.done = cancel, done

	go r.run(ctx, cancel, done, doc, id, onProgress, onDone)
	return nil
}

// Cancel cancels the in-flight run, if any, and returns immediately; the
// run's onDone still fires (with the partial result). Safe to call at any
// time, including with no run in flight.
func (r *Runner) Cancel() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		r.cancel()
	}
}

// Close cancels any in-flight run and waits for its goroutine to exit. After
// Close returns no Runner goroutine is left running (callbacks included).
func (r *Runner) Close() {
	r.mu.Lock()
	cancel, done := r.cancel, r.done
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

// run is the worker goroutine body: the T5 document run with per-page
// persistence and progress, then the single onDone report.
func (r *Runner) run(ctx context.Context, cancel context.CancelFunc, done chan struct{}, doc *reader.Document, id library.DocID, onProgress func(Progress), onDone func(ocr.Result, error)) {
	defer close(done)
	defer cancel()

	// Merge base: the document's stored result, if any, so pages this run
	// does not reach (cancel) or does not re-run keep their earlier results.
	// A load failure is soft — it means "no OCR yet" (store contract).
	stored, ok, loadErr := r.store.Load(id)
	if !ok || loadErr != nil {
		stored = ocr.Result{ID: id}
	}

	var saveErr error
	result, runErr := Document(ctx, r.engine, r.rec, doc, func(pr ocr.PageResult) {
		stored = upsertPage(stored, pr)
		if err := r.store.Save(stored); err != nil && saveErr == nil {
			saveErr = fmt.Errorf("persist OCR result (recognition continued; results are usable this session but will not survive a restart): %w", err)
		}
		if onProgress != nil {
			onProgress(Progress{Page: pr.Page, PageCount: doc.PageInfo.Count, Failed: pr.Failure != nil})
		}
	})

	if onDone != nil {
		onDone(result, errors.Join(runErr, saveErr))
	}
}

// upsertPage returns result with pr replacing its existing entry for that
// page, or inserted in ascending page order — preserving the Result contract
// (ascending, no duplicates) over any merge of runs.
func upsertPage(result ocr.Result, pr ocr.PageResult) ocr.Result {
	pages := result.Pages
	for i, existing := range pages {
		if existing.Page == pr.Page {
			pages[i] = pr
			return result
		}
		if existing.Page > pr.Page {
			pages = append(pages[:i], append([]ocr.PageResult{pr}, pages[i:]...)...)
			result.Pages = pages
			return result
		}
	}
	result.Pages = append(pages, pr)
	return result
}
