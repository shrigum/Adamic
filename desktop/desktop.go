package main

import "github.com/wailsapp/wails/v3/pkg/application"

// Desktop is the desktop-only service the frontend calls for things that only
// make sense with a native window — currently just the file-open dialog. It is
// kept out of package app (which stays transport-agnostic) because it depends
// on the Wails application handle.
type Desktop struct {
	app *application.App
}

// ChoosePDF shows a native open-file dialog filtered to PDFs and returns the
// chosen absolute path, or "" if the user cancelled. The frontend then passes
// the path to App.Open.
func (d *Desktop) ChoosePDF() (string, error) {
	path, err := d.app.Dialog.OpenFile().
		CanChooseFiles(true).
		AddFilter("PDF documents", "*.pdf").
		PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	return path, nil
}
