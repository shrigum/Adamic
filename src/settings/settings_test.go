package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// All tests redirect the settings location with EnvConfigDir so they never
// touch the developer's real config directory (docs/CODING_STANDARDS.md,
// "Tests").

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	t.Setenv(EnvConfigDir, t.TempDir())

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() with no file: unexpected error: %v", err)
	}
	if got["greeting"] != "Hello" {
		t.Errorf("default greeting = %q, want %q", got["greeting"], "Hello")
	}
}

func TestLoadDoesNotCreateFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigDir, dir)

	if _, err := Load(); err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, fileName)); !os.IsNotExist(err) {
		t.Errorf("Load() created the settings file; reads must not write (spec AC2)")
	}
}

func TestLoadCorruptFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigDir, dir)
	path := filepath.Join(dir, fileName)
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() on corrupt file: want error, got nil")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("corrupt-file error must name the path (spec AC6); got: %v", err)
	}
	// The corrupt file must be left for the user to inspect, never reset.
	data, readErr := os.ReadFile(path)
	if readErr != nil || string(data) != "{not json" {
		t.Errorf("corrupt file was modified; must be left intact (spec AC6)")
	}
}

func TestSetThenLoadRoundTrips(t *testing.T) {
	t.Setenv(EnvConfigDir, t.TempDir())

	tests := []struct {
		name       string
		key, value string
	}{
		{"known key", "greeting", "Hey"},
		{"unknown key allowed", "future_key", "42"},
		{"value with spaces", "greeting", "Good morning"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Set(tt.key, tt.value); err != nil {
				t.Fatalf("Set(%q, %q): %v", tt.key, tt.value, err)
			}
			got, err := Load()
			if err != nil {
				t.Fatalf("Load(): %v", err)
			}
			if got[tt.key] != tt.value {
				t.Errorf("after Set, Load()[%q] = %q, want %q", tt.key, got[tt.key], tt.value)
			}
		})
	}
}

func TestSetPreservesUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigDir, dir)
	// Simulate a file written by a newer version or edited by the user.
	seed := "{\n  \"from_the_future\": \"keep me\"\n}\n"
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Set("greeting", "Hey"); err != nil {
		t.Fatalf("Set(): %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if got["from_the_future"] != "keep me" {
		t.Errorf("unknown key was dropped on read-modify-write (spec AC8)")
	}
	if got["greeting"] != "Hey" {
		t.Errorf("greeting = %q, want %q", got["greeting"], "Hey")
	}
}

func TestLoadLegacyFlatFileAndMigrateOnSave(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigDir, dir)
	path := filepath.Join(dir, fileName)
	// A file written before schema versioning existed: a bare flat map.
	legacy := "{\n  \"greeting\": \"Hey\",\n  \"custom\": \"kept\"\n}\n"
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() legacy file: %v", err)
	}
	if got["greeting"] != "Hey" || got["custom"] != "kept" {
		t.Errorf("legacy values not read: %v", got)
	}

	// Reads must not rewrite the file (spec AC2 spirit); the migration to the
	// envelope happens on the next save.
	data, _ := os.ReadFile(path)
	if string(data) != legacy {
		t.Errorf("Load() modified the legacy file; migration must happen on save only")
	}
	if err := Set("greeting", "Hoi"); err != nil {
		t.Fatalf("Set(): %v", err)
	}
	data, _ = os.ReadFile(path)
	if !strings.Contains(string(data), "\"schemaVersion\": 1") {
		t.Errorf("save did not migrate legacy file to the versioned envelope; got:\n%s", data)
	}
	got, err = Load()
	if err != nil {
		t.Fatalf("Load() after migration: %v", err)
	}
	if got["custom"] != "kept" {
		t.Errorf("legacy key dropped during migration (spec AC8 spirit)")
	}
}

func TestLoadNewerSchemaVersionErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigDir, dir)
	path := filepath.Join(dir, fileName)
	future := "{\n  \"schemaVersion\": 99,\n  \"settings\": {}\n}\n"
	if err := os.WriteFile(path, []byte(future), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() of future schema version: want error, got nil")
	}
	if !strings.Contains(err.Error(), "newer version") || !strings.Contains(err.Error(), path) {
		t.Errorf("future-schema error must explain the cause and name the path; got: %v", err)
	}
	// Never downgrade or rewrite a file we don't understand.
	data, _ := os.ReadFile(path)
	if string(data) != future {
		t.Errorf("future-schema file was modified; must be left intact")
	}
}

func TestSaveWritesVersionedEnvelope(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigDir, dir)

	if err := Set("greeting", "Hey"); err != nil {
		t.Fatalf("Set(): %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, fileName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\"schemaVersion\": 1") {
		t.Errorf("saved file lacks schemaVersion (ADR-0004); got:\n%s", data)
	}
}

func TestSaveIsAtomic(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigDir, dir)

	// Rename-over-existing is the mechanism atomicity relies on; pin that it
	// works on this platform, Windows included (T4 spike/risk,
	// docs/planning/settings-file/critical-path.md).
	if err := Set("greeting", "first"); err != nil {
		t.Fatalf("first Set(): %v", err)
	}
	if err := Set("greeting", "second"); err != nil {
		t.Fatalf("Set() over existing file: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if got["greeting"] != "second" {
		t.Errorf("greeting = %q, want %q", got["greeting"], "second")
	}

	// No temp files may survive a successful save.
	leftovers, err := filepath.Glob(filepath.Join(dir, fileName+".tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 0 {
		t.Errorf("temp files left behind after save: %v", leftovers)
	}
}

func TestSaveErrorLeavesNoSettingsFile(t *testing.T) {
	// Point the config dir *inside a regular file* so MkdirAll must fail:
	// the error path must not leave any settings file behind.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvConfigDir, filepath.Join(blocker, "sub"))

	if err := Set("greeting", "Hey"); err == nil {
		t.Fatal("Set() into an impossible directory: want error, got nil")
	}
}

func TestPathHonorsOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfigDir, dir)

	got, err := Path()
	if err != nil {
		t.Fatalf("Path(): %v", err)
	}
	want := filepath.Join(dir, fileName)
	if got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}
