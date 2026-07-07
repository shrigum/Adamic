package reader

import (
	"image"
	"image/color"
	"sync"
)

// StubReader is an in-memory Reader that renders synthetic pages, letting the
// frontend track (T6, the Wails shell) be built and demoed against the command
// contract before the real Document Engine (package document, T3+) exists. It
// opens any path as a fixed synthetic document, renders solid-colour placeholder
// pages sized from Scale, and keeps reading position in memory.
//
// It is not a fake for engine tests — the real engine has its own tests against
// real PDFs. Its only job is to make the boundary callable end to end.
type StubReader struct {
	mu        sync.Mutex
	pageCount int
	positions map[DocumentID]Position
	open      map[DocumentID]bool
}

// NewStubReader returns a StubReader whose synthetic documents have pageCount
// pages. It satisfies Reader.
func NewStubReader(pageCount int) *StubReader {
	if pageCount < 1 {
		pageCount = 1
	}
	return &StubReader{
		pageCount: pageCount,
		positions: map[DocumentID]Position{},
		open:      map[DocumentID]bool{},
	}
}

var _ Reader = (*StubReader)(nil)

// stubPagePt is the intrinsic size of every synthetic page (US Letter at 72 dpi).
const stubPageWidthPt, stubPageHeightPt = 612.0, 792.0

func (s *StubReader) Open(path string) (*Document, error) {
	id := DocumentID("stub:" + path)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.open[id] = true

	sizes := make([]PageSize, s.pageCount)
	for i := range sizes {
		sizes[i] = PageSize{WidthPt: stubPageWidthPt, HeightPt: stubPageHeightPt}
	}
	return &Document{
		ID:       id,
		Path:     path,
		PageInfo: PageInfo{Count: s.pageCount, Sizes: sizes},
		Position: s.positions[id], // zero value if never opened (spec AC7)
	}, nil
}

func (s *StubReader) PageCount(doc DocumentID) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.open[doc] {
		return 0, ErrClosedDocument
	}
	return s.pageCount, nil
}

func (s *StubReader) RenderPage(doc DocumentID, page int, scale Scale) (image.Image, error) {
	s.mu.Lock()
	open := s.open[doc]
	s.mu.Unlock()
	if !open {
		return nil, ErrClosedDocument
	}
	if page < 0 || page >= s.pageCount {
		return nil, ErrPageOutOfRange
	}
	w, h := scale.PixelSize(stubPageWidthPt, stubPageHeightPt)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// A distinct per-page shade so the frontend can see navigation working.
	shade := uint8(40 + (page*37)%180)
	fill := color.RGBA{R: shade, G: shade, B: 255 - shade, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, fill)
		}
	}
	return img, nil
}

func (s *StubReader) Thumbnail(doc DocumentID, page int) (image.Image, error) {
	return s.RenderPage(doc, page, ThumbnailScale())
}

func (s *StubReader) SetPosition(doc DocumentID, pos Position) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.open[doc] {
		return ErrClosedDocument
	}
	s.positions[doc] = pos
	return nil
}

func (s *StubReader) GetPosition(doc DocumentID) (Position, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.positions[doc], nil
}

func (s *StubReader) Close(doc DocumentID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.open, doc) // closing an unknown handle is a no-op
	return nil
}

// PixelSize resolves a Scale against a page's intrinsic point size, returning
// device pixels. It is part of the contract: both the stub and the real engine
// (package document) render at the size it returns, so fit-mode geometry is
// defined in exactly one place. Points are 1/72 inch; at Zoom 1.0 one point
// maps to one pixel (72 dpi baseline).
func (sc Scale) PixelSize(widthPt, heightPt float64) (int, int) {
	zoom := sc.Zoom
	switch {
	case sc.FitWidth && sc.Viewport.WidthPx > 0:
		zoom = float64(sc.Viewport.WidthPx) / widthPt
	case sc.FitPage && sc.Viewport.WidthPx > 0 && sc.Viewport.HeightPx > 0:
		zoom = min(
			float64(sc.Viewport.WidthPx)/widthPt,
			float64(sc.Viewport.HeightPx)/heightPt,
		)
	case zoom <= 0:
		zoom = 1.0 // zero Scale == 100%
	}
	w := int(widthPt * zoom)
	h := int(heightPt * zoom)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return w, h
}
