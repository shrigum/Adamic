package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// useTempStore points the store at a throwaway dir so tests never touch a real
// user config directory (CLAUDE.md).
func useTempStore(t *testing.T) {
	t.Helper()
	t.Setenv(EnvConfigDir, t.TempDir())
}

func TestFileStoreRoundTrip(t *testing.T) {
	useTempStore(t)
	var s FileStore

	// A never-saved document has no record (reader opens at page 1, spec AC7).
	if _, ok, err := s.Load("doc-x"); err != nil || ok {
		t.Fatalf("Load of unknown doc: ok=%v err=%v, want ok=false err=nil", ok, err)
	}

	rec := Record{
		ID: "doc-x", Path: "/books/nl.pdf", PageCount: 42,
		Page: 17, OffsetY: 0.25, LastOpened: time.Now().UTC().Truncate(time.Second),
	}
	if err := s.Save(rec); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, ok, err := s.Load("doc-x")
	if err != nil || !ok {
		t.Fatalf("Load after Save: ok=%v err=%v", ok, err)
	}
	if got != rec {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", got, rec)
	}
}

// TestFileStorePersistsAcrossRestart models AC8: position survives a full
// process exit. A fresh FileStore value (nothing cached in memory) reads the
// record back from disk.
func TestFileStorePersistsAcrossRestart(t *testing.T) {
	useTempStore(t)
	if err := (FileStore{}).Save(Record{ID: "d", Path: "/p", Page: 9}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, ok, err := (FileStore{}).Load("d") // simulate a new process
	if err != nil || !ok {
		t.Fatalf("Load in 'new process': ok=%v err=%v", ok, err)
	}
	if got.Page != 9 {
		t.Errorf("restored page = %d, want 9", got.Page)
	}
}

func TestFileStoreSaveIsAtomicAndReplaces(t *testing.T) {
	useTempStore(t)
	var s FileStore
	s.Save(Record{ID: "d", Page: 1})
	s.Save(Record{ID: "d", Page: 2}) // overwrite
	s.Save(Record{ID: "e", Page: 5}) // second doc coexists

	if got, _, _ := s.Load("d"); got.Page != 2 {
		t.Errorf("doc d page = %d, want 2 (latest write wins)", got.Page)
	}
	if got, _, _ := s.Load("e"); got.Page != 5 {
		t.Errorf("doc e page = %d, want 5 (records coexist)", got.Page)
	}
	// No leftover temp files in the store dir.
	dir := os.Getenv(EnvConfigDir)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}

func TestFileStoreCorruptFileIsError(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigDir, dir)
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := (FileStore{}).Load("d")
	if err == nil {
		t.Fatal("corrupt store: want error, got nil (must not silently reset)")
	}
	if !strings.Contains(err.Error(), "corrupt") {
		t.Errorf("error should name the problem; got: %v", err)
	}
}

func TestFileStoreRejectsNewerSchema(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigDir, dir)
	future := `{"schemaVersion": 999, "records": {}}`
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte(future), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := (FileStore{}).Load("d"); err == nil {
		t.Fatal("newer schema: want error, got nil (must not downgrade)")
	}
}

func TestIdentifyStableAndContentSensitive(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.pdf")
	b := filepath.Join(dir, "b.pdf")
	os.WriteFile(a, []byte("hello"), 0o644)
	os.WriteFile(b, []byte("hello"), 0o644)

	idA1, err := Identify(a)
	if err != nil {
		t.Fatalf("Identify(a): %v", err)
	}
	idA2, _ := Identify(a)
	if idA1 != idA2 {
		t.Error("Identify not stable for the same file")
	}

	// Same bytes, different path → different identity (path is part of the key).
	idB, _ := Identify(b)
	if idA1 == idB {
		t.Error("different paths should yield different IDs even with equal content")
	}

	// Same path, changed content → different identity (content hash changes).
	os.WriteFile(a, []byte("world"), 0o644)
	idA3, _ := Identify(a)
	if idA1 == idA3 {
		t.Error("changed content should change the ID")
	}
}

func TestIdentifyMissingFileErrors(t *testing.T) {
	if _, err := Identify(filepath.Join(t.TempDir(), "nope.pdf")); err == nil {
		t.Error("Identify of a missing file should error")
	}
}
