// Package settings persists user preferences as a flat map of string keys to
// string values, stored as pretty-printed JSON in the OS user config
// directory (ADR-0002: docs/architecture/ADR-0002-settings-file-format.md).
// The file carries a schema version so future layout changes can be migrated
// deterministically (ADR-0004); files written before versioning (a bare flat
// map) are read as legacy and rewritten in the current layout on next save.
//
// Failure modes (docs/CODING_STANDARDS.md, "Own your failure modes"):
//   - Missing file is not an error: Load returns the defaults.
//   - A corrupt or unreadable file is a returned error naming the path;
//     the file is never silently overwritten or reset.
//   - A file with a schema version newer than this build understands is a
//     returned error (likely written by a newer app version) — never
//     reinterpreted or downgraded.
//   - Save is atomic (same-directory temp file + rename): on any error the
//     previous settings file is intact, and a crash cannot leave a
//     partially-written file at the settings path.
//   - Unknown keys found in the file are preserved across read-modify-write
//     (spec assumption A2).
//   - Concurrent writers are not coordinated: two processes saving at once is
//     last-writer-wins (each write is still atomic, so the file is never
//     corrupted, but one process's change is lost). Acceptable for a CLI;
//     revisit before any long-running multi-instance use.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	dirName  = "adamic"
	fileName = "settings.json"

	// EnvConfigDir, when set, overrides the directory containing the settings
	// file. It exists for tests and for portable installs (design review
	// condition C3, docs/planning/settings-file/design-review.md).
	EnvConfigDir = "ADAMIC_CONFIG_DIR"

	// fileSchemaVersion is the on-disk layout version this build reads and
	// writes. Bump it only with a migration from every older version
	// (ADR-0004, docs/architecture/ADR-0004-settings-schema-version.md).
	fileSchemaVersion = 1
)

// fileEnvelope is the on-disk layout: a version field plus the settings map.
// Files that predate versioning are a bare flat map (legacy, version 0) and
// are detected by the absence of schemaVersion.
type fileEnvelope struct {
	SchemaVersion int               `json:"schemaVersion"`
	Settings      map[string]string `json:"settings"`
}

// Defaults returns the built-in default settings. Callers own the returned map.
func Defaults() map[string]string {
	return map[string]string{
		"greeting": "Hello",
	}
}

// Path returns the absolute path of the settings file, whether or not it
// exists. It fails only when the user config directory cannot be determined
// (e.g. HOME/AppData unset) and EnvConfigDir is not set.
func Path() (string, error) {
	if dir := os.Getenv(EnvConfigDir); dir != "" {
		return filepath.Join(dir, fileName), nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config directory (set %s to override): %w", EnvConfigDir, err)
	}
	return filepath.Join(base, dirName, fileName), nil
}

// Load returns the effective settings: Defaults overlaid with whatever is
// stored on disk. A missing file yields the defaults and does not create the
// file (spec AC2).
func Load() (map[string]string, error) {
	stored, err := loadStored()
	if err != nil {
		return nil, err
	}
	effective := Defaults()
	for k, v := range stored {
		effective[k] = v
	}
	return effective, nil
}

// Set stores key=value, creating the settings file (and its directory) on
// first use. Keys already in the file — including ones this version does not
// recognize — are preserved (spec AC8).
func Set(key, value string) error {
	stored, err := loadStored()
	if err != nil {
		return err
	}
	stored[key] = value
	return save(stored)
}

// loadStored reads only what is on disk, without defaults applied. A missing
// file is an empty map.
func loadStored() (map[string]string, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read settings file %s: %w", path, err)
	}
	var env fileEnvelope
	envErr := json.Unmarshal(data, &env)
	if envErr == nil && env.SchemaVersion > fileSchemaVersion {
		return nil, fmt.Errorf("settings file %s uses schema version %d, but this build understands up to %d — it was likely written by a newer version of the app; upgrade, or delete the file to reset to defaults", path, env.SchemaVersion, fileSchemaVersion)
	}
	if envErr == nil && env.SchemaVersion == fileSchemaVersion {
		if env.Settings == nil {
			return map[string]string{}, nil
		}
		return env.Settings, nil
	}

	// No (or unparsable) schemaVersion field: either a legacy pre-versioning
	// file (a bare flat map, migrated to the envelope on next save) or corrupt.
	var legacy map[string]string
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("settings file is corrupt (%v); fix it or delete it to reset to defaults: %s", err, path)
	}
	if legacy == nil {
		legacy = map[string]string{}
	}
	return legacy, nil
}

// save writes the stored map atomically: encode to a temp file in the target
// directory, then rename over the settings file. Same-directory placement
// keeps the rename on one filesystem; os.Rename replaces an existing target
// on all supported platforms (T4 spike, docs/planning/settings-file/critical-path.md).
func save(stored map[string]string) error {
	path, err := Path()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create settings directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(fileEnvelope{
		SchemaVersion: fileSchemaVersion,
		Settings:      stored,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, fileName+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary settings file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	// On any failure below, remove the temp file; the real settings file has
	// not been touched yet.
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write settings file %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("write settings file %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("replace settings file %s: %w", path, err)
	}
	return nil
}
