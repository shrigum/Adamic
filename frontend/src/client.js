// @ts-check
// Typed client for the Go core (package app) over the Wails boundary. Wails v3
// generates JS bindings for the bound *App methods at build time; this module
// defines the shape the frontend codes against and adapts whatever the runtime
// exposes to it, so the view layer never depends on Wails-generated symbol
// names directly. Swap `binding` for the generated module in the desktop build;
// pass a fake in tests.

/**
 * @typedef {object} PageSizeDTO
 * @property {number} widthPt
 * @property {number} heightPt
 */
/**
 * @typedef {object} PositionDTO
 * @property {number} page
 * @property {number} offsetY
 */
/**
 * @typedef {object} DocumentDTO
 * @property {string} id
 * @property {string} path
 * @property {PageSizeDTO[]} pages
 * @property {PositionDTO} position
 */
/**
 * @typedef {object} OpenErrDTO
 * @property {string} kind    machine-readable: not-found|not-pdf|corrupt|password|error
 * @property {string} message human-facing text to display
 */
/**
 * @typedef {object} OpenResult
 * @property {boolean} ok
 * @property {DocumentDTO} [doc]
 * @property {OpenErrDTO} [error]
 */

/**
 * The set of methods package app exposes. The generated Wails binding must
 * satisfy this shape; see src/app/app.go for the Go side.
 * @typedef {object} ReaderBinding
 * @property {(path: string) => Promise<OpenResult>} Open
 * @property {(id: string, page: number, zoom: number) => Promise<string>} RenderPage
 * @property {(id: string, page: number, vw: number, vh: number, fitPage: boolean) => Promise<string>} RenderPageFit
 * @property {(id: string, page: number) => Promise<string>} Thumbnail
 * @property {(id: string, page: number, offsetY: number) => Promise<void>} SetPosition
 * @property {(id: string) => Promise<PositionDTO>} GetPosition
 * @property {(id: string) => Promise<void>} Close
 */

/**
 * ReaderClient is a thin wrapper over the binding. It exists so the view layer
 * has one stable import regardless of how Wails names things, and so tests can
 * inject a fake binding.
 */
export class ReaderClient {
  /** @param {ReaderBinding} binding */
  constructor(binding) {
    /** @type {ReaderBinding} */
    this.binding = binding;
  }

  /** @param {string} path @returns {Promise<OpenResult>} */
  open(path) {
    return this.binding.Open(path);
  }

  /** @param {string} id @param {number} page @param {number} zoom */
  renderPage(id, page, zoom) {
    return this.binding.RenderPage(id, page, zoom);
  }

  /**
   * @param {string} id @param {number} page
   * @param {number} vw @param {number} vh @param {boolean} fitPage
   */
  renderPageFit(id, page, vw, vh, fitPage) {
    return this.binding.RenderPageFit(id, page, vw, vh, fitPage);
  }

  /** @param {string} id @param {number} page */
  thumbnail(id, page) {
    return this.binding.Thumbnail(id, page);
  }

  /** @param {string} id @param {number} page @param {number} offsetY */
  setPosition(id, page, offsetY) {
    return this.binding.SetPosition(id, page, offsetY);
  }

  /** @param {string} id */
  getPosition(id) {
    return this.binding.GetPosition(id);
  }

  /** @param {string} id */
  close(id) {
    return this.binding.Close(id);
  }
}
