package ocr_test

import (
	"strings"
	"testing"

	"github.com/shrigum/adamic/src/document"
	"github.com/shrigum/adamic/src/library"
	"github.com/shrigum/adamic/src/ocr"
)

func TestNeedsOCR(t *testing.T) {
	longText := strings.Repeat("Er staat echte tekst op deze pagina. ", 5)
	tests := []struct {
		name     string
		text     string
		coverage float64
		want     bool
	}{
		{"image-only page is a candidate", "", 0.95, true},
		{"whitespace-only text layer is a candidate", " \n\t \n", 0.95, true},
		{"stray artifact text under a full-page image is a candidate", "p. 7", 1.0, true},
		{"born-digital text page is skipped", longText, 0, false},
		{"text page with a small figure is skipped", longText, 0.3, false},
		{"near-empty text without a dominant image is skipped", "", 0.2, false},
		{"real text under a dominant image is skipped", longText, 0.9, false},
		{
			"text at the threshold is no longer near-empty",
			strings.Repeat("a", ocr.MaxNativeTextRunes), 1.0, false,
		},
		{
			"text just under the threshold counts as near-empty",
			strings.Repeat("a", ocr.MaxNativeTextRunes-1), 1.0, true,
		},
		{"coverage at the floor qualifies", "", ocr.MinImageCoverage, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ocr.NeedsOCR(tt.text, tt.coverage); got != tt.want {
				t.Errorf("NeedsOCR(%d runes, %v) = %v, want %v", len(tt.text), tt.coverage, got, tt.want)
			}
		})
	}
}

// TestDetectionOnFixtures is AC3 end to end: the scanned, image-only fixture
// page is detected as an OCR candidate; the born-digital page with a real
// text layer is not (no OCR run happens anywhere in detection).
func TestDetectionOnFixtures(t *testing.T) {
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

	detect := func(path string) bool {
		t.Helper()
		doc, err := e.Open(path)
		if err != nil {
			t.Fatalf("Open %s: %v", path, err)
		}
		defer e.Close(doc.ID)
		c, err := e.PageContent(doc.ID, 0)
		if err != nil {
			t.Fatalf("PageContent %s: %v", path, err)
		}
		return ocr.NeedsOCR(c.Text, c.ImageCoverage)
	}

	if !detect(fixturePDF) {
		t.Error("scanned image-only fixture page not detected as an OCR candidate (AC3)")
	}
	if detect("../document/testdata/born-digital.pdf") {
		t.Error("born-digital text page detected as an OCR candidate; it must be skipped (AC3)")
	}
}
