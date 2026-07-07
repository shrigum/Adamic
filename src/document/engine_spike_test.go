package document

// This file is the T2 engine spike — the C1 gate from the design review
// (docs/planning/pdf-reader-core/design-review.md). It is deliberately a test,
// not throwaway main code, so `go test ./...` keeps proving the invariant it
// establishes: a real PDF page renders through go-pdfium's WebAssembly
// (purego, no-cgo) backend. If this test ever fails to compile or run on a
// supported platform, the no-cgo assumption in ADR-0012/ADR-0005 has broken and
// must be re-opened before more rendering code is written.
//
// It intentionally calls the go-pdfium binding directly (not through the T1
// command contract, which does not exist yet). Once T3/T4 wrap the engine
// behind that contract, this spike stays as the lowest-level proof that the
// backend works; higher-level tests exercise the contract.

import (
	"bytes"
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
)

// fixture is the trimmed Dutch A1 coursebook (4 real course-material pages,
// image-only — see testdata/). It is a scanned PDF, so it exercises faithful
// rasterization (REQ-1), which is exactly what this spike must prove.
const fixture = "taalcompleet-a1-sample.pdf"

const fixturePageCount = 4

func TestEngineSpike_RenderRealPage(t *testing.T) {
	pdfBytes, err := os.ReadFile(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	// Single-instance wasm pool: no cgo, no C toolchain, no separate PDFium
	// binary. This is the property C1 exists to verify.
	pool, err := webassembly.Init(webassembly.Config{MinIdle: 1, MaxIdle: 1, MaxTotal: 1})
	if err != nil {
		t.Fatalf("webassembly.Init (no-cgo backend unavailable?): %v", err)
	}
	defer func() {
		if err := pool.Close(); err != nil {
			t.Errorf("pool.Close: %v", err)
		}
	}()

	instance, err := pool.GetInstance(30 * time.Second)
	if err != nil {
		t.Fatalf("pool.GetInstance: %v", err)
	}
	defer func() {
		if err := instance.Close(); err != nil {
			t.Errorf("instance.Close: %v", err)
		}
	}()

	doc, err := instance.OpenDocument(&requests.OpenDocument{File: &pdfBytes})
	if err != nil {
		t.Fatalf("OpenDocument: %v", err)
	}
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})

	// --- Proof 1: page count is read from the real document. ---
	pc, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		t.Fatalf("FPDF_GetPageCount: %v", err)
	}
	if pc.PageCount != fixturePageCount {
		t.Errorf("page count = %d, want %d", pc.PageCount, fixturePageCount)
	}

	// --- Proof 2: page 1 rasterizes to a non-empty, correctly shaped image. ---
	render, err := instance.RenderPageInDPI(&requests.RenderPageInDPI{
		DPI: 150,
		Page: requests.Page{
			ByIndex: &requests.PageByIndex{Document: doc.Document, Index: 0},
		},
	})
	if err != nil {
		t.Fatalf("RenderPageInDPI: %v", err)
	}
	defer render.Cleanup()

	img := render.Result.Image
	if img == nil {
		t.Fatal("rendered image is nil")
	}
	b := img.Bounds()
	// The fixture is ~A4 portrait; at 150 DPI that is roughly 1240x1754 px.
	// Assert only the invariants (portrait, non-trivial size) so the test does
	// not become brittle across PDFium versions.
	if b.Dx() < 500 || b.Dy() < 500 {
		t.Errorf("rendered image too small: %dx%d", b.Dx(), b.Dy())
	}
	if b.Dy() <= b.Dx() {
		t.Errorf("expected portrait page, got %dx%d", b.Dx(), b.Dy())
	}
	if isBlank(img) {
		t.Error("rendered page is uniformly blank — engine produced no content")
	}
}

// isBlank reports whether every sampled pixel is identical, which for a real
// course page means rendering silently produced nothing. Sampling a grid keeps
// this O(1)-ish rather than scanning millions of pixels.
func isBlank(img image.Image) bool {
	b := img.Bounds()
	var first bytes.Buffer
	seen := false
	const steps = 20
	for i := 0; i <= steps; i++ {
		for j := 0; j <= steps; j++ {
			x := b.Min.X + (b.Dx()-1)*i/steps
			y := b.Min.Y + (b.Dy()-1)*j/steps
			r, g, bl, a := img.At(x, y).RGBA()
			var px bytes.Buffer
			px.Grow(8)
			px.WriteByte(byte(r >> 8))
			px.WriteByte(byte(g >> 8))
			px.WriteByte(byte(bl >> 8))
			px.WriteByte(byte(a >> 8))
			if !seen {
				first = px
				seen = true
				continue
			}
			if !bytes.Equal(first.Bytes(), px.Bytes()) {
				return false
			}
		}
	}
	return true
}
