package app

import (
	"strings"
	"testing"

	"github.com/shrigum/adamic/src/reader"
)

func newTestApp(pages int) *App { return New(reader.NewStubReader(pages)) }

func TestOpenSuccessShape(t *testing.T) {
	a := newTestApp(5)
	res := a.Open("/books/nl.pdf")
	if !res.Ok || res.Doc == nil {
		t.Fatalf("Open ok=%v doc=%v, want success", res.Ok, res.Doc)
	}
	if len(res.Doc.Pages) != 5 {
		t.Errorf("pages = %d, want 5", len(res.Doc.Pages))
	}
	if res.Doc.Pages[0].WidthPt <= 0 || res.Doc.Pages[0].HeightPt <= 0 {
		t.Errorf("page size not populated: %+v", res.Doc.Pages[0])
	}
	if res.Doc.Position != (PositionDTO{}) {
		t.Errorf("never-opened doc position = %+v, want zero", res.Doc.Position)
	}
}

func TestRenderPageReturnsDataURL(t *testing.T) {
	a := newTestApp(3)
	res := a.Open("/x.pdf")

	url, err := a.RenderPage(res.Doc.ID, 0, 1.0)
	if err != nil {
		t.Fatalf("RenderPage: %v", err)
	}
	if !strings.HasPrefix(url, "data:image/png;base64,") {
		t.Errorf("render result is not a PNG data URL: %.40q", url)
	}
	if len(url) < 100 {
		t.Errorf("data URL suspiciously short (%d chars) — empty image?", len(url))
	}
}

func TestRenderPageFitProducesViewportWidth(t *testing.T) {
	a := newTestApp(2)
	res := a.Open("/x.pdf")
	// Fit-width should not error and should return a decodable data URL.
	if _, err := a.RenderPageFit(res.Doc.ID, 0, 800, 600, false); err != nil {
		t.Errorf("RenderPageFit (fit-width): %v", err)
	}
	if _, err := a.RenderPageFit(res.Doc.ID, 0, 800, 600, true); err != nil {
		t.Errorf("RenderPageFit (fit-page): %v", err)
	}
}

func TestThumbnailReturnsDataURL(t *testing.T) {
	a := newTestApp(2)
	res := a.Open("/x.pdf")
	url, err := a.Thumbnail(res.Doc.ID, 1)
	if err != nil {
		t.Fatalf("Thumbnail: %v", err)
	}
	if !strings.HasPrefix(url, "data:image/png;base64,") {
		t.Errorf("thumbnail is not a data URL: %.40q", url)
	}
}

func TestPositionRoundTripThroughApp(t *testing.T) {
	a := newTestApp(10)
	res := a.Open("/book.pdf")
	if err := a.SetPosition(res.Doc.ID, 4, 0.5); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}
	got, err := a.GetPosition(res.Doc.ID)
	if err != nil {
		t.Fatalf("GetPosition: %v", err)
	}
	if got != (PositionDTO{Page: 4, OffsetY: 0.5}) {
		t.Errorf("position = %+v, want {4, 0.5}", got)
	}
}

func TestOpenSoftErrorsAreNotGoErrors(t *testing.T) {
	// The stub never fails to open, so drive the translation with a reader that
	// returns each OpenError kind.
	kinds := []struct {
		kind     reader.OpenKind
		wantKind string
	}{
		{reader.OpenNotFound, "not-found"},
		{reader.OpenNotPDF, "not-pdf"},
		{reader.OpenCorrupt, "corrupt"},
		{reader.OpenPasswordReqd, "password"},
	}
	for _, tc := range kinds {
		a := New(failingReader{kind: tc.kind})
		res := a.Open("/whatever.pdf")
		if res.Ok {
			t.Errorf("%v: Open reported Ok, want soft failure", tc.kind)
			continue
		}
		if res.Error == nil || res.Error.Kind != tc.wantKind {
			t.Errorf("%v: error kind = %+v, want %q", tc.kind, res.Error, tc.wantKind)
		}
		if res.Error.Message == "" {
			t.Errorf("%v: soft error has no user message", tc.kind)
		}
	}
}

// failingReader is a reader.Reader whose Open always fails with a chosen kind;
// every other method is unused in these tests.
type failingReader struct {
	reader.Reader
	kind reader.OpenKind
}

func (f failingReader) Open(path string) (*reader.Document, error) {
	return nil, &reader.OpenError{Path: path, Kind: f.kind}
}
