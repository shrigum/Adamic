// Package cli's reader integration tests exercise the pdf-reader-core feature
// end-to-end through the frontend binding layer (package app) against the real
// PDFium engine, plus the cross-cutting AC12 no-network inspection. They
// complement the per-package unit tests by proving the full open → navigate →
// render → persist → reopen flow works as assembled (spec AC1–AC12,
// docs/planning/pdf-reader-core/spec.md).
package cli

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shrigum/adamic/src/app"
	"github.com/shrigum/adamic/src/document"
	"github.com/shrigum/adamic/src/library"
)

// fixtureAbs is the Dutch A1 sample, addressed from the repo root (tests run
// with CWD = tests/, the engine's own testdata lives beside its package).
func fixtureAbs(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "src", "document", "testdata", "taalcompleet-a1-sample.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// TestReaderEndToEnd drives the whole feature through package app exactly as the
// frontend will: open a real PDF, read page geometry, render a page and a
// thumbnail as data URLs, save a reading position, and confirm a fresh open
// restores it (AC1, AC6, AC7, AC8).
func TestReaderEndToEnd(t *testing.T) {
	t.Setenv(library.EnvConfigDir, t.TempDir()) // isolate the position store

	eng, err := document.NewEngine()
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	t.Cleanup(func() { eng.Shutdown() })
	a := app.New(eng)

	path := fixtureAbs(t)

	// Open (AC1): success with correct page count and geometry.
	res := a.Open(path)
	if !res.Ok {
		t.Fatalf("Open failed: %+v", res.Error)
	}
	if len(res.Doc.Pages) != 4 {
		t.Fatalf("page count = %d, want 4 (AC1)", len(res.Doc.Pages))
	}
	if res.Doc.Position.Page != 0 {
		t.Errorf("first open should start at page 1 (index 0), got %d (AC7)", res.Doc.Position.Page)
	}

	// Render page 1 and a thumbnail as data URLs (AC1, AC6).
	img, err := a.RenderPage(res.Doc.ID, 0, 1.0)
	if err != nil {
		t.Fatalf("RenderPage: %v", err)
	}
	if !strings.HasPrefix(img, "data:image/png;base64,") {
		t.Errorf("rendered page is not a data URL")
	}
	if _, err := a.Thumbnail(res.Doc.ID, 3); err != nil {
		t.Errorf("Thumbnail of last page: %v", err)
	}

	// Save a position, close, reopen: it restores (AC7, AC8 — the store read is
	// from disk, standing in for an app restart).
	if err := a.SetPosition(res.Doc.ID, 2, 0.5); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}
	a.Close(res.Doc.ID)

	res2 := a.Open(path)
	if !res2.Ok {
		t.Fatalf("reopen failed: %+v", res2.Error)
	}
	if res2.Doc.Position.Page != 2 || res2.Doc.Position.OffsetY != 0.5 {
		t.Errorf("restored position = %+v, want {2, 0.5} (AC7/AC8)", res2.Doc.Position)
	}
}

// TestReaderSoftErrorsEndToEnd confirms the bad-input cases surface as
// displayable soft errors through the binding, not crashes (AC9/AC10).
func TestReaderSoftErrorsEndToEnd(t *testing.T) {
	t.Setenv(library.EnvConfigDir, t.TempDir())
	eng, err := document.NewEngine()
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	t.Cleanup(func() { eng.Shutdown() })
	a := app.New(eng)

	cases := map[string]string{
		"not-found": filepath.Join("..", "src", "document", "testdata", "no-such.pdf"),
		"not-pdf":   filepath.Join("..", "src", "document", "testdata", "not-a-pdf.txt"),
		"corrupt":   filepath.Join("..", "src", "document", "testdata", "corrupt.pdf"),
	}
	for wantKind, p := range cases {
		res := a.Open(p)
		if res.Ok {
			t.Errorf("%s: Open reported success, want soft failure", wantKind)
			continue
		}
		if res.Error.Kind != wantKind {
			t.Errorf("%s: error kind = %q", wantKind, res.Error.Kind)
		}
		if res.Error.Message == "" {
			t.Errorf("%s: no user-facing message", wantKind)
		}
	}

	// The engine is still usable after the failures (AC9: app stays up).
	if res := a.Open(fixtureAbs(t)); !res.Ok {
		t.Errorf("engine unusable after soft errors: %+v", res.Error)
	}
}

// TestReaderNoNetworkImports is the AC12 inspection: no package in the
// pdf-reader-core feature may perform network I/O. We assert statically that
// none of them import net or net/http (or other net/* transport packages).
// The update package legitimately uses the network and is deliberately not part
// of this set.
func TestReaderNoNetworkImports(t *testing.T) {
	featurePkgs := map[string]string{
		"reader":   filepath.Join("..", "src", "reader"),
		"document": filepath.Join("..", "src", "document"),
		"library":  filepath.Join("..", "src", "library"),
		"app":      filepath.Join("..", "src", "app"),
	}
	banned := func(importPath string) bool {
		return importPath == "net" ||
			strings.HasPrefix(importPath, "net/") // net/http, net/url, ...
	}

	for name, dir := range featurePkgs {
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, dir, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, pkg := range pkgs {
			for filename, f := range pkg.Files {
				// Skip test files: httptest/net usage in a hypothetical test
				// would not ship in the feature. (Currently none exist.)
				if strings.HasSuffix(filename, "_test.go") {
					continue
				}
				for _, imp := range f.Imports {
					path := strings.Trim(imp.Path.Value, `"`)
					if banned(path) {
						t.Errorf("AC12 violation: %s imports %q (no network in the reader)", filepath.Base(filename), path)
					}
				}
			}
		}
	}
}
