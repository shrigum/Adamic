package document

import (
	"container/list"
	"fmt"
	"image"
	"sync"

	"github.com/shrigum/adamic/src/reader"
)

// RenderWindow is the virtualized rendering core (task T5): given which pages
// are currently visible, it renders that band plus a small look-ahead and
// nothing else, caching results in an LRU bounded by a fixed page budget. This
// is what keeps a 500-page document responsive — the number of rendered (and
// retained) pages stays bounded regardless of document length (spec AC3/AC11),
// instead of rendering all pages up front.
//
// It sits on top of the Engine's per-page RenderPage; it adds no PDFium
// knowledge of its own. The frontend drives it by reporting the visible range
// as the user scrolls; the window decides what to render, reuse, or evict.
//
// Concurrency: Update is safe to call from the UI goroutine repeatedly; it
// serializes on mu. Rendering is synchronous here — background/async prefetch
// is a later performance lever (its budget is set in T11), not a correctness
// requirement, and is deliberately left out to keep this core simple (the LRU
// bound is the property AC11 rests on).
type RenderWindow struct {
	render    renderFunc
	doc       reader.DocumentID
	pageCount int
	lookAhead int // pages to render on each side of the visible band
	budget    int // max rendered pages retained (LRU capacity)

	mu    sync.Mutex
	cache map[cacheKey]*list.Element
	lru   *list.List // front = most recently used
}

// renderFunc renders one page at a scale. It is exactly the shape of
// Engine.RenderPage; the window depends on this function, not on the Engine
// type, so the virtualization/eviction logic is unit-testable against a large
// synthetic document without a real 500-page PDF. This is a one-method seam
// (a func value), not an engine abstraction — condition C4 stands: there is no
// second engine and no interface hierarchy.
type renderFunc func(doc reader.DocumentID, page int, scale reader.Scale) (image.Image, error)

// cacheKey identifies a rendered page at a specific scale. A different zoom is a
// different cache entry (a page rendered at 100% can't serve a 200% request).
type cacheKey struct {
	page  int
	scale reader.Scale
}

type cacheEntry struct {
	key cacheKey
	img image.Image
}

// NewRenderWindow builds a window over one open document, rendering through the
// given Engine. lookAhead is how many off-screen pages on each side to
// pre-render (0 = only visible). budget is the LRU capacity in pages; it must
// be at least large enough to hold the visible band plus look-ahead on both
// sides, or Update would thrash — NewRenderWindow raises the budget to that
// floor if a smaller one is given.
func NewRenderWindow(e *Engine, doc reader.DocumentID, pageCount, lookAhead, budget int) *RenderWindow {
	return newRenderWindow(e.RenderPage, doc, pageCount, lookAhead, budget)
}

// newRenderWindow is NewRenderWindow with the render function injected directly,
// for tests that drive a large synthetic document.
func newRenderWindow(render renderFunc, doc reader.DocumentID, pageCount, lookAhead, budget int) *RenderWindow {
	if lookAhead < 0 {
		lookAhead = 0
	}
	// Floor: enough to hold a plausible visible band (assume up to lookAhead+1
	// visible) plus look-ahead each side, so a single Update never evicts a page
	// it just rendered.
	floor := 2*lookAhead + (lookAhead + 1)
	if budget < floor {
		budget = floor
	}
	return &RenderWindow{
		render:    render,
		doc:       doc,
		pageCount: pageCount,
		lookAhead: lookAhead,
		budget:    budget,
		cache:     map[cacheKey]*list.Element{},
		lru:       list.New(),
	}
}

// Update reports the currently visible page range [firstVisible, lastVisible]
// (inclusive, zero-based) at a given scale, and returns the rendered images for
// the visible band in page order. It renders any visible-or-look-ahead page not
// already cached, evicts least-recently-used pages beyond the budget, and never
// renders a page outside the look-ahead band. Out-of-range inputs are clamped
// to the document, so a caller that scrolls past the end still gets a valid
// (possibly empty) result rather than an error.
func (w *RenderWindow) Update(firstVisible, lastVisible int, scale reader.Scale) ([]image.Image, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	firstVisible = clamp(firstVisible, 0, w.pageCount-1)
	lastVisible = clamp(lastVisible, 0, w.pageCount-1)
	if lastVisible < firstVisible {
		lastVisible = firstVisible
	}

	// Render (or reuse) the full band: visible plus look-ahead on each side.
	bandStart := clamp(firstVisible-w.lookAhead, 0, w.pageCount-1)
	bandEnd := clamp(lastVisible+w.lookAhead, 0, w.pageCount-1)
	for p := bandStart; p <= bandEnd; p++ {
		if _, err := w.ensure(p, scale); err != nil {
			return nil, fmt.Errorf("render window page %d: %w", p, err)
		}
	}

	// Return just the visible pages, in order, touching them as most-recent.
	visible := make([]image.Image, 0, lastVisible-firstVisible+1)
	for p := firstVisible; p <= lastVisible; p++ {
		img, err := w.ensure(p, scale)
		if err != nil {
			return nil, err
		}
		visible = append(visible, img)
	}
	return visible, nil
}

// ensure returns the cached image for (page, scale), rendering and caching it on
// a miss, and marks it most-recently-used. Eviction runs after any insertion.
func (w *RenderWindow) ensure(page int, scale reader.Scale) (image.Image, error) {
	key := cacheKey{page: page, scale: scale}
	if el, ok := w.cache[key]; ok {
		w.lru.MoveToFront(el)
		return el.Value.(*cacheEntry).img, nil
	}
	img, err := w.render(w.doc, page, scale)
	if err != nil {
		return nil, err
	}
	el := w.lru.PushFront(&cacheEntry{key: key, img: img})
	w.cache[key] = el
	w.evict()
	return img, nil
}

// evict drops least-recently-used entries until the cache is within budget.
func (w *RenderWindow) evict() {
	for w.lru.Len() > w.budget {
		back := w.lru.Back()
		if back == nil {
			return
		}
		ent := w.lru.Remove(back).(*cacheEntry)
		delete(w.cache, ent.key)
	}
}

// CachedCount reports how many rendered pages are currently retained. It exists
// for the AC11/AC3 assertion that this stays bounded (≤ budget) on a large
// document — the whole point of virtualization.
func (w *RenderWindow) CachedCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lru.Len()
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
