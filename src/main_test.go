// Unit tests for the CLI wiring: dispatch, exit codes, and presentation.
// Behavior end-to-end (real process, real files) is covered by tests/;
// these pin run()'s contract cheaply and give every package a test anchor.
package main

import (
	"bytes"
	"strings"
	"testing"
)

// runCLI calls run with captured output and an isolated settings dir.
func runCLI(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	t.Setenv("ADAMIC_CONFIG_DIR", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func TestRunVersionFlag(t *testing.T) {
	stdout, _, code := runCLI(t, "--version")
	if code != 0 {
		t.Fatalf("--version: exit %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != version {
		t.Errorf("--version printed %q, want %q", stdout, version)
	}
}

func TestRunGreetsWithFlagOverride(t *testing.T) {
	stdout, _, code := runCLI(t, "--greeting", "Yo", "--name", "Ada")
	if code != 0 {
		t.Fatalf("greet: exit %d, want 0", code)
	}
	if stdout != "Yo, Ada!\n" {
		t.Errorf("greet = %q, want %q", stdout, "Yo, Ada!\n")
	}
}

func TestRunGreetsWithDefaults(t *testing.T) {
	stdout, _, code := runCLI(t)
	if code != 0 {
		t.Fatalf("no args: exit %d, want 0", code)
	}
	if stdout != "Hello, world!\n" {
		t.Errorf("default greet = %q, want %q", stdout, "Hello, world!\n")
	}
}

func TestRunRejectsUnknownFlag(t *testing.T) {
	_, stderr, code := runCLI(t, "--no-such-flag")
	if code != 2 {
		t.Errorf("unknown flag: exit %d, want 2 (usage error)", code)
	}
	if stderr == "" {
		t.Error("unknown flag should print usage to stderr")
	}
}

func TestRunConfigUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"bare config", []string{"config"}},
		{"unknown subcommand", []string{"config", "frobnicate"}},
		{"get without key", []string{"config", "get"}},
		{"set without value", []string{"config", "set", "greeting"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, stderr, code := runCLI(t, tt.args...)
			if code != 2 {
				t.Errorf("%v: exit %d, want 2 (usage error)", tt.args, code)
			}
			if !strings.Contains(stderr, "usage:") {
				t.Errorf("%v: stderr should show usage, got %q", tt.args, stderr)
			}
		})
	}
}

func TestRunConfigSetGetRoundTrip(t *testing.T) {
	t.Setenv("ADAMIC_CONFIG_DIR", t.TempDir())
	var out, errOut bytes.Buffer

	if code := run([]string{"config", "set", "greeting", "Hei"}, &out, &errOut); code != 0 {
		t.Fatalf("config set: exit %d, stderr: %s", code, errOut.String())
	}
	out.Reset()
	if code := run([]string{"config", "get", "greeting"}, &out, &errOut); code != 0 {
		t.Fatalf("config get: exit %d, stderr: %s", code, errOut.String())
	}
	if out.String() != "Hei\n" {
		t.Errorf("config get after set = %q, want %q", out.String(), "Hei\n")
	}
}
