package ocr_test

// This file is the T2 recognition spike — the C1 gate from the design review
// (docs/planning/ocr/design-review.md). Like the reader's engine spike
// (src/document/engine_spike_test.go) it is deliberately a test, not throwaway
// code, so it keeps proving the invariant it establishes: the Tesseract
// integration chosen by ADR-0014 recognizes a real scanned Dutch page,
// offline, into text + page-point boxes that satisfy the T1 contract.
//
// It exercises the *subprocess* integration (tesseract CLI, TSV output) — the
// path that keeps our Go code cgo-free. It is not the T4 recognizer; T4 wraps
// what this spike proves behind the Recognizer seam. Measurements (latency,
// bundle size, per-platform pick) are recorded in
// docs/planning/ocr/spike-t2-findings.md.
//
// The test needs a Tesseract with the Dutch model and skips when none is
// found: set ADAMIC_TESSERACT to the tesseract executable (with a tessdata/
// directory beside it, the standard Windows layout), or have `tesseract` on
// PATH with nld installed. If this test ever *fails* (rather than skips) on a
// supported platform, the ADR-0014 baseline has broken and the ADR re-opens
// before more recognition code is written.

import (
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shrigum/adamic/src/document"
	"github.com/shrigum/adamic/src/library"
	"github.com/shrigum/adamic/src/ocr"
	"github.com/shrigum/adamic/src/reader"
)

// spikeDPI is the recognition render resolution the spike measures at (spec
// A7: ~300 DPI equivalent, independent of on-screen zoom). The T4 recognizer
// re-derives its own scale; this constant is the spike's measurement point.
const spikeDPI = 300

const fixturePDF = "../document/testdata/taalcompleet-a1-sample.pdf"

// findTesseract locates the tesseract executable and its tessdata directory,
// or skips the test. ADAMIC_TESSERACT takes precedence over PATH so the spike
// can run against a portable, non-installed copy.
func findTesseract(t *testing.T) (exe string, tessdata string) {
	t.Helper()
	if exe = os.Getenv("ADAMIC_TESSERACT"); exe == "" {
		var err error
		exe, err = exec.LookPath("tesseract")
		if err != nil {
			t.Skip("tesseract not found: set ADAMIC_TESSERACT or add it to PATH (spike T2 runs where the engine is present)")
		}
	}
	if tessdata = os.Getenv("TESSDATA_PREFIX"); tessdata == "" {
		tessdata = filepath.Join(filepath.Dir(exe), "tessdata")
	}
	if _, err := os.Stat(filepath.Join(tessdata, "nld.traineddata")); err != nil {
		t.Skipf("Dutch model nld.traineddata not found in %s (ADR-0014: tessdata_best nld)", tessdata)
	}
	return exe, tessdata
}

