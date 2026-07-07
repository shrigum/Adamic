package update

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// serveRelease returns an httptest server answering like the GitHub
// "latest release" endpoint. Tests point Check at it via EnvUpdateURL, so no
// test ever touches the network.
func serveRelease(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCheckReportsNewerVersion(t *testing.T) {
	srv := serveRelease(t, 200, `{"tag_name":"v1.2.0","html_url":"https://example.com/rel/v1.2.0"}`)
	t.Setenv(EnvUpdateURL, srv.URL)

	got, err := Check("1.1.3")
	if err != nil {
		t.Fatalf("Check(): %v", err)
	}
	want := Result{Current: "1.1.3", Latest: "1.2.0", URL: "https://example.com/rel/v1.2.0", Newer: true}
	if got != want {
		t.Errorf("Check() = %+v, want %+v (spec AC1)", got, want)
	}
}

func TestCheckUpToDate(t *testing.T) {
	srv := serveRelease(t, 200, `{"tag_name":"v1.2.0","html_url":"u"}`)
	t.Setenv(EnvUpdateURL, srv.URL)

	got, err := Check("1.2.0")
	if err != nil {
		t.Fatalf("Check(): %v", err)
	}
	if got.Newer {
		t.Errorf("same version reported as newer (spec AC2)")
	}
}

func TestCheckDevBuildNeverClaimsNewer(t *testing.T) {
	srv := serveRelease(t, 200, `{"tag_name":"v9.9.9","html_url":"u"}`)
	t.Setenv(EnvUpdateURL, srv.URL)

	got, err := Check("dev")
	if err != nil {
		t.Fatalf("Check(): %v", err)
	}
	if got.Newer {
		t.Errorf("dev build must not claim a newer version exists (spec AC5)")
	}
	if got.Latest != "9.9.9" {
		t.Errorf("dev build should still report the latest release; got %q", got.Latest)
	}
}

func TestCheckUnconfiguredRepo(t *testing.T) {
	// The ErrNotConfigured path guards a build whose Repo was never set (the
	// uninstantiated-template state). Adamic's Repo is configured
	// (shrigum/Adamic), so force the placeholder here to exercise the guard
	// without making a network call. Restore Repo afterward.
	orig := Repo
	Repo = repoPlaceholder
	t.Cleanup(func() { Repo = orig })

	t.Setenv(EnvUpdateURL, "") // ensure no override
	_, err := Check("1.0.0")
	if !errors.Is(err, ErrNotConfigured) {
		t.Errorf("placeholder repo: want ErrNotConfigured, got %v", err)
	}
}

func TestCheckServerErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{"http error status", 500, `{}`},
		{"not found", 404, `{}`},
		{"unparsable body", 200, `not json`},
		{"non-semver tag", 200, `{"tag_name":"nightly","html_url":"u"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := serveRelease(t, tt.status, tt.body)
			t.Setenv(EnvUpdateURL, srv.URL)
			if _, err := Check("1.0.0"); err == nil {
				t.Errorf("want error, got nil (spec AC3: failures are loud, never a wrong answer)")
			}
		})
	}
}

func TestCheckUnreachableServer(t *testing.T) {
	srv := serveRelease(t, 200, `{}`)
	url := srv.URL
	srv.Close() // now nothing listens there
	t.Setenv(EnvUpdateURL, url)

	_, err := Check("1.0.0")
	if err == nil {
		t.Fatal("unreachable server: want error, got nil (spec AC3)")
	}
	if !strings.Contains(err.Error(), "unaffected") {
		t.Errorf("offline error should reassure that the app is unaffected; got: %v", err)
	}
}

func TestSemverGreater(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"1.0.1", "1.0.0", true},
		{"1.0.0", "1.0.1", false},
		{"2.0.0", "1.9.9", true},
		{"1.10.0", "1.9.0", true}, // numeric, not lexicographic
		{"1.0.0", "1.0.0", false},
		{"1.0.0", "1.0.0-rc1", true},  // release > its pre-release
		{"1.0.0-rc1", "1.0.0", false}, // the sort -V trap
		{"1.0.0-rc2", "1.0.0-rc1", true},
		{"1.0.0+build5", "1.0.0", false}, // build metadata ignored
	}
	for _, tt := range tests {
		t.Run(tt.a+" vs "+tt.b, func(t *testing.T) {
			got, err := semverGreater(tt.a, tt.b)
			if err != nil {
				t.Fatalf("semverGreater(%q, %q): %v", tt.a, tt.b, err)
			}
			if got != tt.want {
				t.Errorf("semverGreater(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
