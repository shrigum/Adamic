package ocr

import "unicode"

// Detection thresholds for "this page needs OCR" (spec A3; task T3). Both are
// deliberately conservative — mis-detecting a born-digital page as an OCR
// candidate wastes an expensive run and risks a worse text layer than the
// native one, so a page must look near-empty of text AND dominated by an
// image before it qualifies. A user-facing force-OCR override for pages with
// a poor-but-present native layer is spec open question 6, not built here.
const (
	// MaxNativeTextRunes is the exclusive upper bound of non-whitespace runes
	// a page's native text layer may hold and still count as "(near-)empty".
	// Scanned pages sometimes carry a stray artifact or page number; 32 runes
	// is well under one sentence of real content.
	MaxNativeTextRunes = 32

	// MinImageCoverage is the minimum fraction of the page area image objects
	// must cover for the page to count as image-dominated (a scanned page is
	// one full-page image; a mostly-text page with a small figure is not).
	MinImageCoverage = 0.5
)

// NeedsOCR reports whether a page is an OCR candidate (spec A3, AC3): its
// native text layer is (near-)empty and image objects dominate its area.
// nativeText is the page's embedded text as the Document Engine extracts it
// (document.PageContent); imageCoverage is that page's image-area fraction in
// [0, 1]. Detection is per page, so mixed documents work; it inspects only,
// never runs recognition.
func NeedsOCR(nativeText string, imageCoverage float64) bool {
	runes := 0
	for _, r := range nativeText {
		if unicode.IsSpace(r) {
			continue
		}
		runes++
		if runes >= MaxNativeTextRunes {
			return false
		}
	}
	return imageCoverage >= MinImageCoverage
}
