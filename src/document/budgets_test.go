package document

import (
	"testing"
	"time"

	"github.com/shrigum/adamic/src/reader"
)

// TestPerfBudgets is the AC11 gate: opening, rendering, and scrolling stay
// within the committed budgets (budgets.go). It asserts against the budget
// constants, not hard-coded times, so tightening a budget is a one-line change
// that this test enforces. The engine is constructed before timing starts so
// one-time wasm warmup is not charged to the per-operation budgets.
func TestPerfBudgets(t *testing.T) {
	e := newTestEngine(t) // constructed (warmed up) before any measurement

	// --- Open budget ---
	start := time.Now()
	doc, err := e.Open(fixturePath(fixture))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if d := time.Since(start); d > OpenBudget {
		t.Errorf("Open took %v, over budget %v (AC11)", d, OpenBudget)
	}
	defer e.Close(doc.ID)

	// --- Render budget (worst of the pages, to avoid a lucky-page pass) ---
	var worstRender time.Duration
	for p := 0; p < doc.PageInfo.Count; p++ {
		start = time.Now()
		if _, err := e.RenderPage(doc.ID, p, reader.Scale{Zoom: 1}); err != nil {
			t.Fatalf("RenderPage(%d): %v", p, err)
		}
		if d := time.Since(start); d > worstRender {
			worstRender = d
		}
	}
	if worstRender > RenderBudget {
		t.Errorf("worst page render took %v, over budget %v (AC11)", worstRender, RenderBudget)
	}

	// --- Scroll budget (per-page, cold cache each step) ---
	w := NewRenderWindow(e, doc.ID, doc.PageInfo.Count, 1, 8)
	var worstScroll time.Duration
	for top := 0; top < doc.PageInfo.Count; top++ {
		start = time.Now()
		if _, err := w.Update(top, top, reader.Scale{Zoom: 1}); err != nil {
			t.Fatalf("scroll Update(%d): %v", top, err)
		}
		if d := time.Since(start); d > worstScroll {
			worstScroll = d
		}
	}
	if worstScroll > ScrollBudget {
		t.Errorf("worst per-page scroll took %v, over budget %v (AC11)", worstScroll, ScrollBudget)
	}
}
