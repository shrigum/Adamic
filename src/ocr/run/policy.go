package run

// Cache & re-OCR policy (task T9; spec A5, AC4, AC5): a document's OCR
// result is whatever the store holds — reading it never triggers
// recognition, so a recognized document is reused on reopen for free, and
// nothing in this package (or its callers) runs OCR on open. Re-running is
// explicit: Start re-runs the whole document, StartPage exactly one page,
// each replacing the stored results of the pages it reaches (a re-run page's
// corrections are dropped with it, ocr.Result.SetPage). Corrections are
// load-modify-save ops on the stored result, refused while a run is in
// flight so the worker's incremental saves and the user's edits cannot lose
// each other's writes.

import (
	"context"
	"fmt"

	"github.com/shrigum/adamic/src/library"
	"github.com/shrigum/adamic/src/ocr"
	"github.com/shrigum/adamic/src/reader"
)

// Result returns the document's stored OCR result, if any — the cached read
// (AC4): it never runs recognition. ok=false means the document has no OCR
// yet. The result carries the user's corrections; readers apply them via
// EffectiveUnits (AC6). A store failure is a returned error the caller
// treats as "no OCR yet" plus a report.
func (r *Runner) Result(id library.DocID) (result ocr.Result, ok bool, err error) {
	r.storeMu.Lock()
	defer r.storeMu.Unlock()
	return r.store.Load(id)
}

// StartPage begins an explicit re-OCR of one page on the worker goroutine
// (AC5): the page is re-detected and re-recognized, and on success its
// stored result is replaced — dropping that page's corrections, whose target
// units are gone (spec A5). It shares the Runner's single run slot with
// Start (ErrRunInProgress) and its cancellation (Cancel/Close).
//
// onDone (if non-nil) fires exactly once, on the worker goroutine, with the
// page's outcome. Soft outcomes ride in the PageResult as a typed Failure; a
// failed attempt does NOT overwrite a previously recognized page (re-OCR
// replaces results, it does not destroy them) — the failure is only
// persisted when the page had no units to lose. The error is non-nil for
// run-level conditions: a page with a usable native text layer (nothing to
// recognize), a closed document, cancellation, or a store that could not
// persist the outcome. An out-of-range page fails Start loudly instead.
func (r *Runner) StartPage(doc *reader.Document, page int, onDone func(ocr.PageResult, error)) error {
	if page < 0 || page >= doc.PageInfo.Count {
		return fmt.Errorf("re-OCR page %d: document has %d pages", page, doc.PageInfo.Count)
	}
	id, err := library.Identify(doc.Path)
	if err != nil {
		return fmt.Errorf("start page re-OCR: %w", err)
	}
	ctx, cancel, done, err := r.begin()
	if err != nil {
		return err
	}
	go r.runPage(ctx, cancel, done, doc, id, page, onDone)
	return nil
}

// runPage is the worker body for StartPage.
func (r *Runner) runPage(ctx context.Context, cancel context.CancelFunc, done chan struct{}, doc *reader.Document, id library.DocID, page int, onDone func(ocr.PageResult, error)) {
	defer close(done)
	defer cancel()

	report := func(pr ocr.PageResult, err error) {
		if onDone != nil {
			onDone(pr, err)
		}
	}

	pr, candidate, err := recognizePage(ctx, r.engine, r.rec, doc, page)
	if err != nil {
		report(pr, err)
		return
	}
	if !candidate {
		report(pr, fmt.Errorf("page %d has a usable text layer; there is nothing to recognize", page+1))
		return
	}

	r.storeMu.Lock()
	defer r.storeMu.Unlock()
	stored, ok, loadErr := r.store.Load(id)
	if !ok || loadErr != nil {
		stored = ocr.Result{ID: id}
	}
	if pr.Failure != nil {
		if existing := pageOf(stored, page); existing != nil && len(existing.Units) > 0 {
			report(pr, nil) // reported, previous good result preserved
			return
		}
	}
	stored.SetPage(pr)
	if err := r.store.Save(stored); err != nil {
		report(pr, fmt.Errorf("persist re-OCR of page %d (the result is usable this session but will not survive a restart): %w", page, err))
		return
	}
	report(pr, nil)
}

// Correct records a user override for a unit's text on the stored result and
// persists it (spec A6, AC6). It is refused with ErrRunInProgress while a
// run is in flight (the worker owns the stored result then), and fails when
// the document has no stored OCR result or the target is invalid
// (ocr.Result.Correct's loud cases).
func (r *Runner) Correct(id library.DocID, page, unit int, text string) error {
	return r.modify(id, "correct", func(result *ocr.Result) error {
		return result.Correct(page, unit, text)
	})
}

// Revert removes the user override for a unit, restoring the engine text on
// read, and persists the removal. Same availability rules as Correct.
func (r *Runner) Revert(id library.DocID, page, unit int) error {
	return r.modify(id, "revert correction", func(result *ocr.Result) error {
		result.Revert(page, unit)
		return nil
	})
}

// modify is the shared load-modify-save of the stored result.
func (r *Runner) modify(id library.DocID, doing string, change func(*ocr.Result) error) error {
	if r.running() {
		return ErrRunInProgress
	}
	r.storeMu.Lock()
	defer r.storeMu.Unlock()
	result, ok, err := r.store.Load(id)
	if err != nil {
		return fmt.Errorf("%s: %w", doing, err)
	}
	if !ok {
		return fmt.Errorf("%s: document has no OCR result", doing)
	}
	if err := change(&result); err != nil {
		return fmt.Errorf("%s: %w", doing, err)
	}
	if err := r.store.Save(result); err != nil {
		return fmt.Errorf("%s: %w", doing, err)
	}
	return nil
}

// pageOf returns the stored entry for a page, or nil.
func pageOf(result ocr.Result, page int) *ocr.PageResult {
	for i := range result.Pages {
		if result.Pages[i].Page == page {
			return &result.Pages[i]
		}
	}
	return nil
}
