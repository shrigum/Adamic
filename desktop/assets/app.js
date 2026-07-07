// @ts-check
// UI controller for the Adamic reader (tasks T7, T9). Wires the tested,
// framework-agnostic ViewerModel to the DOM and to the Go core over Wails.
// It renders pages as <img> data URLs the engine produces, handles page
// navigation, zoom/fit, thumbnails, and reading-position save/restore.

import { ViewerModel } from './viewer.js';
import { readerBinding, choosePDF } from './wailsClient.js';

/** @type {ViewerModel|null} */
let model = null;
/** @type {string|null} */
let docId = null;
/** Debounce handle for saving reading position. */
let saveTimer = 0;

const el = {
  open: /** @type {HTMLButtonElement} */ (document.getElementById('open')),
  pages: /** @type {HTMLDivElement} */ (document.getElementById('pages')),
  thumbs: /** @type {HTMLDivElement} */ (document.getElementById('thumbs')),
  status: /** @type {HTMLDivElement} */ (document.getElementById('status')),
  pageInput: /** @type {HTMLInputElement} */ (document.getElementById('pageInput')),
  pageCount: /** @type {HTMLSpanElement} */ (document.getElementById('pageCount')),
  prev: /** @type {HTMLButtonElement} */ (document.getElementById('prev')),
  next: /** @type {HTMLButtonElement} */ (document.getElementById('next')),
  zoomIn: /** @type {HTMLButtonElement} */ (document.getElementById('zoomIn')),
  zoomOut: /** @type {HTMLButtonElement} */ (document.getElementById('zoomOut')),
  fitWidth: /** @type {HTMLButtonElement} */ (document.getElementById('fitWidth')),
  fitPage: /** @type {HTMLButtonElement} */ (document.getElementById('fitPage')),
  zoomLabel: /** @type {HTMLSpanElement} */ (document.getElementById('zoomLabel')),
};

function setStatus(msg, isError = false) {
  el.status.textContent = msg;
  el.status.classList.toggle('error', isError);
}

async function openPath(path) {
  setStatus('Opening…');
  const res = await readerBinding.Open(path);
  if (!res.ok) {
    setStatus(res.error ? res.error.message : 'Could not open the document.', true);
    return;
  }
  docId = res.doc.id;
  model = new ViewerModel(res.doc.pages, res.doc.position);
  model.setViewport(el.pages.clientWidth - 40, el.pages.clientHeight - 40);
  model.applyFit('width');

  el.pageCount.textContent = 'of ' + model.pageCount;
  el.pageInput.max = String(model.pageCount);
  setStatus(fileName(path));
  await renderAll();
  await buildThumbnails();
  // Jump to the restored page.
  scrollToPage(model.page);
  updateChrome();
}

/** Render every page's <img> shell; the visible ones get real images, the rest
 * are lazy (filled on scroll). For a first cut we render all pages' images —
 * the engine's own render window bounds cost server-side; here we lazily fill
 * as they scroll into view via IntersectionObserver. */
async function renderAll() {
  el.pages.innerHTML = '';
  const io = new IntersectionObserver(onPageVisible, { root: el.pages, rootMargin: '400px' });
  for (let i = 0; i < model.pageCount; i++) {
    const holder = document.createElement('div');
    holder.className = 'page';
    holder.dataset.page = String(i);
    const size = model.pages[i];
    holder.style.aspectRatio = `${size.widthPt} / ${size.heightPt}`;
    el.pages.appendChild(holder);
    io.observe(holder);
  }
}

/** @param {IntersectionObserverEntry[]} entries */
async function onPageVisible(entries) {
  for (const entry of entries) {
    if (!entry.isIntersecting) continue;
    const holder = /** @type {HTMLElement} */ (entry.target);
    if (holder.dataset.loaded) continue;
    holder.dataset.loaded = '1';
    const page = Number(holder.dataset.page);
    try {
      const url = await readerBinding.RenderPage(docId, page, model.zoom);
      const img = new Image();
      img.src = url;
      img.alt = 'Page ' + (page + 1);
      holder.appendChild(img);
    } catch (e) {
      holder.textContent = 'Failed to render page ' + (page + 1);
    }
    // Track the current page as the top-most visible one, and save position.
    reportVisible();
  }
}

let scrollRaf = 0;
function reportVisible() {
  if (scrollRaf) return;
  scrollRaf = requestAnimationFrame(() => {
    scrollRaf = 0;
    if (!model) return;
    const top = topVisiblePage();
    if (top >= 0 && top !== model.page) {
      model.page = top;
      updateChrome();
    }
    schedulePositionSave();
  });
}

