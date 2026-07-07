package document

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/shrigum/adamic/src/reader"
)

// newTestEngine starts a real PDFium wasm engine for the test and shuts it down
// on cleanup. Engine construction is a few hundred ms (wasm module init), so
// tests share one engine per test function rather than per subtest.
func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	e, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	t.Cleanup(func() {
		if err := e.Shutdown(); err != nil {
			t.Errorf("Shutdown: %v", err)
		}
	})
	return e
}

func fixturePath(name string) string { return filepath.Join("testdata", name) }

func TestEngineOpenAndPageCount(t *testing.T) {
	e := newTestEngine(t)

	doc, err := e.Open(fixturePath(fixture))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer e.Close(doc.ID)

	if doc.PageInfo.Count != fixturePageCount {
		t.Errorf("page count = %d, want %d (spec AC1)", doc.PageInfo.Count, fixturePageCount)
	}
	if len(doc.PageInfo.Sizes) != fixturePageCount {
		t.Fatalf("len(Sizes) = %d, want %d (one per page)", len(doc.PageInfo.Sizes), fixturePageCount)
	}
	// The fixture is A4 portrait: ~595x842 pt. Assert portrait + sane range,
	// not exact values (avoids brittleness across PDFium versions).
	s := doc.PageInfo.Sizes[0]
	if s.WidthPt < 400 || s.HeightPt <= s.WidthPt {
		t.Errorf("page 0 size = %.0fx%.0f pt, want portrait A4-ish", s.WidthPt, s.HeightPt)
	}

	// PageCount command agrees with the count returned by Open.
	pc, err := e.PageCount(doc.ID)
	if err != nil {
		t.Fatalf("PageCount: %v", err)
	}
	if pc != fixturePageCount {
		t.Errorf("PageCount() = %d, want %d", pc, fixturePageCount)
	}
}

func TestEngineRenderPageScales(t *testing.T) {
	e := newTestEngine(t)
	doc, err := e.Open(fixturePath(fixture))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer e.Close(doc.ID)

	size := doc.PageInfo.Sizes[0]

	// Zoom 1.0 → one pixel per point.
	img, err := e.RenderPage(doc.ID, 0, reader.Scale{Zoom: 1})
	if err != nil {
		t.Fatalf("RenderPage zoom 1: %v", err)
	}
	wantW, wantH := reader.Scale{Zoom: 1}.PixelSize(size.WidthPt, size.HeightPt)
	if img.Bounds().Dx() != wantW || img.Bounds().Dy() != wantH {
		t.Errorf("zoom-1 image = %dx%d, want %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), wantW, wantH)
	}
	if isBlank(img) {
		t.Error("rendered page is blank (spec AC1: faithful raster)")
	}

	// Fit-to-width doubles the size when the viewport is twice the page width.
	vp := reader.Viewport{WidthPx: int(size.WidthPt * 2), HeightPx: 100}
	fit, err := e.RenderPage(doc.ID, 0, reader.Scale{FitWidth: true, Viewport: vp})
	if err != nil {
		t.Fatalf("RenderPage fit-width: %v", err)
	}
	if got := fit.Bounds().Dx(); got != vp.WidthPx {
		t.Errorf("fit-width image width = %d, want %d (spec AC4)", got, vp.WidthPx)
	}
}

func TestEngineRenderPageOutOfRange(t *testing.T) {
	e := newTestEngine(t)
	doc, err := e.Open(fixturePath(fixture))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer e.Close(doc.ID)

	if _, err := e.RenderPage(doc.ID, fixturePageCount, reader.Scale{Zoom: 1}); !errors.Is(err, reader.ErrPageOutOfRange) {
		t.Errorf("render past last page: want ErrPageOutOfRange, got %v (spec AC5)", err)
	}
	if _, err := e.RenderPage(doc.ID, -1, reader.Scale{Zoom: 1}); !errors.Is(err, reader.ErrPageOutOfRange) {
		t.Errorf("render page -1: want ErrPageOutOfRange, got %v", err)
	}
}

func TestEngineClosedDocument(t *testing.T) {
	e := newTestEngine(t)
	doc, err := e.Open(fixturePath(fixture))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := e.Close(doc.ID); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := e.PageCount(doc.ID); !errors.Is(err, reader.ErrClosedDocument) {
		t.Errorf("PageCount after Close: want ErrClosedDocument, got %v", err)
	}
	if _, err := e.RenderPage(doc.ID, 0, reader.Scale{Zoom: 1}); !errors.Is(err, reader.ErrClosedDocument) {
		t.Errorf("RenderPage after Close: want ErrClosedDocument, got %v", err)
	}
	// Double close is a no-op.
	if err := e.Close(doc.ID); err != nil {
		t.Errorf("double Close: want nil, got %v", err)
	}
}

// TestEngineOpenErrors is T13: every bad-input open is a classified, soft
// *reader.OpenError, and the engine stays usable afterward (spec AC9/AC10).
func TestEngineOpenErrors(t *testing.T) {
	e := newTestEngine(t)

	tests := []struct {
		name string
		path string
		want reader.OpenKind
	}{
		{"missing file", fixturePath("does-not-exist.pdf"), reader.OpenNotFound},
		{"non-PDF file", fixturePath("not-a-pdf.txt"), reader.OpenNotPDF},
		{"corrupt PDF", fixturePath("corrupt.pdf"), reader.OpenCorrupt},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := e.Open(tt.path)
			var oe *reader.OpenError
			if !errors.As(err, &oe) {
				t.Fatalf("want *reader.OpenError, got %v", err)
			}
			if oe.Kind != tt.want {
				t.Errorf("Kind = %v, want %v", oe.Kind, tt.want)
			}
		})
	}

	// The engine still works after all those failures — no crash, no wedged
	// instance (spec AC9: app stays running and usable).
	doc, err := e.Open(fixturePath(fixture))
	if err != nil {
		t.Fatalf("engine unusable after open errors: %v", err)
	}
	e.Close(doc.ID)
}
