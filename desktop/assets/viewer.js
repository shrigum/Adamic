// @ts-check
// Viewer model for the Adamic PDF reader (tasks T7, T9). This is the
// framework-agnostic navigation/zoom/scroll logic that the Wails frontend
// drives — deliberately DOM-free and transport-free so it is unit-testable
// under `node --test` today and portable to whatever view layer the desktop
// shell uses. It holds no rendering itself; it decides *what* to render (which
// pages are visible) and *at what scale*, and the view layer asks the Go core
// (package app, over the Wails boundary) for the actual page images.
//
// Coordinates match the core: pages are zero-based; sizes are in points; fit
// modes are computed from page geometry and the viewport, mirroring
// reader.Scale.PixelSize on the Go side so the frontend and core agree on size.

/** @typedef {{ widthPt: number, heightPt: number }} PageSize */
/** @typedef {'single' | 'continuous'} ViewMode */
/** @typedef {'width' | 'page' | 'custom'} FitMode */

/** Discrete zoom stops (50%–400%), spec A5. */
export const ZOOM_STOPS = [0.5, 0.75, 1.0, 1.25, 1.5, 2.0, 3.0, 4.0];

export const MIN_ZOOM = ZOOM_STOPS[0];
export const MAX_ZOOM = ZOOM_STOPS[ZOOM_STOPS.length - 1];

/** How many off-screen pages to keep rendered on each side (matches the core
 * render window's look-ahead intent). */
export const LOOK_AHEAD = 2;

export class ViewerModel {
  /**
   * @param {PageSize[]} pages  per-page intrinsic sizes in points
   * @param {{ page?: number, offsetY?: number }} [position]  restored position
   */
  constructor(pages, position = {}) {
    if (!Array.isArray(pages) || pages.length === 0) {
      throw new Error('ViewerModel requires at least one page');
    }
    /** @type {PageSize[]} */
    this.pages = pages;
    /** @type {ViewMode} */
    this.mode = 'continuous';
    /** @type {FitMode} */
    this.fit = 'width';
    /** @type {number} current zoom multiplier (1.0 = 100%) */
    this.zoom = 1.0;
    /** @type {number} current page (zero-based) */
    this.page = clamp(position.page ?? 0, 0, pages.length - 1);
    /** @type {number} within-page scroll offset, fraction of page height */
    this.offsetY = position.offsetY ?? 0;
    /** @type {{ widthPx: number, heightPx: number }} */
    this.viewport = { widthPx: 0, heightPx: 0 };
  }

  get pageCount() {
    return this.pages.length;
  }

  /** Current reading position, for persisting via app.SetPosition. */
  get position() {
    return { page: this.page, offsetY: this.offsetY };
  }

  // --- Navigation (T9) ---

  /** Move to the next page, clamped at the last (no wrap). Returns true if the
   * page changed. */
  nextPage() {
    return this.goToPage(this.page + 1);
  }

  /** Move to the previous page, clamped at the first (no wrap). */
  prevPage() {
    return this.goToPage(this.page - 1);
  }

  /**
   * Navigate to a zero-based page index. Out-of-range requests are rejected
   * (no navigation) so a caller can surface a message (spec AC5). Returns true
   * if the current page changed.
   * @param {number} page
   */
  goToPage(page) {
    if (!Number.isInteger(page) || page < 0 || page >= this.pageCount) {
      return false;
    }
    if (page === this.page) return false;
    this.page = page;
    this.offsetY = 0; // a deliberate jump lands at the top of the target page
    return true;
  }

  /**
   * Validate a human "go to page N" entry, where N is 1-based (what the user
   * types). Returns the resolved zero-based index, or null if invalid — the
   * caller shows an error for null (spec AC5).
   * @param {number|string} oneBased
   * @returns {number|null}
   */
  resolveUserPage(oneBased) {
    const n = typeof oneBased === 'string' ? Number(oneBased.trim()) : oneBased;
    if (!Number.isInteger(n) || n < 1 || n > this.pageCount) return null;
    return n - 1;
  }

  // --- Zoom & fit (T9, spec AC4) ---

  /** @param {ViewMode} mode */
  setMode(mode) {
    this.mode = mode;
  }

  /**
   * Set the viewport (device pixels) and recompute the zoom if a fit mode is
   * active — this is what makes fit-to-width/page track window resizes (AC4).
   * @param {number} widthPx
   * @param {number} heightPx
   */
  setViewport(widthPx, heightPx) {
    this.viewport = { widthPx, heightPx };
    if (this.fit !== 'custom') this.applyFit(this.fit);
  }

  /**
   * Apply a fit mode, computing the zoom from the current page's geometry and
   * viewport. 'custom' leaves the explicit zoom untouched.
   * @param {FitMode} fit
   */
  applyFit(fit) {
    this.fit = fit;
    if (fit === 'custom') return;
    const p = this.pages[this.page];
    const { widthPx, heightPx } = this.viewport;
    if (widthPx <= 0) return; // no viewport yet; zoom stays until one is set
    if (fit === 'width') {
      this.zoom = widthPx / p.widthPt;
    } else {
      // fit whole page: bounded by the tighter dimension
      this.zoom = Math.min(widthPx / p.widthPt, heightPx / p.heightPt);
    }
  }

  /**
   * Set an explicit zoom (switches fit to 'custom'), clamped to [MIN,MAX].
   * @param {number} zoom
   */
  setZoom(zoom) {
    this.fit = 'custom';
    this.zoom = clamp(zoom, MIN_ZOOM, MAX_ZOOM);
  }

  /** Zoom in to the next discrete stop above the current zoom. */
  zoomIn() {
    const next = ZOOM_STOPS.find((z) => z > this.zoom + 1e-9);
    this.setZoom(next ?? MAX_ZOOM);
  }

  /** Zoom out to the next discrete stop below the current zoom. */
  zoomOut() {
    const below = ZOOM_STOPS.filter((z) => z < this.zoom - 1e-9);
    this.setZoom(below.length ? below[below.length - 1] : MIN_ZOOM);
  }

  // --- Virtualized window (T7) ---

  /**
   * The inclusive [first, last] page range to keep rendered. In single-page
   * mode that is just the current page (plus look-ahead for snappy paging); in
   * continuous mode it is the visible span the caller reports plus look-ahead.
   * The caller passes the currently visible span in continuous mode; single
   * mode ignores it. Result is clamped to the document.
   * @param {number} [firstVisible]
   * @param {number} [lastVisible]
   * @returns {{ first: number, last: number }}
   */
  renderRange(firstVisible, lastVisible) {
    let first;
    let last;
    if (this.mode === 'single') {
      first = this.page;
      last = this.page;
    } else {
      first = firstVisible ?? this.page;
      last = lastVisible ?? this.page;
    }
    first = clamp(first - LOOK_AHEAD, 0, this.pageCount - 1);
    last = clamp(last + LOOK_AHEAD, 0, this.pageCount - 1);
    return { first, last };
  }
}

/**
 * @param {number} v @param {number} lo @param {number} hi
 * @returns {number}
 */
export function clamp(v, lo, hi) {
  if (hi < lo) return lo;
  if (v < lo) return lo;
  if (v > hi) return hi;
  return v;
}
