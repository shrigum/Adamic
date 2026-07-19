package document

import (
	"fmt"

	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/requests"

	"github.com/shrigum/adamic/src/reader"
)

// PageContent summarizes what one page natively carries — the input to the
// needs-OCR detection heuristic (ocr feature, spec A3/AC3) and, later, to
// REQ-2's text mapping. It is inspection only: computing it renders nothing
// and runs no OCR.
type PageContent struct {
	// Text is the page's embedded text layer as PDFium extracts it. A scanned
	// page typically yields an empty or near-empty string.
	Text string

	// ImageCoverage is the fraction of the page area covered by image
	// objects, in [0, 1]. Overlapping images are summed then clamped, so it
	// is an upper-bound heuristic, not exact ink coverage: 1.0 means "at
	// least the whole page could be image".
	ImageCoverage float64
}

// PageContent inspects one page of an open document: its native text layer
// and how much of the page's area image objects cover. page is zero-based;
// out of range is reader.ErrPageOutOfRange, a closed handle is
// reader.ErrClosedDocument, and any PDFium failure is a wrapped error — the
// caller treats them all as soft (a page that cannot be inspected is simply
// not an OCR candidate).
func (e *Engine) PageContent(id reader.DocumentID, page int) (PageContent, error) {
	ref, err := e.ref(id)
	if err != nil {
		return PageContent{}, err
	}
	pc, err := e.instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: ref})
	if err != nil {
		return PageContent{}, fmt.Errorf("page count for content: %w", err)
	}
	if page < 0 || page >= pc.PageCount {
		return PageContent{}, reader.ErrPageOutOfRange
	}
	byIndex := requests.Page{ByIndex: &requests.PageByIndex{Document: ref, Index: page}}

	text, err := e.instance.GetPageText(&requests.GetPageText{Page: byIndex})
	if err != nil {
		return PageContent{}, fmt.Errorf("extract page %d text: %w", page, err)
	}

	sz, err := e.instance.GetPageSize(&requests.GetPageSize{Page: byIndex})
	if err != nil {
		return PageContent{}, fmt.Errorf("page %d size: %w", page, err)
	}
	coverage, err := e.imageCoverage(byIndex, page, sz.Width, sz.Height)
	if err != nil {
		return PageContent{}, err
	}
	return PageContent{Text: text.Text, ImageCoverage: coverage}, nil
}

// imageCoverage sums the page-area fraction covered by the page's image
// objects, each clipped to the page rectangle, clamped to 1.
func (e *Engine) imageCoverage(byIndex requests.Page, page int, widthPt, heightPt float64) (float64, error) {
	if widthPt <= 0 || heightPt <= 0 {
		return 0, nil
	}
	count, err := e.instance.FPDFPage_CountObjects(&requests.FPDFPage_CountObjects{Page: byIndex})
	if err != nil {
		return 0, fmt.Errorf("count page %d objects: %w", page, err)
	}
	area := 0.0
	for i := 0; i < count.Count; i++ {
		obj, err := e.instance.FPDFPage_GetObject(&requests.FPDFPage_GetObject{Page: byIndex, Index: i})
		if err != nil {
			return 0, fmt.Errorf("page %d object %d: %w", page, i, err)
		}
		typ, err := e.instance.FPDFPageObj_GetType(&requests.FPDFPageObj_GetType{PageObject: obj.PageObject})
		if err != nil {
			return 0, fmt.Errorf("page %d object %d type: %w", page, i, err)
		}
		if typ.Type != enums.FPDF_PAGEOBJ_IMAGE {
			continue
		}
		b, err := e.instance.FPDFPageObj_GetBounds(&requests.FPDFPageObj_GetBounds{PageObject: obj.PageObject})
		if err != nil {
			return 0, fmt.Errorf("page %d object %d bounds: %w", page, i, err)
		}
		w := min(float64(b.Right), widthPt) - max(float64(b.Left), 0)
		h := min(float64(b.Top), heightPt) - max(float64(b.Bottom), 0)
		if w > 0 && h > 0 {
			area += w * h
		}
	}
	return min(area/(widthPt*heightPt), 1.0), nil
}
