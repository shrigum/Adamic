package ocr_test

import (
	"strings"
	"testing"

	"github.com/shrigum/adamic/src/ocr"
)

// correctable returns a result with a recognized page 0 (two units), a gap at
// page 1, and a failed page 2 — the shapes Correct must distinguish.
func correctable() ocr.Result {
	return ocr.Result{
		ID: "doc",
		Pages: []ocr.PageResult{
			{Page: 0, Units: []ocr.RecognizedUnit{
				{Text: "Goedemorgen", Box: ocr.Box{X: 1, Y: 1, W: 10, H: 2}, Confidence: 0.9},
				{Text: "dz", Box: ocr.Box{X: 1, Y: 5, W: 10, H: 2}, Confidence: 0.3},
			}},
			{Page: 2, Failure: &ocr.PageFailure{Kind: ocr.FailureUnreadable, Message: "boom"}},
		},
	}
}

func TestCorrectionTakesPrecedenceAndRetainsOriginal(t *testing.T) {
	r := correctable()
	if err := r.Correct(0, 1, "Dag"); err != nil {
		t.Fatalf("Correct: %v", err)
	}

	units, ok := r.EffectiveUnits(0)
	if !ok {
		t.Fatal("EffectiveUnits(0) ok = false")
	}
	if units[1].Text != "Dag" {
		t.Errorf("effective text = %q, want the correction %q (AC6 precedence)", units[1].Text, "Dag")
	}
	if units[0].Text != "Goedemorgen" {
		t.Errorf("uncorrected unit changed: %q", units[0].Text)
	}
	if got := r.Pages[0].Units[1].Text; got != "dz" {
		t.Errorf("engine original = %q, want %q retained (AC6 revertibility)", got, "dz")
	}
	if text, ok := r.CorrectionFor(0, 1); !ok || text != "Dag" {
		t.Errorf("CorrectionFor = %q, %v; want the stored override", text, ok)
	}
	// The box and confidence ride along unchanged — only text is overridden.
	if units[1].Box != r.Pages[0].Units[1].Box || units[1].Confidence != r.Pages[0].Units[1].Confidence {
		t.Error("correction changed the unit's box or confidence")
	}
}

func TestCorrectionReplacedAndReverted(t *testing.T) {
	r := correctable()
	if err := r.Correct(0, 1, "Dag"); err != nil {
		t.Fatalf("Correct: %v", err)
	}
	if err := r.Correct(0, 1, "Dag!"); err != nil {
		t.Fatalf("Correct again: %v", err)
	}
	if len(r.Corrections) != 1 {
		t.Fatalf("got %d corrections after re-correcting one unit, want 1", len(r.Corrections))
	}
	units, _ := r.EffectiveUnits(0)
	if units[1].Text != "Dag!" {
		t.Errorf("effective text = %q, want the latest correction", units[1].Text)
	}

	r.Revert(0, 1)
	if _, ok := r.CorrectionFor(0, 1); ok {
		t.Error("correction still present after Revert")
	}
	units, _ = r.EffectiveUnits(0)
	if units[1].Text != "dz" {
		t.Errorf("effective text after Revert = %q, want the engine original", units[1].Text)
	}
	r.Revert(0, 1) // reverting again is a no-op, not a panic
}

func TestCorrectionInvalidTargetsAreLoud(t *testing.T) {
	r := correctable()
	tests := []struct {
		name       string
		page, unit int
		text       string
		wantErr    string
	}{
		{"page without a result", 1, 0, "x", "no OCR result"},
		{"failed page", 2, 0, "x", "failed recognition"},
		{"unit out of range", 0, 2, "x", "has 2 units"},
		{"negative unit", 0, -1, "x", "has 2 units"},
		{"empty text", 0, 0, "  ", "Revert"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.Correct(tt.page, tt.unit, tt.text)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Correct(%d, %d, %q) error = %v, want containing %q", tt.page, tt.unit, tt.text, err, tt.wantErr)
			}
		})
	}
	if len(r.Corrections) != 0 {
		t.Errorf("invalid corrections were stored: %+v", r.Corrections)
	}
}

