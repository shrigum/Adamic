// Command adamic is the application entry point. For now it carries the
// template's greeting scaffold and the persistent local settings file (see
// docs/planning/settings-file/ for that feature's full planning trail); the
// greeting command is scaffold to be replaced by the first real feature
// (REQ-1, docs/planning/BACKLOG.md).
//
// This file is CLI wiring only: flag parsing, command dispatch, presentation,
// and exit codes. Anything worth testing lives in a subpackage
// (docs/CODING_STANDARDS.md, "Module boundaries").
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/shrigum/adamic/src/settings"
	"github.com/shrigum/adamic/src/update"
)

// version is injected at build time via -ldflags "-X main.version=..."
// (scripts/build.sh). The git tag is the single source of truth.
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "config" {
		return runConfig(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "update" {
		return runUpdate(stdout, stderr)
	}

	fs := flag.NewFlagSet("adamic", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "world", "name to greet")
	greeting := fs.String("greeting", "", "override the configured greeting for this run only")
	showVersion := fs.Bool("version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}

	word := *greeting
	if word == "" {
		s, err := settings.Load()
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		word = s["greeting"]
	}
	fmt.Fprintf(stdout, "%s, %s!\n", word, *name)
	return 0
}

// runUpdate performs the explicit, opt-in check for a newer release
// (ADR-0003). It is the only code path in the app that touches the network.
func runUpdate(stdout, stderr io.Writer) int {
	res, err := update.Check(version)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	switch {
	case res.Current == "dev":
		fmt.Fprintf(stdout, "you are running a development build; the latest release is v%s\n%s\n", res.Latest, res.URL)
	case res.Newer:
		fmt.Fprintf(stdout, "update available: v%s (you have v%s)\n%s\n", res.Latest, res.Current, res.URL)
	default:
		fmt.Fprintf(stdout, "you are on the latest release (v%s)\n", res.Current)
	}
	return 0
}

func runConfig(args []string, stdout, stderr io.Writer) int {
	usage := "usage: adamic config <get <key> | set <key> <value> | list | path>"
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	switch args[0] {
	case "get":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "usage: adamic config get <key>")
			return 2
		}
		s, err := settings.Load()
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		v, ok := s[args[1]]
		if !ok {
			fmt.Fprintf(stderr, "error: setting %q is not set and has no default (see `adamic config list`)\n", args[1])
			return 1
		}
		fmt.Fprintln(stdout, v)
		return 0

	case "set":
		if len(args) != 3 {
			fmt.Fprintln(stderr, "usage: adamic config set <key> <value>")
			return 2
		}
		if err := settings.Set(args[1], args[2]); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		return 0

	case "list":
		s, err := settings.Load()
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		keys := make([]string, 0, len(s))
		for k := range s {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(stdout, "%s=%s\n", k, s[k])
		}
		return 0

	case "path":
		p, err := settings.Path()
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		fmt.Fprintln(stdout, p)
		return 0

	default:
		fmt.Fprintln(stderr, usage)
		return 2
	}
}
