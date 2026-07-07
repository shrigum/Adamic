// Package cli contains integration tests that exercise the built binary
// end-to-end, as a user would run it (settings-file T8 and acceptance
// criteria AC1/AC3–AC6; see docs/planning/settings-file/spec.md).
//
// The binary is built once in TestMain so `go test ./...` from the repo root
// remains the single test entry point. Every invocation redirects the
// settings location with ADAMIC_CONFIG_DIR so no test touches the real user
// config directory.
package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var binPath string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "adamic-cli-test-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)

	binPath = filepath.Join(tmp, "adamic")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	build := exec.Command("go", "build", "-o", binPath, "github.com/shrigum/adamic/src")
	build.Dir = ".." // repo root, where go.mod lives
	if out, err := build.CombinedOutput(); err != nil {
		panic("building adamic binary for integration tests: " + err.Error() + "\n" + string(out))
	}

	os.Exit(m.Run())
}

// runApp invokes the built binary with an isolated settings directory and
// returns stdout, stderr, and the exit code.
func runApp(t *testing.T, configDir string, args ...string) (string, string, int) {
	t.Helper()
	return runAppEnv(t, []string{"ADAMIC_CONFIG_DIR=" + configDir}, args...)
}

// runAppEnv is runApp with arbitrary extra environment variables.
func runAppEnv(t *testing.T, extraEnv []string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Env = append(os.Environ(), extraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("running %v: %v", args, err)
	}
	return stdout.String(), stderr.String(), code
}

func TestSettingsPersistAcrossRuns(t *testing.T) {
	dir := t.TempDir()

	_, stderr, code := runApp(t, dir, "config", "set", "greeting", "Hey")
	if code != 0 {
		t.Fatalf("config set: exit %d, stderr: %s", code, stderr)
	}
	stdout, stderr, code := runApp(t, dir, "--name", "World")
	if code != 0 {
		t.Fatalf("greet: exit %d, stderr: %s", code, stderr)
	}
	if got, want := stdout, "Hey, World!\n"; got != want {
		t.Errorf("greet after set = %q, want %q (spec AC1)", got, want)
	}
}

func TestNoFileCreatedOnRead(t *testing.T) {
	dir := t.TempDir()

	stdout, stderr, code := runApp(t, dir, "--name", "World")
	if code != 0 {
		t.Fatalf("greet: exit %d, stderr: %s", code, stderr)
	}
	if got, want := stdout, "Hello, World!\n"; got != want {
		t.Errorf("default greet = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.json")); !os.IsNotExist(err) {
		t.Errorf("read-only use created a settings file (spec AC2)")
	}
}

func TestConfigGet(t *testing.T) {
	dir := t.TempDir()

	t.Run("default for known unset key", func(t *testing.T) {
		stdout, _, code := runApp(t, dir, "config", "get", "greeting")
		if code != 0 || stdout != "Hello\n" {
			t.Errorf("get greeting = (%q, exit %d), want (%q, exit 0) (spec AC3)", stdout, code, "Hello\n")
		}
	})
	t.Run("stored value", func(t *testing.T) {
		runApp(t, dir, "config", "set", "greeting", "Hey")
		stdout, _, code := runApp(t, dir, "config", "get", "greeting")
		if code != 0 || stdout != "Hey\n" {
			t.Errorf("get greeting = (%q, exit %d), want (%q, exit 0)", stdout, code, "Hey\n")
		}
	})
	t.Run("unknown unset key errors", func(t *testing.T) {
		_, stderr, code := runApp(t, dir, "config", "get", "no_such_key")
		if code != 1 {
			t.Errorf("get no_such_key: exit %d, want 1 (spec AC3)", code)
		}
		if !strings.Contains(stderr, "no_such_key") {
			t.Errorf("error should name the key; stderr: %s", stderr)
		}
	})
}

func TestConfigList(t *testing.T) {
	dir := t.TempDir()
	runApp(t, dir, "config", "set", "zeta", "1")
	runApp(t, dir, "config", "set", "alpha", "2")

	stdout, stderr, code := runApp(t, dir, "config", "list")
	if code != 0 {
		t.Fatalf("config list: exit %d, stderr: %s", code, stderr)
	}
	want := "alpha=2\ngreeting=Hello\nzeta=1\n"
	if stdout != want {
		t.Errorf("config list = %q, want sorted effective settings %q (spec AC4)", stdout, want)
	}
}

func TestConfigPath(t *testing.T) {
	dir := t.TempDir()

	stdout, stderr, code := runApp(t, dir, "config", "path")
	if code != 0 {
		t.Fatalf("config path: exit %d, stderr: %s", code, stderr)
	}
	want := filepath.Join(dir, "settings.json") + "\n"
	if stdout != want {
		t.Errorf("config path = %q, want %q (spec AC5; must work with no file present)", stdout, want)
	}
}

func TestUpdateCommand(t *testing.T) {
	t.Run("newer release available", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"tag_name":"v99.0.0","html_url":"https://example.com/rel"}`))
		}))
		defer srv.Close()

		// The test binary is built without -ldflags, so it runs as "dev":
		// it must report the latest release without claiming an upgrade
		// comparison (spec AC5), and print the release URL.
		stdout, stderr, code := runAppEnv(t, []string{"ADAMIC_UPDATE_URL=" + srv.URL}, "update")
		if code != 0 {
			t.Fatalf("update: exit %d, stderr: %s", code, stderr)
		}
		if !strings.Contains(stdout, "99.0.0") || !strings.Contains(stdout, "https://example.com/rel") {
			t.Errorf("update output missing version or URL: %q", stdout)
		}
	})

	t.Run("unreachable server fails loud and harmless", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		url := srv.URL
		srv.Close()

		_, stderr, code := runAppEnv(t, []string{"ADAMIC_UPDATE_URL=" + url}, "update")
		if code != 1 {
			t.Errorf("update while offline: exit %d, want 1 (spec AC3)", code)
		}
		if !strings.Contains(stderr, "unaffected") {
			t.Errorf("offline error should reassure the app is unaffected; stderr: %s", stderr)
		}
	})

	t.Run("no releases yet degrades gracefully", func(t *testing.T) {
		// Adamic's Repo is a real, configured repository (shrigum/Adamic) with
		// no releases yet, so the Releases API returns 404. The check must
		// report the failure clearly and leave the app unaffected — it must
		// never crash or claim an update. (Before the first release this is the
		// live behavior of `adamic update`.)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Not Found", http.StatusNotFound)
		}))
		defer srv.Close()

		_, stderr, code := runAppEnv(t, []string{"ADAMIC_UPDATE_URL=" + srv.URL}, "update")
		if code != 1 {
			t.Errorf("update with no releases: exit %d, want 1 (spec AC3)", code)
		}
		if !strings.Contains(stderr, "unaffected") {
			t.Errorf("error should reassure the app is unaffected; stderr: %s", stderr)
		}
	})
}

func TestCorruptFileErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := runApp(t, dir, "--name", "World")
	if code != 1 {
		t.Errorf("greet with corrupt settings: exit %d, want 1 (spec AC6)", code)
	}
	if !strings.Contains(stderr, path) {
		t.Errorf("corrupt-file error must name the path %q; stderr: %s", path, stderr)
	}
	// Never silently reset: file must be byte-identical afterwards.
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "{broken" {
		t.Errorf("corrupt settings file was modified (spec AC6)")
	}
}
