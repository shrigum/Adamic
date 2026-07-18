package ocr

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/shrigum/adamic/src/library"
	"github.com/shrigum/adamic/src/reader"
)

// a4 is an A4 page in points, the fixture's page profile.
var a4 = reader.PageSize{WidthPt: 595, HeightPt: 842}

func validUnit() RecognizedUnit {
	return RecognizedUnit{
		Text:       "Nederlands",
		Box:        Box{X: 72, Y: 100, W: 120, H: 14},
		Confidence: 0.93,
		Group:      "line-3",
	}
}

// TestValidate pins the contract invariants of spec AC2: non-empty text,
// confidence in [0, 1], positive box size, box inside the page bounds.
func TestValidate(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*RecognizedUnit)
		wantOK bool
	}{
		{"valid unit passes", func(u *RecognizedUnit) {}, true},
		{"no group id is valid", func(u *RecognizedUnit) { u.Group = "" }, true},
		{"confidence zero is valid", func(u *RecognizedUnit) { u.Confidence = 0 }, true},
		{"confidence one is valid", func(u *RecognizedUnit) { u.Confidence = 1 }, true},
		{"box touching the page edges is inside", func(u *RecognizedUnit) {
			u.Box = Box{X: 0, Y: 0, W: a4.WidthPt, H: a4.HeightPt}
		}, true},
		{"empty text fails", func(u *RecognizedUnit) { u.Text = "" }, false},
		{"confidence below zero fails", func(u *RecognizedUnit) { u.Confidence = -0.01 }, false},
		{"confidence above one fails", func(u *RecognizedUnit) { u.Confidence = 1.01 }, false},
		{"unnormalized tesseract confidence fails", func(u *RecognizedUnit) { u.Confidence = 93 }, false},
		{"zero-width box fails", func(u *RecognizedUnit) { u.Box.W = 0 }, false},
		{"negative-height box fails", func(u *RecognizedUnit) { u.Box.H = -5 }, false},
		{"box past the right edge fails", func(u *RecognizedUnit) { u.Box.X = a4.WidthPt - 1 }, false},
		{"box past the bottom edge fails", func(u *RecognizedUnit) { u.Box.Y = a4.HeightPt - 1 }, false},
		{"negative origin fails", func(u *RecognizedUnit) { u.Box.X = -1 }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := validUnit()
			tc.mutate(&u)
			err := u.Validate(a4)
			if tc.wantOK && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
			if !tc.wantOK && err == nil {
				t.Fatalf("Validate() = nil, want error for %+v", u)
			}
		})
	}
}

// TestResultJSONRoundTrip pins that the full result shape — units, groups, and
// a typed per-page failure — survives JSON encode/decode unchanged. The store
// (T6) persists this shape and the app binding (T11) serializes it; a field
// that cannot round-trip is a contract bug.
func TestResultJSONRoundTrip(t *testing.T) {
	orig := Result{
		ID: library.DocID("/books/taalcompleet.pdf\x00abc123"),
		Pages: []PageResult{
			{Page: 0, Units: []RecognizedUnit{validUnit()}},
			{Page: 2, Units: []RecognizedUnit{
				{Text: "Les 1", Box: Box{X: 50, Y: 60, W: 40, H: 12}, Confidence: 1},
			}},
			{Page: 3, Failure: &PageFailure{
				Kind:    FailureUnreadable,
				Message: "page image could not be decoded; re-run OCR after checking the file",
			}},
		},
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Result
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(orig, got) {
		t.Fatalf("round trip changed the result:\norig: %+v\ngot:  %+v", orig, got)
	}
}

// TestPageResultSuccessAndFailureAreDistinct pins the AC8 shape: a failed page
// carries a failure and no units; a blank-but-readable page is a success with
// zero units, not a failure.
func TestPageResultSuccessAndFailureAreDistinct(t *testing.T) {
	failed := PageResult{Page: 1, Failure: &PageFailure{Kind: FailureEngine, Message: "OCR engine missing"}}
	if failed.Units != nil {
		t.Fatalf("failed page should carry no units, got %v", failed.Units)
	}
	blank := PageResult{Page: 4}
	if blank.Failure != nil {
		t.Fatalf("blank page is a success, got failure %+v", blank.Failure)
	}
}
