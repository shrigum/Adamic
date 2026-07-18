// Package tesseract implements ocr.Recognizer by running the Tesseract OCR
// engine as a subprocess — the integration chosen by ADR-0014 and proven by
// the T2 spike (docs/planning/ocr/spike-t2-findings.md). The engine binding is
// confined to this package (design-review note; like PDFium in document):
// nothing else invokes the tesseract executable or reads its TSV output.
//
// Failure modes (docs/CODING_STANDARDS.md, "Own your failure modes"): all
// errors are soft engine-level conditions for the caller to normalize into the
// per-page ocr.PageFailure (AC8) — Find fails when no usable executable or
// language model is present (with what to do about it); RecognizePage fails
// when the subprocess cannot run or exits nonzero, when its output cannot be
// parsed, or when ctx is cancelled mid-recognition (the subprocess is killed;
// the error wraps ctx.Err()). No error leaves partial state behind: the only
// on-disk artifact is a temp image, removed on every path. A recognition
// producing units that violate the ocr contract is reported as a loud error,
// because that is a bug in this package, not a bad scan.
package tesseract

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shrigum/adamic/src/ocr"
	"github.com/shrigum/adamic/src/reader"
)

// EnvExecutable names the environment variable that pins the tesseract
// executable to use, taking precedence over PATH lookup — the same override
// the T2 spike honors, so a portable (or later, bundled) engine needs no
// system install. A tessdata directory is expected beside the executable
// (the standard Windows layout) unless TESSDATA_PREFIX overrides it.
const EnvExecutable = "ADAMIC_TESSERACT"

// Recognizer drives one tesseract executable + language model as a subprocess,
// one cold process per page (the spike's measured model: ~2 s/page at 300 DPI
// on the fixture). It satisfies ocr.Recognizer. The zero value is unusable;
// construct via Find. A Recognizer holds no mutable state and is safe for
// concurrent use.
type Recognizer struct {
	exe      string
	tessdata string
	lang     string
}

var _ ocr.Recognizer = (*Recognizer)(nil)

// Find locates a usable Tesseract engine for the given language (ISO 639-3 as
// Tesseract names its models, e.g. "nld"): the executable from EnvExecutable
// or PATH, the tessdata directory from TESSDATA_PREFIX or beside the
// executable, and the language's .traineddata file within it. It fails —
// telling the user what to install or set — if any piece is missing; it does
// not run the engine.
func Find(lang string) (*Recognizer, error) {
	exe := os.Getenv(EnvExecutable)
	if exe == "" {
		var err error
		exe, err = exec.LookPath("tesseract")
		if err != nil {
			return nil, fmt.Errorf("find OCR engine: tesseract not found; install it or set %s to the executable: %w", EnvExecutable, err)
		}
	}
	if _, err := os.Stat(exe); err != nil {
		return nil, fmt.Errorf("find OCR engine: %s is not usable; point %s at the tesseract executable: %w", exe, EnvExecutable, err)
	}
	tessdata := os.Getenv("TESSDATA_PREFIX")
	if tessdata == "" {
		tessdata = filepath.Join(filepath.Dir(exe), "tessdata")
	}
	model := filepath.Join(tessdata, lang+".traineddata")
	if _, err := os.Stat(model); err != nil {
		return nil, fmt.Errorf("find OCR model: %s language data not found; put %s.traineddata in %s (ADR-0014: tessdata_best): %w", lang, lang, tessdata, err)
	}
	return &Recognizer{exe: exe, tessdata: tessdata, lang: lang}, nil
}

