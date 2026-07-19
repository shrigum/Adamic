package store

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/shrigum/adamic/src/library"
	"github.com/shrigum/adamic/src/ocr"
)

func useTempStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(library.EnvConfigDir, dir)
	return dir
}

// sampleResult builds a small but shape-complete result: a recognized page
// with a grouped unit and a failed page, so the round-trip covers every
// contract field including the failure pointer.
func sampleResult(id library.DocID) ocr.Result {
	return ocr.Result{
		ID: id,
		Pages: []ocr.PageResult{
			{
				Page: 0,
				Units: []ocr.RecognizedUnit{
					{Text: "Goedemorgen", Box: ocr.Box{X: 10, Y: 20, W: 80, H: 12}, Confidence: 0.96, Group: "b1.p1.l1"},
					{Text: "docent", Box: ocr.Box{X: 10, Y: 40, W: 40, H: 12}, Confidence: 0.93},
				},
			},
			{
				Page:    2,
				Failure: &ocr.PageFailure{Kind: ocr.FailureUnreadable, Message: "this page's image could not be rendered"},
			},
		},
		Corrections: []ocr.Correction{{Page: 0, Unit: 1, Text: "Dag"}},
	}
}

func TestFileStoreRoundTrip(t *testing.T) {
	useTempStore(t)
	want := sampleResult("doc-a")

	if err := (FileStore{}).Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, ok, err := FileStore{}.Load("doc-a")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !ok {
		t.Fatal("Load ok = false after Save")
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round-trip mismatch:\ngot  %+v\nwant %+v", got, want)
	}
}

func TestFileStoreLoadMissingIsNotAnError(t *testing.T) {
	useTempStore(t)
	_, ok, err := FileStore{}.Load("never-saved")
	if err != nil {
		t.Fatalf("Load(missing) error = %v, want nil (no OCR yet is soft)", err)
	}
	if ok {
		t.Error("Load(missing) ok = true, want false")
	}
}

func TestFileStoreSaveReplacesPreviousResult(t *testing.T) {
	useTempStore(t)
	first := sampleResult("doc-a")
	if err := (FileStore{}).Save(first); err != nil {
		t.Fatalf("Save first: %v", err)
	}
	second := ocr.Result{ID: "doc-a", Pages: []ocr.PageResult{{
		Page:  1,
		Units: []ocr.RecognizedUnit{{Text: "nieuw", Box: ocr.Box{X: 1, Y: 1, W: 5, H: 5}, Confidence: 0.5}},
	}}}
	if err := (FileStore{}).Save(second); err != nil {
		t.Fatalf("Save second: %v", err)
	}
	got, ok, err := FileStore{}.Load("doc-a")
	if err != nil || !ok {
		t.Fatalf("Load: ok=%v err=%v", ok, err)
	}
	if !reflect.DeepEqual(got, second) {
		t.Errorf("Load after re-save = %+v, want the replacement %+v", got, second)
	}
}

func TestFileStoreDocumentsAreIsolated(t *testing.T) {
	useTempStore(t)
	a := sampleResult("doc-a")
	b := sampleResult("doc-b")
	if err := (FileStore{}).Save(a); err != nil {
		t.Fatalf("Save a: %v", err)
	}
	if err := (FileStore{}).Save(b); err != nil {
		t.Fatalf("Save b: %v", err)
	}
	got, ok, err := FileStore{}.Load("doc-a")
	if err != nil || !ok {
		t.Fatalf("Load a: ok=%v err=%v", ok, err)
	}
	if got.ID != "doc-a" {
		t.Errorf("Load(doc-a).ID = %q — results collided across documents", got.ID)
	}
}

func TestFileStoreCorruptFileIsALoudNamedError(t *testing.T) {
	useTempStore(t)
	path, err := (FileStore{}).Path("doc-a")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err = FileStore{}.Load("doc-a")
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("Load(corrupt) error = %v, want naming %s", err, path)
	}
	if !strings.Contains(err.Error(), "re-run OCR") {
		t.Errorf("Load(corrupt) error = %v, want telling the user what to do", err)
	}
}

func TestFileStoreNewerSchemaIsRefused(t *testing.T) {
	useTempStore(t)
	path, err := (FileStore{}).Path("doc-a")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	future := `{"schemaVersion": 999, "result": {"id": "doc-a", "pages": []}}`
	if err := os.WriteFile(path, []byte(future), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err = FileStore{}.Load("doc-a")
	if err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("Load(future schema) error = %v, want a refusal naming the newer version", err)
	}
}

func TestFileStoreMismatchedDocumentIsRefused(t *testing.T) {
	useTempStore(t)
	path, err := (FileStore{}).Path("doc-a")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	other := `{"schemaVersion": 1, "result": {"id": "doc-b", "pages": []}}`
	if err := os.WriteFile(path, []byte(other), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err = FileStore{}.Load("doc-a")
	if err == nil || !strings.Contains(err.Error(), "different document") {
		t.Fatalf("Load(mismatched id) error = %v, want a refusal", err)
	}
}

func TestFileStoreEmptyIdentityIsRejected(t *testing.T) {
	useTempStore(t)
	if err := (FileStore{}).Save(ocr.Result{}); err == nil {
		t.Error("Save with empty ID succeeded, want an error")
	}
	if _, _, err := (FileStore{}).Load(""); err == nil {
		t.Error("Load with empty ID succeeded, want an error")
	}
}

func TestFileStoreLeavesNoTempFiles(t *testing.T) {
	dir := useTempStore(t)
	if err := (FileStore{}).Save(sampleResult("doc-a")); err != nil {
		t.Fatalf("Save: %v", err)
	}
	var leftovers []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err == nil && strings.Contains(d.Name(), ".tmp-") {
			leftovers = append(leftovers, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) > 0 {
		t.Errorf("leftover temp files after Save: %v", leftovers)
	}
}
