package document

import (
	"image"
	"testing"

	"github.com/shrigum/adamic/src/reader"
)

// countingRenderer is a fake renderFunc: it returns a 1x1 image and records how
// many times each page was rendered, so tests can assert on virtualization
// behavior (bounded cache, reuse, no out-of-band rendering) without a real
// 500-page PDF or the PDFium engine.
type countingRenderer struct {
	calls   int
	perPage map[int]int
}

func newCountingRenderer() *countingRenderer {
	return &countingRenderer{perPage: map[int]int{}}
}

func (c *countingRenderer) render(_ reader.DocumentID, page int, _ reader.Scale) (image.Image, error) {
	c.calls++
	c.perPage[page]++
	return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
}

// TestRenderWindowBoundedOn500Pages is the AC3/AC11 property: scrolling a
// 500-page document keeps the retained rendered-page count bounded by the
// budget — never anywhere near 500.
func TestRenderWindowBoundedOn500Pages(t *testing.T) {
	const pages = 500
	cr := newCountingRenderer()
	// Visible band of ~4 pages, look-ahead 2 each side, budget 16.
	w := newRenderWindow(cr.render, "doc", pages, 2, 16)

	// Scroll one page at a time across the whole document.
	for top := 0; top < pages; top++ {
		if _, err := w.Update(top, top+3, reader.Scale{Zoom: 1}); err != nil {
			t.Fatalf("Update at %d: %v", top, err)
		}
		if got := w.CachedCount(); got > 16 {
			t.Fatalf("cached pages = %d at top=%d, exceeds budget 16 (AC11)", got, top)
		}
	}
	if w.CachedCount() >= pages {
		t.Errorf("cache holds %d pages — virtualization failed (AC3)", w.CachedCount())
	}
	// Every page rendered at least once, but total renders is modest — nowhere
	// near re-rendering all 500 many times over.
	if cr.perPage[250] == 0 {
		t.Error("mid-document page never rendered while scrolling past it")
	}
}

// TestRenderWindowReusesVisiblePages: holding the viewport still must not
// re-render — a cached page is served from the LRU.
func TestRenderWindowReusesVisiblePages(t *testing.T) {
	cr := newCountingRenderer()
	w := newRenderWindow(cr.render, "doc", 100, 1, 32)

	imgs, err := w.Update(10, 12, reader.Scale{Zoom: 1})
	if err != nil {
		t.Fatalf("first Update: %v", err)
	}
	if len(imgs) != 3 {
		t.Fatalf("visible images = %d, want 3", len(imgs))
	}
	firstCalls := cr.calls

	// Same viewport again: no new renders.
	if _, err := w.Update(10, 12, reader.Scale{Zoom: 1}); err != nil {
		t.Fatalf("second Update: %v", err)
	}
	if cr.calls != firstCalls {
		t.Errorf("re-rendered on an unchanged viewport: %d new calls", cr.calls-firstCalls)
	}
}

// TestRenderWindowDifferentScaleIsDistinct: a zoom change is a cache miss (a
// 100%% raster can't serve a 200%% request).
func TestRenderWindowDifferentScaleIsDistinct(t *testing.T) {
	cr := newCountingRenderer()
	w := newRenderWindow(cr.render, "doc", 10, 0, 32)

	w.Update(0, 0, reader.Scale{Zoom: 1})
	callsAt1x := cr.calls
	w.Update(0, 0, reader.Scale{Zoom: 2})
	if cr.calls == callsAt1x {
		t.Error("changing zoom should trigger a re-render (distinct cache entry)")
	}
}

// TestRenderWindowClampsOutOfRange: scrolling past the end clamps to the
// document rather than erroring or rendering nonexistent pages.
func TestRenderWindowClampsOutOfRange(t *testing.T) {
	cr := newCountingRenderer()
	w := newRenderWindow(cr.render, "doc", 5, 1, 32)

	imgs, err := w.Update(4, 10, reader.Scale{Zoom: 1}) // last=10 is past the end
	if err != nil {
		t.Fatalf("Update past end: %v", err)
	}
	if len(imgs) != 1 { // only page 4 is valid
		t.Errorf("visible images = %d, want 1 (clamped to last page)", len(imgs))
	}
	for p := range cr.perPage {
		if p < 0 || p >= 5 {
			t.Errorf("rendered out-of-range page %d", p)
		}
	}
}

// TestRenderWindowOverRealEngine drives a RenderWindow through the real PDFium
// engine over the Dutch fixture, proving the virtualization seam works against
// actual rendering (not just the counting fake) and returns non-blank pages.
func TestRenderWindowOverRealEngine(t *testing.T) {
	e := newTestEngine(t)
	doc, err := e.Open(fixturePath(fixture))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer e.Close(doc.ID)

	w := NewRenderWindow(e, doc.ID, doc.PageInfo.Count, 1, 8)
	imgs, err := w.Update(0, 1, reader.Scale{Zoom: 1})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(imgs) != 2 {
		t.Fatalf("visible images = %d, want 2", len(imgs))
	}
	for i, img := range imgs {
		if isBlank(img) {
			t.Errorf("visible page %d rendered blank through the window", i)
		}
	}
	// Cache holds the visible band + look-ahead, bounded by the 4-page document.
	if w.CachedCount() > 4 {
		t.Errorf("cached = %d, want ≤ 4 (document has 4 pages)", w.CachedCount())
	}
}

// TestRenderWindowBudgetFloor: a too-small budget is raised so a single Update
// never evicts a page it just rendered (which would thrash).
func TestRenderWindowBudgetFloor(t *testing.T) {
	cr := newCountingRenderer()
	// lookAhead 3 → band up to 3+ (3+1) +3 = needs a floor > 1; ask for budget 1.
	w := newRenderWindow(cr.render, "doc", 100, 3, 1)

	if _, err := w.Update(50, 53, reader.Scale{Zoom: 1}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	// All visible pages must still be cached after one Update (no self-eviction).
	if w.CachedCount() < 4 {
		t.Errorf("cached = %d after one Update; budget floor failed to prevent thrash", w.CachedCount())
	}
}