func TestCorrectionsStaySorted(t *testing.T) {
	r := ocr.Result{ID: "doc", Pages: []ocr.PageResult{
		{Page: 0, Units: []ocr.RecognizedUnit{{Text: "a", Box: ocr.Box{W: 1, H: 1}, Confidence: 1}, {Text: "b", Box: ocr.Box{W: 1, H: 1}, Confidence: 1}}},
		{Page: 3, Units: []ocr.RecognizedUnit{{Text: "c", Box: ocr.Box{W: 1, H: 1}, Confidence: 1}}},
	}}
	for _, c := range []ocr.Correction{{Page: 3, Unit: 0, Text: "C"}, {Page: 0, Unit: 1, Text: "B"}, {Page: 0, Unit: 0, Text: "A"}} {
		if err := r.Correct(c.Page, c.Unit, c.Text); err != nil {
			t.Fatalf("Correct(%+v): %v", c, err)
		}
	}
	want := []ocr.Correction{{Page: 0, Unit: 0, Text: "A"}, {Page: 0, Unit: 1, Text: "B"}, {Page: 3, Unit: 0, Text: "C"}}
	for i, c := range r.Corrections {
		if c != want[i] {
			t.Fatalf("Corrections[%d] = %+v, want %+v (sorted by page, unit)", i, c, want[i])
		}
	}
}

func TestEffectiveUnitsMissingAndFailedPages(t *testing.T) {
	r := correctable()
	if _, ok := r.EffectiveUnits(1); ok {
		t.Error("EffectiveUnits(no result) ok = true, want false")
	}
	units, ok := r.EffectiveUnits(2)
	if !ok || units != nil {
		t.Errorf("EffectiveUnits(failed page) = %v, %v; want nil units, ok=true", units, ok)
	}
}

func TestSetPageDropsOnlyThatPagesCorrections(t *testing.T) {
	r := correctable()
	if err := r.Correct(0, 0, "Hoi"); err != nil {
		t.Fatalf("Correct: %v", err)
	}
	r.Pages = append(r.Pages, ocr.PageResult{Page: 3, Units: []ocr.RecognizedUnit{{Text: "x", Box: ocr.Box{W: 1, H: 1}, Confidence: 1}}})
	if err := r.Correct(3, 0, "y"); err != nil {
		t.Fatalf("Correct page 3: %v", err)
	}

	// Re-OCR page 0: its correction addressed units that no longer exist.
	r.SetPage(ocr.PageResult{Page: 0, Units: []ocr.RecognizedUnit{{Text: "fresh", Box: ocr.Box{W: 1, H: 1}, Confidence: 1}}})

	if _, ok := r.CorrectionFor(0, 0); ok {
		t.Error("page 0 correction survived that page's re-OCR (spec A5: replaced)")
	}
	if text, ok := r.CorrectionFor(3, 0); !ok || text != "y" {
		t.Errorf("page 3 correction = %q, %v; want untouched", text, ok)
	}
	units, _ := r.EffectiveUnits(0)
	if len(units) != 1 || units[0].Text != "fresh" {
		t.Errorf("page 0 after SetPage = %+v, want the replacement units", units)
	}
}

func TestSetPageKeepsAscendingOrder(t *testing.T) {
	var r ocr.Result
	for _, p := range []int{2, 0, 3, 1, 2} {
		r.SetPage(ocr.PageResult{Page: p})
	}
	if len(r.Pages) != 4 {
		t.Fatalf("got %d pages, want 4 distinct", len(r.Pages))
	}
	for i, pr := range r.Pages {
		if pr.Page != i {
			t.Errorf("Pages[%d].Page = %d, want ascending 0..3 (Result contract)", i, pr.Page)
		}
	}
}
