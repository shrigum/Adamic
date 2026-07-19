// @ts-check
// UI controller for the Adamic reader (tasks T7, T9). Wires the tested,
// framework-agnostic ViewerModel to the DOM and to the Go core over Wails.
// It renders pages as <img> data URLs the engine produces, handles page
// navigation, zoom/fit, thumbnails, and reading-position save/restore.

import { ViewerModel } from './viewer.js';
import { OCRReviewModel, unitRectPct, fitText, needsAttention } from './ocrReview.js';
import { readerBinding, choosePDF, ocrBinding, onOCREvent, OCR_EVENTS } from './wailsClient.js';

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
  ocrAction: /** @type {HTMLButtonElement} */ (document.getElementById('ocrAction')),
  ocrCancel: /** @type {HTMLButtonElement} */ (document.getElementById('ocrCancel')),
  ocrReview: /** @type {HTMLButtonElement} */ (document.getElementById('ocrReview')),
};

const ocrModel = new OCRReviewModel();

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
  await refreshOCR();
}

// --- OCR: trigger, progress, per-region review (ocr T12) ---

/** Refresh candidates + stored result for the open document (never runs
 * recognition — reading is cached, spec AC4). OCR being unavailable (no
 * engine on this system) is soft: the reader works, the buttons stay hidden. */
async function refreshOCR() {
  ocrModel.reviewing = false;
  try {
    ocrModel.setCandidates(await ocrBinding.Candidates(docId));
    await refreshOCRResult();
    ocrModel.status = 'idle';
  } catch (e) {
    ocrModel.status = 'unavailable';
    ocrModel.setCandidates([]);
    ocrModel.setResult(null);
  }
  redrawOCROverlays();
  updateOCRChrome();
}

async function refreshOCRResult() {
  const res = await ocrBinding.Result(docId);
  ocrModel.setResult(res);
  if (res && res.error) setStatus(res.error, true);
}

function updateOCRChrome() {
  el.ocrAction.hidden = !ocrModel.canRecognize;
  el.ocrCancel.hidden = ocrModel.status !== 'running';
  el.ocrReview.hidden = !ocrModel.canReview;
  el.ocrReview.textContent = ocrModel.reviewing ? 'Hide review' : 'Review text';
  el.ocrAction.textContent = ocrModel.result ? 'Recognize again' : 'Recognize text';
}

/** Rebuild the review overlay on every page holder (cheap: only in review
 * mode; boxes are page-percent so they track any zoom, AC15). */
function redrawOCROverlays() {
  for (const layer of el.pages.querySelectorAll('.ocr-layer')) layer.remove();
  if (!ocrModel.reviewing || !model) return;
  for (const holder of el.pages.querySelectorAll('.page')) {
    drawPageOverlay(/** @type {HTMLElement} */ (holder));
  }
}

/** @param {HTMLElement} holder */
function drawPageOverlay(holder) {
  const page = Number(holder.dataset.page);
  const pageSize = model.pages[page];
  const units = ocrModel.unitsForPage(page);
  const failure = ocrModel.failureForPage(page);
  if (units.length === 0 && !failure) return;

  const layer = document.createElement('div');
  layer.className = 'ocr-layer';

  const bar = document.createElement('div');
  bar.className = 'ocr-page-bar';
  if (failure) {
    const note = document.createElement('span');
    note.className = 'ocr-fail';
    note.textContent = failure.message;
    bar.appendChild(note);
  }
  const redo = document.createElement('button');
  redo.textContent = 'Re-OCR page';
  redo.title = 'Run text recognition again for this page (replaces its result, spec AC5)';
  redo.addEventListener('click', async () => {
    try {
      await ocrBinding.RecognizePage(docId, page);
      setStatus(`Re-recognizing page ${page + 1}…`);
    } catch (e) {
      setStatus(String(e), true);
    }
  });
  bar.appendChild(redo);
  layer.appendChild(bar);

  units.forEach((u, i) => {
    const div = document.createElement('div');
    div.className = 'ocr-unit' + (u.corrected ? ' corrected' : '') + (needsAttention(u) ? ' low' : '');
    const r = unitRectPct(u.box, { widthPt: pageSize.widthPt, heightPt: pageSize.heightPt });
    div.style.left = r.left + '%';
    div.style.top = r.top + '%';
    div.style.width = r.width + '%';
    div.style.height = r.height + '%';
    div.title = `Confidence ${Math.round(u.confidence * 100)}%`
      + (u.corrected ? ` — engine read “${u.engineText}”. Click to edit; save the engine text to revert.` : '. Click to correct.');
    const span = document.createElement('span');
    span.textContent = u.text;
    div.appendChild(span);
    div.addEventListener('click', (ev) => {
      ev.stopPropagation();
      openUnitEditor(layer, div, page, i, u);
    });
    layer.appendChild(div);
  });

  holder.appendChild(layer);
  fitUnitTexts(layer);
}

