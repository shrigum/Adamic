// @ts-check
// Adapts the Wails-generated service bindings to the small surface the UI
// controller (app.js) uses. The bindings under ./bindings/ are produced by
// `wails3 generate bindings` and call Go by numeric method ID over the runtime
// module at /wails/runtime.js — the supported call path. This file is the only
// place that knows the generated import locations.

import * as App from './bindings/github.com/shrigum/adamic/src/app/app.js';
import * as Desktop from './bindings/github.com/shrigum/adamic/desktop/desktop.js';
import { Events } from '/wails/runtime.js';

export const readerBinding = {
  /** @param {string} path */
  Open: (path) => App.Open(path),
  /** @param {string} id @param {number} page @param {number} zoom */
  RenderPage: (id, page, zoom) => App.RenderPage(id, page, zoom),
  /** @param {string} id @param {number} page @param {number} vw @param {number} vh @param {boolean} fitPage */
  RenderPageFit: (id, page, vw, vh, fitPage) => App.RenderPageFit(id, page, vw, vh, fitPage),
  /** @param {string} id @param {number} page */
  Thumbnail: (id, page) => App.Thumbnail(id, page),
  /** @param {string} id @param {number} page @param {number} offsetY */
  SetPosition: (id, page, offsetY) => App.SetPosition(id, page, offsetY),
  /** @param {string} id */
  GetPosition: (id) => App.GetPosition(id),
  /** @param {string} id */
  Close: (id) => App.Close(id),
};

/** Show the native PDF picker; returns the chosen path or "". */
export function choosePDF() {
  return Desktop.ChoosePDF();
}

/** The OCR command surface (ocr T11/T12). */
export const ocrBinding = {
  /** @param {string} id */
  Candidates: (id) => App.OCRCandidates(id),
  /** @param {string} id */
  Start: (id) => App.OCRStart(id),
  Cancel: () => App.OCRCancel(),
  /** @param {string} id @param {number} page */
  RecognizePage: (id, page) => App.OCRRecognizePage(id, page),
  /** @param {string} id */
  Result: (id) => App.OCRResult(id),
  /** @param {string} id @param {number} page @param {number} unit @param {string} text */
  Correct: (id, page, unit, text) => App.OCRCorrect(id, page, unit, text),
  /** @param {string} id @param {number} page @param {number} unit */
  Revert: (id, page, unit) => App.OCRRevert(id, page, unit),
};

/** Event names emitted by the Go core during OCR runs (package app). */
export const OCR_EVENTS = {
  progress: 'ocr:progress',
  done: 'ocr:done',
  pageDone: 'ocr:pageDone',
};

/** Subscribe to an OCR event; the handler receives the DTO payload.
 * @param {string} name @param {(data: any) => void} handler */
export function onOCREvent(name, handler) {
  return Events.On(name, (ev) => {
    const d = ev && ev.data;
    // The runtime wraps variadic Emit payloads in an array; unwrap ours.
    handler(Array.isArray(d) && d.length === 1 ? d[0] : d);
  });
}
