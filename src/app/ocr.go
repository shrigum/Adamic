package app

// The OCR binding (task T11 of docs/planning/ocr/critical-path.md): the
// commands the frontend needs — detect candidates, start/cancel a run, read
// the result with corrections applied, correct/revert a unit, re-OCR one
// page — as flat, JSON-serializable methods over the run.Runner policy
// component. No engine or persistence logic lives here.
//
// Runs are asynchronous: OCRStart/OCRRecognizePage return once the run is
// accepted, and progress/completion arrive as events through the emit
// function EnableOCR installs (the desktop shell passes the Wails event
// emitter; tests pass a collector). Every method fails softly with a
// user-facing error when OCR was never enabled (no engine on this system) —
// the reader keeps working without recognition (spec error summary).

import (
	"context"
	"errors"
	"fmt"

	"github.com/shrigum/adamic/src/document"
	"github.com/shrigum/adamic/src/library"
	"github.com/shrigum/adamic/src/ocr"
	"github.com/shrigum/adamic/src/ocr/run"
)

// Event names the frontend subscribes to for OCR runs.
const (
	// EventOCRProgress carries an OCRProgressDTO after each candidate page
	// of a full run.
	EventOCRProgress = "ocr:progress"

	// EventOCRDone carries an OCRDoneDTO exactly once per full run.
	EventOCRDone = "ocr:done"

	// EventOCRPageDone carries an OCRPageDoneDTO exactly once per explicit
	// single-page re-OCR.
	EventOCRPageDone = "ocr:pageDone"
)

// ocrBinding is the OCR surface's wiring, installed by EnableOCR.
type ocrBinding struct {
	engine *document.Engine
	runner *run.Runner
	emit   func(name string, data any)
}

// EnableOCR wires the OCR commands: the engine for detection, the runner for
// runs/results/corrections, and the event emitter progress flows through.
// Before this is called (or when the desktop shell finds no OCR engine at
// startup and never calls it), every OCR command fails softly, saying
// recognition is unavailable.
func (a *App) EnableOCR(engine *document.Engine, runner *run.Runner, emit func(name string, data any)) {
	if emit == nil {
		emit = func(string, any) {}
	}
	a.mu.Lock()
	a.ocr = &ocrBinding{engine: engine, runner: runner, emit: emit}
	a.mu.Unlock()
}

// requireOCR returns the OCR wiring, or the soft "not available" error every
// OCR command shares.
func (a *App) requireOCR() (*ocrBinding, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ocr == nil {
		return nil, errors.New("text recognition is not available: no OCR engine was found on this system")
	}
	return a.ocr, nil
}

// docID returns the stable document identity for an open handle.
func (a *App) docID(id string) (library.DocID, error) {
	doc, err := a.doc(id)
	if err != nil {
		return "", err
	}
	key, err := library.Identify(doc.Path)
	if err != nil {
		return "", fmt.Errorf("identify document: %w", err)
	}
	return key, nil
}

// OCRProgressDTO is one full-run progress event: a candidate page finished.
type OCRProgressDTO struct {
	Page      int  `json:"page"`
	PageCount int  `json:"pageCount"`
	Failed    bool `json:"failed"`
}

// OCRDoneDTO ends a full run. On success Ok is true and Pages counts the
// candidate pages in the (possibly merged) result; a cancelled run has
// Cancelled true; anything else carries a user-facing Error. After this
// event the frontend refetches OCRResult.
type OCRDoneDTO struct {
	Ok        bool   `json:"ok"`
	Cancelled bool   `json:"cancelled"`
	Pages     int    `json:"pages"`
	Error     string `json:"error,omitempty"`
}

// OCRPageDoneDTO ends an explicit single-page re-OCR.
type OCRPageDoneDTO struct {
	Page   int    `json:"page"`
	Failed bool   `json:"failed"`
	Error  string `json:"error,omitempty"`
}

// OCRResultDTO is a document's stored OCR result. HasResult false means the
// document has no OCR yet; a store problem reads the same way, softly, with
// its message in Error (the document stays readable either way).
type OCRResultDTO struct {
	HasResult bool         `json:"hasResult"`
	Pages     []OCRPageDTO `json:"pages,omitempty"`
	Error     string       `json:"error,omitempty"`
}

// OCRPageDTO is one candidate page's outcome: units or a typed failure.
type OCRPageDTO struct {
	Page    int            `json:"page"`
	Units   []OCRUnitDTO   `json:"units,omitempty"`
	Failure *OCRFailureDTO `json:"failure,omitempty"`
}

// OCRUnitDTO is one recognized unit as the review UI consumes it (spec A6,
// A14): Text is the effective text (user correction applied), EngineText the
// engine's original, and Box the page-point rectangle that is the unit's
// on-page location and hit target.
type OCRUnitDTO struct {
	Text       string  `json:"text"`
	EngineText string  `json:"engineText"`
	Corrected  bool    `json:"corrected"`
	Box        BoxDTO  `json:"box"`
	Confidence float64 `json:"confidence"`
	Group      string  `json:"group,omitempty"`
}