func TestOCRSpike_RecognizeDutchFixturePage(t *testing.T) {
	exe, tessdata := findTesseract(t)

	// Render page 1 of the scanned fixture through the real Document Engine at
	// a recognition-appropriate scale (spec A7) — the exact input path the T4
	// recognizer will use.
	t.Setenv(library.EnvConfigDir, t.TempDir())
	e, err := document.NewEngine()
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	defer func() {
		if err := e.Shutdown(); err != nil {
			t.Errorf("Shutdown: %v", err)
		}
	}()
	doc, err := e.Open(fixturePDF)
	if err != nil {
		t.Fatalf("Open fixture: %v", err)
	}
	defer e.Close(doc.ID)

	img, err := e.RenderPage(doc.ID, 0, reader.Scale{Zoom: float64(spikeDPI) / 72.0})
	if err != nil {
		t.Fatalf("RenderPage: %v", err)
	}
	pageSize := doc.PageInfo.Sizes[0]

	pngPath := filepath.Join(t.TempDir(), "page1.png")
	f, err := os.Create(pngPath)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("encode png: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close png: %v", err)
	}

	// Recognize via the subprocess integration: TSV on stdout, Dutch model,
	// explicit DPI (the PNG carries none). Timed: this is the per-page latency
	// measurement T15 turns into a budget.
	cmd := exec.Command(exe, pngPath, "stdout", "-l", "nld", "--dpi", strconv.Itoa(spikeDPI), "tsv")
	cmd.Env = append(os.Environ(), "TESSDATA_PREFIX="+tessdata)
	start := time.Now()
	out, err := cmd.Output()
	elapsed := time.Since(start)
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("tesseract failed: %v\nstderr: %s", err, ee.Stderr)
		}
		t.Fatalf("run tesseract: %v", err)
	}
	t.Logf("recognized page 1 in %v (render %dx%d px, %s)", elapsed, img.Bounds().Dx(), img.Bounds().Dy(), exe)

	// Map TSV word rows onto the T1 contract with the pixel→point transform
	// derived from the render scale (design-review note: one place only).
	pxPerPt := float64(img.Bounds().Dx()) / pageSize.WidthPt
	units := parseTSVWords(t, string(out), pxPerPt)
	if len(units) == 0 {
		t.Fatal("no recognized units on a page full of text (AC1)")
	}

	// Proof 1 (AC2/T1): every unit satisfies the contract — non-empty text,
	// confidence in [0,1], positive box inside the page bounds.
	for _, u := range units {
		if err := u.Validate(pageSize); err != nil {
			t.Errorf("contract violation: %v", err)
		}
	}

	// Proof 2 (AC1): known words from the fixture's page 1 are recognized.
	// Case-normalized exact word match against the page's exercise headings and
	// vocabulary.
	want := []string{"welk", "woord", "nederlands", "goedemorgen", "mevrouw", "meneer", "luister"}
	got := map[string]bool{}
	for _, u := range units {
		got[strings.ToLower(strings.Trim(u.Text, ".,!?*"))] = true
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("expected word %q not recognized on page 1 (AC1)", w)
		}
	}

	// Proof 3: confidence is meaningful — the page is clean print, so the
	// median unit confidence should be well above chance. Guards against a
	// misconfigured model silently returning garbage with low confidence.
	high := 0
	for _, u := range units {
		if u.Confidence >= 0.7 {
			high++
		}
	}
	if high*2 < len(units) {
		t.Errorf("only %d/%d units have confidence >= 0.7 — model or input misconfigured?", high, len(units))
	}
}

// parseTSVWords converts Tesseract TSV output into RecognizedUnits: level-5
// (word) rows only, pixel boxes divided by pxPerPt into page points,
// confidence normalized from Tesseract's 0–100 to the contract's [0,1]. The
// block/paragraph/line numbers become the opaque Group id (spec A2).
func parseTSVWords(t *testing.T, tsv string, pxPerPt float64) []ocr.RecognizedUnit {
	t.Helper()
	var units []ocr.RecognizedUnit
	lines := strings.Split(tsv, "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue // header
		}
		f := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(f) != 12 {
			t.Fatalf("TSV row %d has %d fields, want 12: %q", i, len(f), line)
		}
		if f[0] != "5" {
			continue // page/block/para/line rows carry no text
		}
		text := f[11]
		if strings.TrimSpace(text) == "" {
			continue
		}
		left, err1 := strconv.Atoi(f[6])
		top, err2 := strconv.Atoi(f[7])
		width, err3 := strconv.Atoi(f[8])
		height, err4 := strconv.Atoi(f[9])
		conf, err5 := strconv.ParseFloat(f[10], 64)
		for _, err := range []error{err1, err2, err3, err4, err5} {
			if err != nil {
				t.Fatalf("TSV row %d: bad number: %v (%q)", i, err, line)
			}
		}
		units = append(units, ocr.RecognizedUnit{
			Text: text,
			Box: ocr.Box{
				X: float64(left) / pxPerPt,
				Y: float64(top) / pxPerPt,
				W: float64(width) / pxPerPt,
				H: float64(height) / pxPerPt,
			},
			Confidence: conf / 100,
			Group:      fmt.Sprintf("b%s.p%s.l%s", f[2], f[3], f[4]),
		})
	}
	return units
}