// RecognizePage implements ocr.Recognizer: it writes img to a temp PNG, runs
// `tesseract <png> stdout -l <lang> --dpi <n> tsv` with the engine's tessdata,
// and maps the TSV word rows onto the ocr contract. The pixel→point transform
// lives here and only here (design-review note): pixel boxes are divided by
// the per-axis pixels-per-point ratio of img's dimensions to size, so the
// units are in page points regardless of the render scale. The --dpi passed to
// the engine is derived from the same ratio (the PNG carries no density).
func (r *Recognizer) RecognizePage(ctx context.Context, img image.Image, size reader.PageSize) ([]ocr.RecognizedUnit, error) {
	if r.exe == "" {
		return nil, fmt.Errorf("recognize page: Recognizer not constructed via Find")
	}
	pxW, pxH := img.Bounds().Dx(), img.Bounds().Dy()
	if pxW <= 0 || pxH <= 0 || size.WidthPt <= 0 || size.HeightPt <= 0 {
		return nil, fmt.Errorf("recognize page: degenerate input (%dx%d px image on %vx%v pt page)", pxW, pxH, size.WidthPt, size.HeightPt)
	}
	sx := float64(pxW) / size.WidthPt
	sy := float64(pxH) / size.HeightPt
	dpi := int(math.Round(72 * sx))

	pngPath, err := writeTempPNG(img)
	if err != nil {
		return nil, fmt.Errorf("recognize page: %w", err)
	}
	defer os.Remove(pngPath)

	cmd := exec.CommandContext(ctx, r.exe, pngPath, "stdout", "-l", r.lang, "--dpi", strconv.Itoa(dpi), "tsv")
	cmd.Env = append(os.Environ(), "TESSDATA_PREFIX="+r.tessdata)
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("recognize page: cancelled: %w", ctx.Err())
		}
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("recognize page: tesseract failed: %w (stderr: %s)", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("recognize page: run tesseract: %w", err)
	}

	units, err := parseTSVWords(string(out), sx, sy)
	if err != nil {
		return nil, fmt.Errorf("recognize page: %w", err)
	}
	for _, u := range units {
		if err := u.Validate(size); err != nil {
			return nil, fmt.Errorf("recognize page: bug: emitted unit violates the ocr contract: %w", err)
		}
	}
	return units, nil
}

// writeTempPNG writes img to a temp file and returns its path; the caller
// removes it. A file (not a pipe) keeps this on the exact subprocess path the
// T2 spike proved.
func writeTempPNG(img image.Image) (string, error) {
	f, err := os.CreateTemp("", "adamic-ocr-*.png")
	if err != nil {
		return "", fmt.Errorf("create temp page image: %w", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", fmt.Errorf("encode page image: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("write page image: %w", err)
	}
	return f.Name(), nil
}

// tsvFields is the fixed column count of Tesseract's TSV output:
// level, page_num, block_num, par_num, line_num, word_num,
// left, top, width, height, conf, text.
const tsvFields = 12

// parseTSVWords converts Tesseract TSV output into recognized units: level-5
// (word) rows only, pixel boxes scaled into page points by the per-axis
// pixels-per-point ratios sx and sy, confidence normalized from Tesseract's
// 0–100 to the contract's [0, 1]. The block/paragraph/line numbers become the
// opaque Group id (spec A2). Structural rows (page/block/para/line) and
// whitespace-only word rows are skipped; a malformed row is an error naming
// the row.
func parseTSVWords(tsv string, sx, sy float64) ([]ocr.RecognizedUnit, error) {
	var units []ocr.RecognizedUnit
	lines := strings.Split(tsv, "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue // header row, trailing newline
		}
		f := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(f) != tsvFields {
			return nil, fmt.Errorf("tesseract tsv row %d has %d fields, want %d: %q", i, len(f), tsvFields, line)
		}
		if f[0] != "5" {
			continue // structural rows carry no text
		}
		if strings.TrimSpace(f[11]) == "" {
			continue
		}
		var box [4]int
		for j, field := range f[6:10] {
			n, err := strconv.Atoi(field)
			if err != nil {
				return nil, fmt.Errorf("tesseract tsv row %d: bad box number %q: %w", i, field, err)
			}
			box[j] = n
		}
		conf, err := strconv.ParseFloat(f[10], 64)
		if err != nil {
			return nil, fmt.Errorf("tesseract tsv row %d: bad confidence %q: %w", i, f[10], err)
		}
		units = append(units, ocr.RecognizedUnit{
			Text: f[11],
			Box: ocr.Box{
				X: float64(box[0]) / sx,
				Y: float64(box[1]) / sy,
				W: float64(box[2]) / sx,
				H: float64(box[3]) / sy,
			},
			Confidence: conf / 100,
			Group:      fmt.Sprintf("b%s.p%s.l%s", f[2], f[3], f[4]),
		})
	}
	return units, nil
}
