package document

import "time"

// Performance budgets (task T11, spec AC11 / NFR-PERF-03). These are ceilings
// the acceptance tests assert against so a performance regression fails CI —
// not targets, and not tuned to any one machine. They were set from real
// measurements on the image-only Dutch A1 fixture (heavy full-page JPEG scans,
// a realistic worst case for rasterization), then given generous headroom so a
// slower developer machine does not flake the suite:
//
//	measurement (this dev box)      budget (ceiling)   headroom
//	open+parse+geometry (4pg)  ~2ms        OpenBudget   250ms    ~100x
//	render one page @100%      ~57ms       RenderBudget 400ms    ~7x
//	scroll, per page (window)  ~47ms       ScrollBudget 400ms    ~8x
//
// The render/scroll headroom is smaller than open's on purpose: rasterization
// is the real cost and the number we most want to catch regressing, but 400ms
// still comfortably clears a slow CI runner. Revisit these if the engine
// backend changes (e.g. cgo) or the fixture is replaced; they are the numbers
// AC11 is measured against, so a change here is an AC-relevant change.
const (
	// OpenBudget bounds opening a document: read the bytes, parse, and read
	// page geometry — i.e. everything Engine.Open does except first render.
	// (Engine construction / wasm warmup is a one-time process cost measured
	// separately, not per-open.)
	OpenBudget = 250 * time.Millisecond

	// RenderBudget bounds rendering a single page at a normal reading scale.
	RenderBudget = 400 * time.Millisecond

	// ScrollBudget bounds the per-page cost of advancing the render window by
	// one page during a continuous scroll (spec AC11, the 500-page case).
	ScrollBudget = 400 * time.Millisecond
)