// BoxDTO is a page-point rectangle (top-left origin, y down).
type BoxDTO struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// OCRFailureDTO is a page's typed recognition failure, for display.
type OCRFailureDTO struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

// OCRCandidates returns the zero-based pages of an open document that need
// OCR (spec A3) — empty when the document reads natively. Detection only; no
// recognition runs.
func (a *App) OCRCandidates(id string) ([]int, error) {
	b, err := a.requireOCR()
	if err != nil {
		return nil, err
	}
	doc, err := a.doc(id)
	if err != nil {
		return nil, err
	}
	return run.Candidates(b.engine, doc)
}

// OCRStart begins recognition of a document's candidate pages. It returns
// once the run is accepted; progress and completion arrive as
// EventOCRProgress/EventOCRDone events. Starting while a run is in flight is
// an error the frontend shows.
func (a *App) OCRStart(id string) error {
	b, err := a.requireOCR()
	if err != nil {
		return err
	}
	doc, err := a.doc(id)
	if err != nil {
		return err
	}
	return b.runner.Start(doc,
		func(p run.Progress) {
			b.emit(EventOCRProgress, OCRProgressDTO{Page: p.Page, PageCount: p.PageCount, Failed: p.Failed})
		},
		func(result ocr.Result, err error) {
			b.emit(EventOCRDone, doneDTO(result, err))
		})
}

// OCRCancel cancels the in-flight run, if any; the run still ends with its
// EventOCRDone (Cancelled true). Already-recognized pages stay persisted
// (spec AC7).
func (a *App) OCRCancel() error {
	b, err := a.requireOCR()
	if err != nil {
		return err
	}
	b.runner.Cancel()
	return nil
}

// OCRRecognizePage explicitly re-OCRs one page (spec A5, AC5), replacing its
// stored result on success. Completion arrives as EventOCRPageDone.
func (a *App) OCRRecognizePage(id string, page int) error {
	b, err := a.requireOCR()
	if err != nil {
		return err
	}
	doc, err := a.doc(id)
	if err != nil {
		return err
	}
	return b.runner.StartPage(doc, page, func(pr ocr.PageResult, err error) {
		dto := OCRPageDoneDTO{Page: page, Failed: pr.Failure != nil}
		if err != nil {
			dto.Error = err.Error()
		}
		b.emit(EventOCRPageDone, dto)
	})
}

// OCRResult returns the document's stored OCR result with corrections
// applied to each unit's Text (AC6) and the engine originals alongside.
// Reading never triggers recognition (AC4).
func (a *App) OCRResult(id string) (OCRResultDTO, error) {
	b, err := a.requireOCR()
	if err != nil {
		return OCRResultDTO{}, err
	}
	key, err := a.docID(id)
	if err != nil {
		return OCRResultDTO{}, err
	}
	result, ok, err := b.runner.Result(key)
	if err != nil {
		return OCRResultDTO{Error: err.Error()}, nil
	}
	if !ok {
		return OCRResultDTO{}, nil
	}

	dto := OCRResultDTO{HasResult: true, Pages: make([]OCRPageDTO, 0, len(result.Pages))}
	for _, pr := range result.Pages {
		pageDTO := OCRPageDTO{Page: pr.Page}
		if pr.Failure != nil {
			pageDTO.Failure = &OCRFailureDTO{Kind: string(pr.Failure.Kind), Message: pr.Failure.Message}
		}
		effective, _ := result.EffectiveUnits(pr.Page)
		for i, u := range pr.Units {
			_, corrected := result.CorrectionFor(pr.Page, i)
			pageDTO.Units = append(pageDTO.Units, OCRUnitDTO{
				Text:       effective[i].Text,
				EngineText: u.Text,
				Corrected:  corrected,
				Box:        BoxDTO{X: u.Box.X, Y: u.Box.Y, W: u.Box.W, H: u.Box.H},
				Confidence: u.Confidence,
				Group:      u.Group,
			})
		}
		dto.Pages = append(dto.Pages, pageDTO)
	}
	return dto, nil
}

// OCRCorrect stores a user override for a unit's text (spec A6, AC6). The
// engine original is retained; OCRResult shows the override with
// Corrected=true. Refused while a run is in flight.
func (a *App) OCRCorrect(id string, page, unit int, text string) error {
	b, err := a.requireOCR()
	if err != nil {
		return err
	}
	key, err := a.docID(id)
	if err != nil {
		return err
	}
	return b.runner.Correct(key, page, unit, text)
}

// OCRRevert removes a unit's user override, restoring the engine text.
func (a *App) OCRRevert(id string, page, unit int) error {
	b, err := a.requireOCR()
	if err != nil {
		return err
	}
	key, err := a.docID(id)
	if err != nil {
		return err
	}
	return b.runner.Revert(key, page, unit)
}

// doneDTO maps a run's outcome onto the done event payload.
func doneDTO(result ocr.Result, err error) OCRDoneDTO {
	dto := OCRDoneDTO{Pages: len(result.Pages)}
	switch {
	case err == nil:
		dto.Ok = true
	case errors.Is(err, context.Canceled):
		dto.Cancelled = true
	default:
		dto.Error = err.Error()
	}
	return dto
}
