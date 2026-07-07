// Package update checks GitHub Releases for a newer published version of the
// app (ADR-0003: docs/architecture/ADR-0003-update-check.md).
//
// The check is strictly opt-in: nothing in this package runs unless the user
// explicitly invokes `app update`, and the app is fully functional without
// ever calling it (local-first: the network is optional and this is the only
// code in the application that touches it).
//
// Failure modes (docs/CODING_STANDARDS.md, "Own your failure modes"):
//   - Repo not configured (template not yet instantiated): ErrNotConfigured.
//   - Network unreachable, timeout (5s), or non-200 response: a returned
//     error saying the check failed and that the app is unaffected. No retry,
//     no cache, no state written anywhere.
//   - An unparsable latest-version tag is an error, never a false "newer
//     version available".
package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Repo is the "owner/name" GitHub repository whose Releases are checked.
// It can be overridden at build time via
//
//	-ldflags "-X github.com/shrigum/adamic/src/update.Repo=owner/name"
//
// The repository is public, so the unauthenticated check works (ADR-0003).
var Repo = "shrigum/Adamic"

const (
	repoPlaceholder = "OWNER/REPO"

	// EnvUpdateURL overrides the release-lookup URL. It exists for tests and
	// for self-hosted forges; when set, the check proceeds even if Repo is
	// unconfigured.
	EnvUpdateURL = "ADAMIC_UPDATE_URL"

	requestTimeout = 5 * time.Second
)

// ErrNotConfigured means the template's Repo placeholder was never replaced,
// so there is no repository to check against.
var ErrNotConfigured = errors.New("update checks are not configured for this build (no release repository set)")

// Result reports the outcome of a successful check.
type Result struct {
	Current string // version this binary was built as ("dev" for local builds)
	Latest  string // latest published release version, without the "v" prefix
	URL     string // human-facing release page
	// Newer is true when Latest is a higher SemVer than Current. It is always
	// false for "dev" builds, which cannot be meaningfully compared.
	Newer bool
}

// Check queries the release repository for the latest published version and
// compares it with currentVersion. It performs exactly one HTTP request.
func Check(currentVersion string) (Result, error) {
	url := os.Getenv(EnvUpdateURL)
	if url == "" {
		if Repo == repoPlaceholder || Repo == "" {
			return Result{}, ErrNotConfigured
		}
		url = "https://api.github.com/repos/" + Repo + "/releases/latest"
	}

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return Result{}, fmt.Errorf("check for updates (the app is unaffected and works offline): %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("check for updates: %s returned %s (the app is unaffected)", url, resp.Status)
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return Result{}, fmt.Errorf("check for updates: parse release info from %s: %w", url, err)
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	if _, _, _, err := parseCore(latest); err != nil {
		return Result{}, fmt.Errorf("check for updates: latest release tag %q is not SemVer: %w", release.TagName, err)
	}

	res := Result{Current: currentVersion, Latest: latest, URL: release.HTMLURL}
	if currentVersion != "dev" {
		res.Newer, err = semverGreater(latest, currentVersion)
		if err != nil {
			return Result{}, fmt.Errorf("check for updates: compare %q with running version %q: %w", latest, currentVersion, err)
		}
	}
	return res, nil
}

// semverGreater reports whether version a is greater than b. Pre-release
// versions rank below their release (1.0.0-rc1 < 1.0.0); two pre-releases of
// the same core version are compared as plain strings, which is sufficient
// for this project's tagging scheme (rc1 < rc2). Build metadata is ignored.
func semverGreater(a, b string) (bool, error) {
	aMaj, aMin, aPat, err := parseCore(a)
	if err != nil {
		return false, err
	}
	bMaj, bMin, bPat, err := parseCore(b)
	if err != nil {
		return false, err
	}
	if aMaj != bMaj {
		return aMaj > bMaj, nil
	}
	if aMin != bMin {
		return aMin > bMin, nil
	}
	if aPat != bPat {
		return aPat > bPat, nil
	}
	aPre, bPre := prerelease(a), prerelease(b)
	if aPre == bPre {
		return false, nil
	}
	if aPre == "" {
		return true, nil // release > pre-release
	}
	if bPre == "" {
		return false, nil
	}
	return aPre > bPre, nil
}

// parseCore extracts the MAJOR.MINOR.PATCH integers of a SemVer string.
func parseCore(v string) (major, minor, patch int, err error) {
	core := strings.SplitN(v, "-", 2)[0]
	core = strings.SplitN(core, "+", 2)[0]
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("%q is not MAJOR.MINOR.PATCH", v)
	}
	nums := make([]int, 3)
	for i, p := range parts {
		nums[i], err = strconv.Atoi(p)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("%q is not MAJOR.MINOR.PATCH: %w", v, err)
		}
	}
	return nums[0], nums[1], nums[2], nil
}

// prerelease returns the pre-release part of a SemVer string, "" if none.
func prerelease(v string) string {
	rest := strings.SplitN(v, "+", 2)[0]
	if i := strings.Index(rest, "-"); i >= 0 {
		return rest[i+1:]
	}
	return ""
}
