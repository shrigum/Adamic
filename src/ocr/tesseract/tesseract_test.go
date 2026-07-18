package tesseract

import (
	"context"
	"errors"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shrigum/adamic/src/document"
	"github.com/shrigum/adamic/src/library"
	"github.com/shrigum/adamic/src/ocr"
	"github.com/shrigum/adamic/src/reader"
)

const tsvHeader = "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext"

// row builds one TSV line from its fields.
func row(fields ...string) string { return strings.Join(fields, "\t") }

func TestParseTSVWords(t *testing.T) {
	tests := []struct {
		name    string
		tsv     string
		sx, sy  float64
		want    []ocr.RecognizedUnit
		wantErr string // substring of the expected error; "" means success
	}{
		{
			name: "word rows become units with the pixel to point transform",
			tsv: strings.Join([]string{
				tsvHeader,
				row("1", "1", "0", "0", "0", "0", "0", "0", "1000", "2000", "-1", ""),
				row("5", "1", "2", "1", "3", "1", "100", "200", "50", "20", "96.5", "Goedemorgen"),
			}, "\n") + "\n",
			sx: 2, sy: 4,
			want: []ocr.RecognizedUnit{{
				Text:       "Goedemorgen",
				Box:        ocr.Box{X: 50, Y: 50, W: 25, H: 5},
				Confidence: 0.965,
				Group:      "b2.p1.l3",
			}},
		},
		{
			name: "structural and whitespace rows are skipped",
			tsv: strings.Join([]string{
				tsvHeader,
				row("2", "1", "1", "0", "0", "0", "0", "0", "10", "10", "-1", ""),
				row("3", "1", "1", "1", "0", "0", "0", "0", "10", "10", "-1", ""),
				row("4", "1", "1", "1", "1", "0", "0", "0", "10", "10", "-1", ""),
				row("5", "1", "1", "1", "1", "1", "0", "0", "10", "10", "40", " "),
			}, "\n"),
			sx: 1, sy: 1,
			want: nil,
		},
		{
			name: "header only means zero units",
			tsv:  tsvHeader + "\n",
			sx:   1, sy: 1,
			want: nil,
		},
		{
			name: "crlf line endings are tolerated",
			tsv: tsvHeader + "\r\n" +
				row("5", "1", "1", "1", "1", "1", "10", "10", "10", "10", "50", "woord") + "\r\n",
			sx: 1, sy: 1,
			want: []ocr.RecognizedUnit{{
				Text:       "woord",
				Box:        ocr.Box{X: 10, Y: 10, W: 10, H: 10},
				Confidence: 0.5,
				Group:      "b1.p1.l1",
			}},
		},
		{
			name: "wrong field count is a loud error naming the row",
			tsv:  tsvHeader + "\n" + row("5", "1", "1"),
			sx:   1, sy: 1,
			wantErr: "row 1 has 3 fields",
		},
		{
			name: "unparseable box number is an error",
			tsv:  tsvHeader + "\n" + row("5", "1", "1", "1", "1", "1", "ten", "10", "10", "10", "50", "woord"),
			sx:   1, sy: 1,
			wantErr: "bad box number",
		},
		{
			name: "unparseable confidence is an error",
			tsv:  tsvHeader + "\n" + row("5", "1", "1", "1", "1", "1", "10", "10", "10", "10", "high", "woord"),
			sx:   1, sy: 1,
			wantErr: "bad confidence",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTSVWords(tt.tsv, tt.sx, tt.sy)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("parseTSVWords() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTSVWords() error = %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("parseTSVWords() = %d units, want %d: %+v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("unit %d = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFind(t *testing.T) {
	// fakeEngine lays out a plausible install: an executable file with a
	// tessdata directory holding the Dutch model beside it.
	fakeEngine := func(t *testing.T) (exe, tessdata string) {
		t.Helper()
		dir := t.TempDir()
		exe = filepath.Join(dir, "tesseract.exe")
		if err := os.WriteFile(exe, []byte("stub"), 0o755); err != nil {
			t.Fatal(err)
		}
		tessdata = filepath.Join(dir, "tessdata")
		if err := os.MkdirAll(tessdata, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tessdata, "nld.traineddata"), []byte("stub"), 0o644); err != nil {
			t.Fatal(err)
		}
		return exe, tessdata
	}

	t.Run("env override with model beside the executable", func(t *testing.T) {
		exe, tessdata := fakeEngine(t)
		t.Setenv(EnvExecutable, exe)
		t.Setenv("TESSDATA_PREFIX", "")
		r, err := Find("nld")
		if err != nil {
			t.Fatalf("Find() error = %v", err)
		}
		if r.exe != exe || r.tessdata != tessdata || r.lang != "nld" {
			t.Errorf("Find() = %+v, want exe %s, tessdata %s, lang nld", r, exe, tessdata)
		}
	})

	t.Run("TESSDATA_PREFIX overrides the beside-the-exe default", func(t *testing.T) {
		exe, _ := fakeEngine(t)
		elsewhere := t.TempDir()
		if err := os.WriteFile(filepath.Join(elsewhere, "nld.traineddata"), []byte("stub"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv(EnvExecutable, exe)
		t.Setenv("TESSDATA_PREFIX", elsewhere)
		r, err := Find("nld")
		if err != nil {
			t.Fatalf("Find() error = %v", err)
		}
		if r.tessdata != elsewhere {
			t.Errorf("Find() tessdata = %s, want %s", r.tessdata, elsewhere)
		}
	})

	t.Run("missing executable tells the user what to set", func(t *testing.T) {
		t.Setenv(EnvExecutable, filepath.Join(t.TempDir(), "absent.exe"))
		_, err := Find("nld")
		if err == nil || !strings.Contains(err.Error(), EnvExecutable) {
			t.Fatalf("Find() error = %v, want mentioning %s", err, EnvExecutable)
		}
	})

	t.Run("missing language model names the model and directory", func(t *testing.T) {
		exe, tessdata := fakeEngine(t)
		if err := os.Remove(filepath.Join(tessdata, "nld.traineddata")); err != nil {
			t.Fatal(err)
		}
		t.Setenv(EnvExecutable, exe)
		t.Setenv("TESSDATA_PREFIX", "")
		_, err := Find("nld")
		if err == nil || !strings.Contains(err.Error(), "nld.traineddata") {
			t.Fatalf("Find() error = %v, want mentioning nld.traineddata", err)
		}
	})
}

func TestRecognizePageRejectsBadUse(t *testing.T) {
	t.Run("zero value recognizer is a loud error", func(t *testing.T) {
		var r Recognizer
		_, err := r.RecognizePage(context.Background(), image.NewRGBA(image.Rect(0, 0, 10, 10)), reader.PageSize{WidthPt: 10, HeightPt: 10})
		if err == nil || !strings.Contains(err.Error(), "Find") {
			t.Fatalf("RecognizePage() error = %v, want construction error mentioning Find", err)
		}
	})
	t.Run("degenerate image or page size is rejected before running the engine", func(t *testing.T) {
		r := &Recognizer{exe: "unused", tessdata: "unused", lang: "nld"}
		_, err := r.RecognizePage(context.Background(), image.NewRGBA(image.Rect(0, 0, 0, 0)), reader.PageSize{WidthPt: 10, HeightPt: 10})
		if err == nil || !strings.Contains(err.Error(), "degenerate") {
			t.Fatalf("RecognizePage() error = %v, want degenerate-input error", err)
		}
		_, err = r.RecognizePage(context.Background(), image.NewRGBA(image.Rect(0, 0, 10, 10)), reader.PageSize{})
		if err == nil || !strings.Contains(err.Error(), "degenerate") {
			t.Fatalf("RecognizePage() error = %v, want degenerate-input error", err)
		}
	})
}

// recognitionDPI mirrors the T2 spike's measurement point (spec A7: ~300 DPI
// equivalent) for the real-engine tests below.
const recognitionDPI = 300

const fixturePDF = "../../document/testdata/taalcompleet-a1-sample.pdf"

// findEngine returns a real Recognizer or skips, like the T2 spike: set
// ADAMIC_TESSERACT or have tesseract + nld on PATH.
func findEngine(t *testing.T) *Recognizer {
	t.Helper()
	r, err := Find("nld")
	if err != nil {
		t.Skipf("no usable Tesseract engine, skipping real-engine test: %v", err)
	}
	return r
}

// renderFixturePage renders one page of the scanned Dutch fixture through the
// real Document Engine at recognition scale — the exact input path T5 will use.
func renderFixturePage(t *testing.T, page int) (image.Image, reader.PageSize) {
	t.Helper()
	t.Setenv(library.EnvConfigDir, t.TempDir())
	e, err := document.NewEngine()
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	t.Cleanup(func() {
		if err := e.Shutdown(); err != nil {
			t.Errorf("Shutdown: %v", err)
		}
	})
	doc, err := e.Open(fixturePDF)
	if err != nil {
		t.Fatalf("Open fixture: %v", err)
	}
	t.Cleanup(func() { e.Close(doc.ID) })
	img, err := e.RenderPage(doc.ID, page, reader.Scale{Zoom: float64(recognitionDPI) / 72.0})
	if err != nil {
		t.Fatalf("RenderPage: %v", err)
	}
	return img, doc.PageInfo.Sizes[page]
}

// TestRecognizePageDutchFixture drives the T4 seam end to end on the fixture:
// AC1 (expected Dutch words recognized) and AC2 (every unit satisfies the
// contract — RecognizePage guarantees it, asserted here through the public
// boundary anyway).
func TestRecognizePageDutchFixture(t *testing.T) {
	r := findEngine(t)
	img, size := renderFixturePage(t, 0)

	units, err := r.RecognizePage(context.Background(), img, size)
	if err != nil {
		t.Fatalf("RecognizePage: %v", err)
	}
	if len(units) == 0 {
		t.Fatal("no recognized units on a page full of text (AC1)")
	}
	for _, u := range units {
		if err := u.Validate(size); err != nil {
			t.Errorf("contract violation (AC2): %v", err)
		}
	}
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
}

// TestRecognizePageCancelled pins the cancellation contract T7 relies on: a
// done context aborts recognition with an error wrapping ctx.Err().
func TestRecognizePageCancelled(t *testing.T) {
	r := findEngine(t)
	img, size := renderFixturePage(t, 0)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.RecognizePage(ctx, img, size)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RecognizePage with cancelled ctx: error = %v, want wrapping context.Canceled", err)
	}
}
