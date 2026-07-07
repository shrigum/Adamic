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
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/shrigum/adamic/src/app"
	"github.com/shrigum/adamic/src/document"
)

//go:embed all:assets
var assets embed.FS

func main() {
	engine, err := document.NewEngine()
	if err != nil {
		log.Fatalf("start document engine: %v", err)
	}
	defer engine.Shutdown()

	reader := app.New(engine)
	desktop := &Desktop{}

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
	// picker to the window; hand it over now that the app exists.
	desktop.app = wailsApp

	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Adamic",
		Width:  1100,
		Height: 850,
	})

	if err := wailsApp.Run(); err != nil {
		log.Fatalf("run: %v", err)
	}
}
