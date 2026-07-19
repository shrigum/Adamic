// @ts-check
// OCR review model (ocr feature task T12). Framework-agnostic state for the
// "Recognize text" flow — trigger, progress, cancel — and the per-region
// review overlay: which units to show on a page, and the geometry that fits
// a unit's text into its page-point box in BOTH dimensions (spec A14, AC15)
// so the displayed word coincides with the printed word at any zoom. DOM-free
// and transport-free, unit-tested under `node --test`; the view layer feeds
// it the DTOs and events from the Go core (package app) and asks it what to
// draw. Scope is per-region view + inline correction only (design-review C6
// guard): no bulk find/replace, no document re-flow.

/** @typedef {{ x: number, y: number, w: number, h: number }} Box */
/** @typedef {{ text: string, engineText: string, corrected: boolean, box: Box, confidence: number, group?: string }} UnitDTO */
/** @typedef {{ page: number, units?: UnitDTO[], failure?: { kind: string, message: string } }} PageDTO */
/** @typedef {{ hasResult: boolean, pages?: PageDTO[], error?: string }} ResultDTO */

/** Units below this confidence are visually flagged for review (the engine
 * reports ~0.93–0.97 on clean fixture print; broken reads sit far lower). */
export const LOW_CONFIDENCE = 0.7;

/** Fraction of the box height used as the font size before width fitting —
 * glyph ascenders/descenders overflow the em box, so a full-height font
 * visually spills out of the printed word's rectangle. */
export const FONT_FIT = 0.8;

export class OCRReviewModel {
  constructor() {
    /** @type {'unavailable'|'idle'|'running'|'done'|'cancelled'|'failed'} */
    this.status = 'idle';
    /** @type {number[]} zero-based pages needing OCR (spec A3) */
    this.candidates = [];
    /** @type {ResultDTO|null} the stored result, corrections applied */
    this.result = null;
    this.pagesDone = 0;
    this.pageCount = 0;
    this.failures = 0;
    /** Whether the review overlay is shown. */
    this.reviewing = false;
    /** @type {string} user-facing error from the last run, if any */
    this.error = '';
  }

  /** @param {number[]} pages */
  setCandidates(pages) {
    this.candidates = pages || [];
  }

  /** @param {ResultDTO|null} result */
  setResult(result) {
    this.result = result && result.hasResult ? result : null;
  }

  /** The "Recognize text" affordance applies only when there is something to
   * recognize and no run is in flight (spec A5: explicit trigger, never
   * automatic). */
  get canRecognize() {
    return this.status !== 'running' && this.status !== 'unavailable' && this.candidates.length > 0;
  }

  /** Review is offered once a result exists (AC6). */
  get canReview() {
    return this.result !== null && this.status !== 'running';
  }

  started() {
    this.status = 'running';
    this.pagesDone = 0;
    this.failures = 0;
    this.error = '';
  }

  /** @param {{ page: number, pageCount: number, failed: boolean }} p */
  onProgress(p) {
    this.status = 'running';
    this.pagesDone += 1;
    this.pageCount = p.pageCount;
    if (p.failed) this.failures += 1;
  }

  /** @param {{ ok: boolean, cancelled: boolean, pages: number, error?: string }} d */
  onDone(d) {
    if (d.ok) this.status = 'done';
    else if (d.cancelled) this.status = 'cancelled';
    else this.status = 'failed';
    this.error = d.error || '';
  }

  /** One line for the status area. */
  statusText() {
    switch (this.status) {
      case 'running': {
        const of = this.pageCount ? ` ${this.pagesDone}/${this.pageCount}` : '';
        const failed = this.failures ? ` (${this.failures} failed)` : '';
        return `Recognizing…${of}${failed}`;
      }
      case 'done':
        return this.failures ? `Text recognized; ${this.failures} page(s) failed.` : 'Text recognized.';
      case 'cancelled':
        return 'Recognition cancelled — finished pages are kept.';
      case 'failed':
        return this.error || 'Recognition failed.';
      default:
        return '';
    }
  }

  /** Units to draw on a page's overlay ([] when none/failed). @param {number} page */
  unitsForPage(page) {
    if (!this.result || !this.result.pages) return [];
    const pr = this.result.pages.find((p) => p.page === page);
    return (pr && pr.units) || [];
  }

  /** The page's typed failure, if it has one. @param {number} page */
  failureForPage(page) {
    if (!this.result || !this.result.pages) return null;
    const pr = this.result.pages.find((p) => p.page === page);
    return (pr && pr.failure) || null;
  }
}

/**
 * A unit's overlay rectangle as percentages of the page, computed from
 * page-point coordinates. Percent geometry is zoom-independent: the overlay
 * container matches the rendered page element, so the box coincides with the
 * printed word at any zoom (AC15, spec A14 — this box is the hit target).
 * @param {Box} box  unit box in page points
 * @param {{ widthPt: number, heightPt: number }} pageSize
 */
export function unitRectPct(box, pageSize) {
  return {
    left: (box.x / pageSize.widthPt) * 100,
    top: (box.y / pageSize.heightPt) * 100,
    width: (box.w / pageSize.widthPt) * 100,
    height: (box.h / pageSize.heightPt) * 100,
  };
}

/**
 * Fit rendered text into its box in both dimensions (AC15): the font size is
 * set from the box's displayed height, and the horizontal scale squeezes or
 * stretches the measured text run to exactly the box's displayed width.
 * @param {number} boxWpx  box width as displayed, in px
 * @param {number} boxHpx  box height as displayed, in px
 * @param {number} measuredWpx  the text's natural width at fontSizePx(boxHpx)
 * @returns {{ fontSize: number, scaleX: number }}
 */
export function fitText(boxWpx, boxHpx, measuredWpx) {
  const fontSize = fontSizePx(boxHpx);
  const scaleX = measuredWpx > 0 ? boxWpx / measuredWpx : 1;
  return { fontSize, scaleX };
}

/** The font size used inside a box of the given displayed height. @param {number} boxHpx */
export function fontSizePx(boxHpx) {
  return Math.max(4, boxHpx * FONT_FIT);
}

/** Whether a unit should be flagged for review. @param {UnitDTO} u */
export function needsAttention(u) {
  return !u.corrected && u.confidence < LOW_CONFIDENCE;
}
