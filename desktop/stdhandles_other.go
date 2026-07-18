//go:build !windows

package main

// ensureStdHandles is a no-op outside Windows: only a -H windowsgui build
// launched without a console has invalid standard handles (see
// stdhandles_windows.go); on other platforms the streams are always usable.
func ensureStdHandles() {}
