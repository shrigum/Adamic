package reader

import (
	"errors"
	"testing"
)

func TestScalePixelSize(t *testing.T) {
	const wPt, hPt = 612.0, 792.0 // US Letter
	tests := []struct {
		name  string
		scale Scale
		wantW int
		wantH int
	}{
		{"zero scale is 100%", Scale{}, 612, 792},
		{"explicit zoom 2x", Scale{Zoom: 2.0}, 1224, 1584},
		{"explicit zoom half", Scale{Zoom: 0.5}, 306, 396},
		{
			"fit-width fills viewport width",
			Scale{FitWidth: true, Viewport: Viewport{WidthPx: 1224, HeightPx: 100}},
			1224, 1584, // width doubled → height doubles too (aspect kept)
		},
		{
			"fit-page bounds by the tighter dimension",
			// viewport 1224 wide (would be 2x) but only 792 tall (1x): height binds.
			Scale{FitPage: true, Viewport: Viewport{WidthPx: 1224, HeightPx: 792}},
			612, 792,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, h := tt.scale.PixelSize(wPt, hPt)
			if w != tt.wantW || h != tt.wantH {
				t.Errorf("pixelSize() = %dx%d, want %dx%d", w, h, tt.wantW, tt.wantH)
			}
		})
	}
}

func TestScalePixelSizeNeverZero(t *testing.T) {
	// A degenerate zoom must still yield a drawable 1x1, never a zero-size image.
	w, h := Scale{Zoom: 0.00001}.PixelSize(612, 792)
	if w < 1 || h < 1 {
		t.Errorf("pixelSize() = %dx%d, want at least 1x1", w, h)
	}
}

func TestStubReaderRoundTrip(t *testing.T) {
	r := NewStubReader(10)

	doc, err := r.Open("/some/book.pdf")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if doc.PageInfo.Count != 10 {
		t.Errorf("page count = %d, want 10", doc.PageInfo.Count)
	}
	if len(doc.PageInfo.Sizes) != 10 {
		t.Errorf("len(Sizes) = %d, want 10 (one per page)", len(doc.PageInfo.Sizes))
	}
	if (doc.Position != Position{}) {
		t.Errorf("never-opened document should restore to zero position, got %+v (spec AC7)", doc.Position)
	}

	// A page renders at the requested scale.
	img, err := r.RenderPage(doc.ID, 0, Scale{Zoom: 1})
	if err != nil {
		t.Fatalf("RenderPage: %v", err)
	}
	if img.Bounds().Dx() != 612 {
		t.Errorf("page width = %d, want 612 at zoom 1", img.Bounds().Dx())
	}

	// Position persists within the session and comes back on reopen.
	want := Position{Page: 4, OffsetY: 0.5}
	if err := r.SetPosition(doc.ID, want); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}
	reopened, err := r.Open("/some/book.pdf")
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if reopened.Position != want {
		t.Errorf("restored position = %+v, want %+v (spec AC7)", reopened.Position, want)
	}
}

func TestStubReaderErrorPaths(t *testing.T) {
	r := NewStubReader(3)
	doc, _ := r.Open("/x.pdf")

	if _, err := r.RenderPage(doc.ID, 3, Scale{}); !errors.Is(err, ErrPageOutOfRange) {
		t.Errorf("render page 3 of 3 (0-based): want ErrPageOutOfRange, got %v (spec AC5)", err)
	}
	if _, err := r.RenderPage(doc.ID, -1, Scale{}); !errors.Is(err, ErrPageOutOfRange) {
		t.Errorf("render page -1: want ErrPageOutOfRange, got %v", err)
	}

	// Commands on a closed handle fail loudly rather than panicking.
	if err := r.Close(doc.ID); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := r.PageCount(doc.ID); !errors.Is(err, ErrClosedDocument) {
		t.Errorf("PageCount after Close: want ErrClosedDocument, got %v", err)
	}
	if _, err := r.RenderPage(doc.ID, 0, Scale{}); !errors.Is(err, ErrClosedDocument) {
		t.Errorf("RenderPage after Close: want ErrClosedDocument, got %v", err)
	}
	// Closing an already-closed handle is a no-op.
	if err := r.Close(doc.ID); err != nil {
		t.Errorf("double Close should be a no-op, got %v", err)
	}
}

func TestOpenErrorClassifies(t *testing.T) {
	err := error(&OpenError{Path: "/x", Kind: OpenPasswordReqd})
	var oe *OpenError
	if !errors.As(err, &oe) {
		t.Fatal("OpenError should be matchable with errors.As")
	}
	if oe.Kind != OpenPasswordReqd {
		t.Errorf("Kind = %v, want OpenPasswordReqd", oe.Kind)
	}
	if oe.Error() == "" {
		t.Error("OpenError.Error() should be non-empty")
	}
}