/** Fit each unit's text into its box in both dimensions (AC15): font size
 * from the displayed box height, then a horizontal scale onto the box width. */
function fitUnitTexts(layer) {
  for (const div of layer.querySelectorAll('.ocr-unit')) {
    const span = div.querySelector('span');
    if (!span) continue;
    const boxW = div.clientWidth;
    const boxH = div.clientHeight;
    div.style.fontSize = fitText(boxW, boxH, 0).fontSize + 'px';
    span.style.transform = 'none';
    const measured = span.scrollWidth;
    span.style.transform = `scaleX(${fitText(boxW, boxH, measured).scaleX})`;
  }
}

/** Inline correction editor over a unit (spec A6): Enter saves the override
 * (or reverts when the text equals the engine original), Escape cancels. */
function openUnitEditor(layer, unitDiv, page, unitIndex, u) {
  if (layer.querySelector('.ocr-edit')) return;
  const input = document.createElement('input');
  input.className = 'ocr-edit';
  input.value = u.text;
  input.style.left = unitDiv.style.left;
  input.style.top = unitDiv.style.top;
  input.style.width = `max(${unitDiv.style.width}, 80px)`;
  input.style.height = unitDiv.style.height;
  input.style.fontSize = unitDiv.style.fontSize;
  layer.appendChild(input);
  input.focus();
  input.select();

  let closed = false;
  const close = () => {
    if (!closed) {
      closed = true;
      input.remove();
    }
  };
  const save = async () => {
    const text = input.value.trim();
    close();
    if (text === '' || text === u.text) return;
    try {
      if (text === u.engineText) await ocrBinding.Revert(docId, page, unitIndex);
      else await ocrBinding.Correct(docId, page, unitIndex, text);
      await refreshOCRResult();
      redrawOCROverlays();
    } catch (e) {
      setStatus(String(e), true);
    }
  };
  input.addEventListener('keydown', (ev) => {
    if (ev.key === 'Enter') save();
    else if (ev.key === 'Escape') close();
  });
  input.addEventListener('blur', close);
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
  redrawOCROverlays();
  if (!model || model.fit === 'custom') return;
  model.setViewport(el.pages.clientWidth - 40, el.pages.clientHeight - 40);
  model.applyFit(model.fit);
  rerenderForZoom();
});

// --- OCR controls + events (ocr T12) ---

el.ocrAction.addEventListener('click', async () => {
  try {
    await ocrBinding.Start(docId);
    ocrModel.started();
    setStatus(ocrModel.statusText());
  } catch (e) {
    setStatus(String(e), true);
  }
  updateOCRChrome();
});

el.ocrCancel.addEventListener('click', () => {
  ocrBinding.Cancel().catch(() => {});
});

el.ocrReview.addEventListener('click', () => {
  ocrModel.reviewing = !ocrModel.reviewing;
  redrawOCROverlays();
  updateOCRChrome();
});

onOCREvent(OCR_EVENTS.progress, (p) => {
  ocrModel.onProgress(p);
  setStatus(ocrModel.statusText());
  updateOCRChrome();
});

onOCREvent(OCR_EVENTS.done, async (d) => {
  ocrModel.onDone(d);
  setStatus(ocrModel.statusText(), ocrModel.status === 'failed');
  try {
    await refreshOCRResult();
  } catch (e) { /* result stays as-is; the status line already reports */ }
  redrawOCROverlays();
  updateOCRChrome();
});

onOCREvent(OCR_EVENTS.pageDone, async (d) => {
  setStatus(d.error ? d.error : `Page ${d.page + 1} re-recognized.`, Boolean(d.error));
  try {
    await refreshOCRResult();
  } catch (e) { /* keep the previous result on a failed read */ }
  redrawOCROverlays();
});

setStatus('Open a PDF to begin.');
