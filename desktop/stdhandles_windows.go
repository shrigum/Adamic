package main

import (
	"os"
	"syscall"
)

// ensureStdHandles makes os.Stdout and os.Stderr valid file handles when the
// process was started without a console (a -H windowsgui build launched from
// Explorer). In that case the original handles are invalid, and any library
// that probes them — notably the wazero WebAssembly runtime backing PDFium,
// which calls GetFileType on /dev/stdout during module instantiation — fails.
// Reopening both to NUL gives them well-defined, harmless sinks.
func ensureStdHandles() {
	if !stdHandleValid(os.Stdout) {
		if nul, err := os.OpenFile("NUL", os.O_WRONLY, 0); err == nil {
			os.Stdout = nul
		}
	}
	if !stdHandleValid(os.Stderr) {
		if nul, err := os.OpenFile("NUL", os.O_WRONLY, 0); err == nil {
			os.Stderr = nul
		}
	}
}

// stdHandleValid reports whether f wraps a usable OS handle. A windowed process
// has a zero/invalid handle for the standard streams; GetFileType returns
// FILE_TYPE_UNKNOWN with a non-nil error for those.
func stdHandleValid(f *os.File) bool {
	if f == nil {
		return false
	}
	h := syscall.Handle(f.Fd())
	if h == syscall.InvalidHandle || h == 0 {
		return false
	}
	ft, err := syscall.GetFileType(h)
	if err != nil {
		return false
	}
	return ft != syscall.FILE_TYPE_UNKNOWN
}
