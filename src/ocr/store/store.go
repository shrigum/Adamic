// Package store is the interim file-backed ocr.Store (task T6): one JSON
// file per document's OCR result, in an ocr/ subdirectory of the user config
// dir, written atomically. It mirrors library.FileStore (design-review
// condition C3) — same config-dir override, same versioned envelope, same
// temp-file+rename write discipline — and is swapped for the SQLite store
// (ADR-0008) by implementing ocr.Store there, with no caller change.
//
// Failure modes (docs/CODING_STANDARDS.md, "Own your failure modes"):
//   - A missing result file is not an error: Load returns ok=false (the
//     document simply has no OCR yet, spec error summary).
//   - A corrupt/unreadable result file is a returned error naming the path;
//     it is never silently discarded or overwritten. Callers treat it as soft
//     — the document still opens and reads, OCR can be re-run.
//   - Save is atomic (same-directory temp file + rename): a crash never
//     leaves a partially written result, and the previous result survives any
//     failure.
//   - A result written by a newer schema version this build doesn't
//     understand is refused (a returned error), never rewritten/downgraded.
package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/shrigum/adamic/src/library"
	"github.com/shrigum/adamic/src/ocr"
)

const (
	dirName    = "adamic"
	ocrDirName = "ocr"

	// schemaVersion is the on-disk layout version of a stored OCR result — a
	// near-term SemVer surface (spec open Q4). Bump only with a migration
	// (the same envelope discipline as the library and settings stores).
	schemaVersion = 1
)

// FileStore is the interim file-backed ocr.Store.
type FileStore struct{}

var _ ocr.Store = FileStore{}

// envelope is the on-disk layout: a version plus the one document's result.
type envelope struct {
	SchemaVersion int        `json:"schemaVersion"`
	Result        ocr.Result `json:"result"`
}

// Path returns the absolute path of the OCR result file for id, whether or
// not it exists. The filename is a content hash of the DocID (the id embeds
// an absolute path, which is not filesystem-safe). It fails only when the
// user config dir can't be determined and library.EnvConfigDir is unset.
func (FileStore) Path(id library.DocID) (string, error) {
	sum := sha256.Sum256([]byte(id))
	name := hex.EncodeToString(sum[:]) + ".json"
	if dir := os.Getenv(library.EnvConfigDir); dir != "" {
		return filepath.Join(dir, ocrDirName, name), nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config directory (set %s to override): %w", library.EnvConfigDir, err)
	}
	return filepath.Join(base, dirName, ocrDirName, name), nil
}

func (s FileStore) Load(id library.DocID) (ocr.Result, bool, error) {
	if id == "" {
		return ocr.Result{}, false, errors.New("load OCR result: empty document identity")
	}
	path, err := s.Path(id)
	if err != nil {
		return ocr.Result{}, false, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return ocr.Result{}, false, nil
	}
	if err != nil {
		return ocr.Result{}, false, fmt.Errorf("read OCR result %s: %w", path, err)
	}
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return ocr.Result{}, false, fmt.Errorf("OCR result %s is corrupt (%v); delete it and re-run OCR", path, err)
	}
	if env.SchemaVersion > schemaVersion {
		return ocr.Result{}, false, fmt.Errorf("OCR result %s uses schema version %d, newer than this build (%d) understands; upgrade the app", path, env.SchemaVersion, schemaVersion)
	}
	if env.Result.ID != id {
		return ocr.Result{}, false, fmt.Errorf("OCR result %s belongs to a different document; delete it and re-run OCR", path)
	}
	return env.Result, true, nil
}

func (s FileStore) Save(result ocr.Result) error {
	if result.ID == "" {
		return errors.New("save OCR result: empty document identity")
	}
	path, err := s.Path(result.ID)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create OCR result directory %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(envelope{SchemaVersion: schemaVersion, Result: result}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode OCR result: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary OCR result file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write OCR result %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("write OCR result %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("replace OCR result %s: %w", path, err)
	}
	return nil
}
