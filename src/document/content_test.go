package document

import (
	"errors"
	"strings"
	"testing"

	"github.com/shrigum/adamic/src/reader"
)

func TestEnginePageContent(t *testing.T) {
	e := newTestEngine(t)

	t.Run("scanned page is near-empty text under a dominant image", func(t *testing.T) {
		doc, err := e.Open(fixturePath(fixture))
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		defer e.Close(doc.ID)
		c, err := e.PageContent(doc.ID, 0)
		if err != nil {
			t.Fatalf("PageContent: %v", err)
		}
		if got := len(strings.TrimSpace(c.Text)); got > 32 {
			t.Errorf("scanned page native text = %d chars (%q…), want near-empty", got, c.Text[:min(got, 40)])
		}
		if c.ImageCoverage < 0.5 {
			t.Errorf("scanned page ImageCoverage = %v, want a dominant image (>= 0.5)", c.ImageCoverage)
		}
		if c.ImageCoverage > 1 {
			t.Errorf("ImageCoverage = %v, want clamped to [0, 1]", c.ImageCoverage)
		}
	})

	t.Run("born-digital page carries its text layer and no image", func(t *testing.T) {
		doc, err := e.Open(fixturePath("born-digital.pdf"))
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		defer e.Close(doc.ID)
		c, err := e.PageContent(doc.ID, 0)
		if err != nil {
			t.Fatalf("PageContent: %v", err)
		}
		if !strings.Contains(c.Text, "tekstlaag") {
			t.Errorf("born-digital page text = %q, want the fixture's sentence in it", c.Text)
		}
		if c.ImageCoverage != 0 {
			t.Errorf("born-digital page ImageCoverage = %v, want 0 (no image objects)", c.ImageCoverage)
		}
	})

	t.Run("page out of range is the typed error", func(t *testing.T) {
		doc, err := e.Open(fixturePath(fixture))
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		defer e.Close(doc.ID)
		if _, err := e.PageContent(doc.ID, fixturePageCount); !errors.Is(err, reader.ErrPageOutOfRange) {
			t.Errorf("PageContent(out of range) error = %v, want ErrPageOutOfRange", err)
		}
	})

	t.Run("closed document is the typed error", func(t *testing.T) {
		if _, err := e.PageContent("no-such-doc", 0); !errors.Is(err, reader.ErrClosedDocument) {
			t.Errorf("PageContent(closed) error = %v, want ErrClosedDocument", err)
		}
	})
}
