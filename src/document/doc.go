// Package document is Adamic's Document Engine: it opens PDF files and renders
// their pages to images for the frontend, wrapping the PDFium engine
// (github.com/klippa-app/go-pdfium, per ADR-0012) so that no other package
// imports the binding directly.
//
// The engine is used through the pdf-reader-core command contract (see the
// command interface in package cmd, task T1). This file currently carries only
// the package doc and the spike (engine_spike_test.go), which proves the C1
// gate from the design review: one real PDF page renders via the wasm/purego
// backend with no cgo, on every target platform. Real engine methods (open,
// page count, render at scale — tasks T3/T4) land on top of this once C1 is
// green.
//
// Failure modes (owned here per CODING_STANDARDS.md): opening a missing,
// non-PDF, corrupt, or password-protected file returns a typed error and never
// panics; a render failure on an otherwise-valid document is likewise an error,
// not a crash. These are normalized into the command contract's error shape in
// task T13.
package document