/** @returns {number} index of the topmost page whose top is at/above viewport top */
function topVisiblePage() {
  const holders = el.pages.querySelectorAll('.page');
  const rootTop = el.pages.getBoundingClientRect().top;
  let best = 0;
  for (const h of holders) {
    const r = h.getBoundingClientRect();
    if (r.bottom - rootTop > 0) {
      best = Number(/** @type {HTMLElement} */ (h).dataset.page);
      break;
    }
  }
  return best;
}

function scrollToPage(page) {
  const holder = el.pages.querySelector(`.page[data-page="${page}"]`);
  if (holder) holder.scrollIntoView({ block: 'start' });
}

async function buildThumbnails() {
  el.thumbs.innerHTML = '';
  for (let i = 0; i < model.pageCount; i++) {
    const t = document.createElement('div');
    t.className = 'thumb';
    t.title = 'Page ' + (i + 1);
    const label = document.createElement('span');
    label.textContent = String(i + 1);
    t.appendChild(label);
    t.addEventListener('click', () => {
      if (model.goToPage(i)) {
        scrollToPage(i);
        updateChrome();
      }
    });
    el.thumbs.appendChild(t);
    // Lazily fill the thumbnail image.
    readerBinding.Thumbnail(docId, i).then((url) => {
      const img = new Image();
      img.src = url;
      t.insertBefore(img, label);
    }).catch(() => {});
  }
}

/** Re-render all currently-loaded pages at the new zoom. */
async function rerenderForZoom() {
  const holders = el.pages.querySelectorAll('.page[data-loaded]');
  for (const h of holders) {
    const page = Number(/** @type {HTMLElement} */ (h).dataset.page);
    try {
      const url = await readerBinding.RenderPage(docId, page, model.zoom);
      const img = h.querySelector('img');
      if (img) img.src = url;
    } catch (e) { /* leave the old image */ }
  }
  updateChrome();
}

function updateChrome() {
  if (!model) return;
  el.pageInput.value = String(model.page + 1);
  el.zoomLabel.textContent = Math.round(model.zoom * 100) + '%';
  el.prev.disabled = model.page <= 0;
  el.next.disabled = model.page >= model.pageCount - 1;
}

function schedulePositionSave() {
  if (!docId || !model) return;
  clearTimeout(saveTimer);
  saveTimer = setTimeout(() => {
    readerBinding.SetPosition(docId, model.page, model.offsetY).catch(() => {});
  }, 400);
}

function fileName(path) {
  const parts = path.split(/[\\/]/);
  return parts[parts.length - 1] || path;
}

// --- Wire controls (T9) ---

el.open.addEventListener('click', async () => {
  try {
    const path = await choosePDF();
    if (path) await openPath(path);
  } catch (e) {
    setStatus('Open dialog failed: ' + e, true);
  }
});

el.prev.addEventListener('click', () => {
  if (model && model.prevPage()) { scrollToPage(model.page); updateChrome(); schedulePositionSave(); }
});
el.next.addEventListener('click', () => {
  if (model && model.nextPage()) { scrollToPage(model.page); updateChrome(); schedulePositionSave(); }
});
el.pageInput.addEventListener('change', () => {
  if (!model) return;
  const resolved = model.resolveUserPage(el.pageInput.value);
  if (resolved === null) {
    setStatus(`Page must be between 1 and ${model.pageCount}.`, true);
    updateChrome();
    return;
  }
  if (model.goToPage(resolved)) { scrollToPage(resolved); }
  setStatus(fileName(el.status.textContent || ''));
  updateChrome();
});

el.zoomIn.addEventListener('click', () => { if (model) { model.zoomIn(); rerenderForZoom(); } });
el.zoomOut.addEventListener('click', () => { if (model) { model.zoomOut(); rerenderForZoom(); } });
el.fitWidth.addEventListener('click', () => {
  if (!model) return;
  model.setViewport(el.pages.clientWidth - 40, el.pages.clientHeight - 40);
  model.applyFit('width');
  rerenderForZoom();
});
el.fitPage.addEventListener('click', () => {
  if (!model) return;
  model.setViewport(el.pages.clientWidth - 40, el.pages.clientHeight - 40);
  model.applyFit('page');
  rerenderForZoom();
});

el.pages.addEventListener('scroll', reportVisible);

window.addEventListener('resize', () => {
  if (!model || model.fit === 'custom') return;
  model.setViewport(el.pages.clientWidth - 40, el.pages.clientHeight - 40);
  model.applyFit(model.fit);
  rerenderForZoom();
});

setStatus('Open a PDF to begin.');
