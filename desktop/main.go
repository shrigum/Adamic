// Command adamic-desktop is the Wails v3 desktop shell for the Adamic PDF
// reader (task T6, ADR-0005). It is wiring only: it starts the PDFium-backed
// Document Engine, binds the transport-agnostic frontend surface (package app)
// plus a tiny desktop-only service for the native file-open dialog, serves the
// embedded frontend assets, and opens a window. All reader logic lives in the
// importable packages (document, reader, library, app); nothing here knows about
// PDFs.
package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/shrigum/adamic/src/app"
	"github.com/shrigum/adamic/src/document"
	"github.com/shrigum/adamic/src/ocr/run"
	"github.com/shrigum/adamic/src/ocr/store"
	"github.com/shrigum/adamic/src/ocr/tesseract"
)

//go:embed all:assets
var assets embed.FS

// fatal records a startup failure to a log file next to the executable and
// exits. In a windowed (-H windowsgui) build there is no console, so a plain
// log.Fatal would kill the app with no visible reason — the classic "nothing
// happens on double-click". Writing the reason to adamic-error.log makes every
// startup failure diagnosable.
func fatal(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if exe, err := os.Executable(); err == nil {
		logPath := filepath.Join(filepath.Dir(exe), "adamic-error.log")
		_ = os.WriteFile(logPath, []byte(msg+"\n"), 0o644)
	}
	log.Fatal(msg)
}

func main() {
	// A panic in a windowed build is otherwise silent; capture it too.
	defer func() {
		if r := recover(); r != nil {
			fatal("panic: %v\n%s", r, debug.Stack())
		}
	}()

	// In a windowed (-H windowsgui) build the process has no console, so
	// os.Stdout/os.Stderr are invalid handles. The PDFium WebAssembly runtime
	// (wazero) probes those handles when it instantiates and fails with
	// "GetFileType /dev/stdout: The handle is invalid" — which killed the app
	// silently on double-click. Point both at NUL so the handles are valid.
	ensureStdHandles()

	engine, err := document.NewEngine()
	if err != nil {
		fatal("start document engine: %v", err)
	}
	defer engine.Shutdown()

	reader := app.New(engine)
	desktop := &Desktop{}

	// OCR is optional equipment: with no Tesseract on the system the reader
	// runs fine and the OCR commands report recognition as unavailable
	// (package app's soft error). The Dutch model is the MVP language
	// (ADR-0013); the engine pick is ADR-0014.
	recognizer, ocrErr := tesseract.Find("nld")
	var runner *run.Runner
	if ocrErr == nil {
		runner = run.NewRunner(engine, recognizer, store.FileStore{})
		defer runner.Close()
	} else {
		log.Printf("OCR unavailable: %v", ocrErr)
	}

	wailsApp := application.New(application.Options{
		Name:        "Adamic",
		Description: "A local-first PDF reader with a language-learning layer.",
		Services: []application.Service{
			application.NewService(reader),
			application.NewService(desktop),
		},
		Assets: application.AssetOptions{
			// BundledAssetFileServer (not AssetFileServerFS) also serves the
			// Wails runtime module at /wails/runtime.js, which the frontend
			// imports to get Call.ByName for the bindings.
			Handler: application.BundledAssetFileServer(assets),
		},
	})

	// The dialog service needs the application handle to attach the native
	// picker to the window; hand it over now that the app exists. Same for
	// the OCR event emitter: progress flows through Wails events.
	desktop.app = wailsApp
	if runner != nil {
		reader.EnableOCR(engine, runner, func(name string, data any) {
			wailsApp.Event.Emit(name, data)
		})
	}

	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Adamic",
		Width:  1100,
		Height: 850,
	})

	if err := wailsApp.Run(); err != nil {
		fatal("run: %v", err)
	}
}
