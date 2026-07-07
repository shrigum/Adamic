// Package library is the minimal per-document store REQ-1 needs to remember
// where a reader left off. It is deliberately narrow (design-review condition
// C3): a Save/Load pair over a small document record keyed by stable document
// identity — not a general repository. Structured user data ultimately lives in
// SQLite (ADR-0008); this file-backed store is the interim implementation the
// pdf-reader-core spec assumes (A3), and when the SQLite store lands it
// implements the same Store interface or replaces it, with no change to the
// reader or its acceptance criteria.
//
// Document identity (spec A4) is the absolute file path plus a content hash, so
// a moved file still restores its position and two different files never
// collide on a shared path. Hashing is the caller's concern (Identify); the
// store just keys on the resulting DocID.
//
// Failure modes (docs/CODING_STANDARDS.md, "Own your failure modes"):
//   - A missing store file is not an error: Load returns "no record" (the
//     reader opens at page 1, spec AC7).
//   - A corrupt/unreadable store file is a returned error naming the path; it is
//     never silently discarded or overwritten. The reader treats a load failure
//     as soft — the document still opens, position just isn't restored.
//   - Save is atomic (same-directory temp file + rename): a crash never leaves a
//     partially written store, and the previous contents survive any failure.
//   - A store written by a newer schema version this build doesn't understand
//     is refused (a returned error), never silently rewritten and downgraded.
package library

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// DocID is a stable identity for a document: absolute path + content hash (A4).
// It is the key under which a reading position is stored.
type DocID string

// Record is the minimum per-document metadata REQ-1 stores (spec A4). Title,
// author, language and the rest of FR-DOC-02 belong to the later Library
// Manager, not here.
type Record struct {
	ID         DocID     `json:"id"`
	Path       string    `json:"path"`
	PageCount  int       `json:"pageCount"`
	Page       int       `json:"page"`    // last reading position: page index (0-based)
	OffsetY    float64   `json:"offsetY"` // within-page scroll offset, fraction of page height
	LastOpened time.Time `json:"lastOpened"`
}

// Store persists reading positions. It is the swappable seam (C3): the
// file-backed FileStore below now, a SQLite-backed store later (ADR-0008).
type Store interface {
	// Load returns the stored record for id and whether one existed. A document
	// with no stored record returns ok=false and a nil error (open at page 1).
	Load(id DocID) (rec Record, ok bool, err error)
	// Save writes rec under rec.ID, replacing any previous record for it.
	Save(rec Record) error
}

// Identify computes the stable DocID for a file: its absolute path plus a
// content hash. The hash reads the whole file; callers that must not block on a
// large file should call this off the open path (spec risk T10 mitigation). A
// read error is returned — identity can't be faked, so the caller decides
// whether to proceed without position persistence.
func Identify(path string) (DocID, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	f, err := os.Open(abs)
	if err != nil {
		return "", fmt.Errorf("identify %s: %w", abs, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", abs, err)
	}
	sum := hex.EncodeToString(h.Sum(nil))
	return DocID(abs + "\x00" + sum), nil
}

// EnvConfigDir overrides the directory holding the library store file. It
// matches settings.EnvConfigDir so both stores live in one place and tests
// never touch real user directories (CLAUDE.md). Shared name, one override.
const EnvConfigDir = "ADAMIC_CONFIG_DIR"

const (
	dirName  = "adamic"
	fileName = "library.json"

	// schemaVersion is the on-disk layout version. Bump only with a migration
	// (mirrors the settings store's envelope discipline, ADR-0004).
	schemaVersion = 1
)

// FileStore is the interim file-backed Store: one JSON file of records in the
// user config dir, written atomically.
type FileStore struct{}

var _ Store = FileStore{}

// envelope is the on-disk layout: a version plus records keyed by DocID. A
// build that doesn't recognize a newer schemaVersion refuses to load rather
// than silently rewriting (load()), which is the forward-compatibility
// guarantee — full field-level preservation waits for the SQLite store.
type envelope struct {
	SchemaVersion int              `json:"schemaVersion"`
	Records       map[DocID]Record `json:"records"`
}

// Path returns the absolute path of the library store file, whether or not it
// exists. It fails only when the user config dir can't be determined and
// EnvConfigDir is unset.
func (FileStore) Path() (string, error) {
	if dir := os.Getenv(EnvConfigDir); dir != "" {
		return filepath.Join(dir, fileName), nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config directory (set %s to override): %w", EnvConfigDir, err)
	}
	return filepath.Join(base, dirName, fileName), nil
}

func (s FileStore) Load(id DocID) (Record, bool, error) {
	env, err := s.load()
	if err != nil {
		return Record{}, false, err
	}
	rec, ok := env.Records[id]
	return rec, ok, nil
}

func (s FileStore) Save(rec Record) error {
	env, err := s.load()
	if err != nil {
		return err
	}
	if env.Records == nil {
		env.Records = map[DocID]Record{}
	}
	env.Records[rec.ID] = rec
	return s.save(env)
}

// load reads the store file. A missing file is an empty envelope (not an error).
func (s FileStore) load() (envelope, error) {
	path, err := s.Path()
	if err != nil {
		return envelope{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return envelope{SchemaVersion: schemaVersion, Records: map[DocID]Record{}}, nil
	}
	if err != nil {
		return envelope{}, fmt.Errorf("read library store %s: %w", path, err)
	}
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return envelope{}, fmt.Errorf("library store %s is corrupt (%v); fix or delete it to reset", path, err)
	}
	if env.SchemaVersion > schemaVersion {
		return envelope{}, fmt.Errorf("library store %s uses schema version %d, newer than this build (%d) understands; upgrade the app", path, env.SchemaVersion, schemaVersion)
	}
	if env.Records == nil {
		env.Records = map[DocID]Record{}
	}
	return env, nil
}

// save writes the envelope atomically: temp file in the target dir, then rename
// over the store (same discipline as the settings store).
func (s FileStore) save(env envelope) error {
	path, err := s.Path()
	if err != nil {
		return err
	}
	env.SchemaVersion = schemaVersion
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create library directory %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return fmt.Errorf("encode library store: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, fileName+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary library file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write library store %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("write library store %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("replace library store %s: %w", path, err)
	}
	return nil
}
