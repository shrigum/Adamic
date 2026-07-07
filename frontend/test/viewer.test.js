// @ts-check
import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  ViewerModel,
  clamp,
  ZOOM_STOPS,
  MIN_ZOOM,
  MAX_ZOOM,
  LOOK_AHEAD,
} from '../src/viewer.js';

/** A4-ish pages in points. */
function pages(n) {
  return Array.from({ length: n }, () => ({ widthPt: 595, heightPt: 842 }));
}

test('restores position on construct, clamped to the document', () => {
  const m = new ViewerModel(pages(10), { page: 4, offsetY: 0.3 });
  assert.equal(m.page, 4);
  assert.equal(m.offsetY, 0.3);

  const clamped = new ViewerModel(pages(3), { page: 99 });
  assert.equal(clamped.page, 2, 'out-of-range restored page clamps to last');
});

test('next/prev clamp at the ends without wrapping (AC2)', () => {
  const m = new ViewerModel(pages(3));
  assert.equal(m.page, 0);
  assert.equal(m.prevPage(), false, 'prev at first page is a no-op');
  assert.equal(m.page, 0);

  assert.equal(m.nextPage(), true);
  assert.equal(m.page, 1);
  m.nextPage();
  assert.equal(m.page, 2);
  assert.equal(m.nextPage(), false, 'next at last page is a no-op');
  assert.equal(m.page, 2, 'no wrap to page 0');
});

test('goToPage rejects out-of-range, accepts valid (AC5)', () => {
  const m = new ViewerModel(pages(5));
  assert.equal(m.goToPage(3), true);
  assert.equal(m.page, 3);
  assert.equal(m.goToPage(-1), false);
  assert.equal(m.goToPage(5), false, 'page index 5 is out of range for 5 pages');
  assert.equal(m.goToPage(2.5), false, 'non-integer rejected');
  assert.equal(m.page, 3, 'rejected navigation left the page unchanged');
});

test('resolveUserPage maps 1-based input, null on invalid (AC5)', () => {
  const m = new ViewerModel(pages(10));
  assert.equal(m.resolveUserPage(1), 0);
  assert.equal(m.resolveUserPage('10'), 9);
  assert.equal(m.resolveUserPage(0), null, '0 is invalid (1-based)');
  assert.equal(m.resolveUserPage(11), null, 'beyond last page');
  assert.equal(m.resolveUserPage('abc'), null);
  assert.equal(m.resolveUserPage(3.5), null);
});

test('goToPage resets within-page offset to top', () => {
  const m = new ViewerModel(pages(5), { page: 0, offsetY: 0.9 });
  m.goToPage(2);
  assert.equal(m.offsetY, 0, 'a jump lands at the top of the target page');
});

test('fit-to-width sets zoom from page width and viewport (AC4)', () => {
  const m = new ViewerModel(pages(3));
  m.applyFit('width');
  m.setViewport(1190, 800); // 2x the 595pt page width
  assert.ok(Math.abs(m.zoom - 2.0) < 1e-9, `zoom ${m.zoom} should be ~2.0`);
});

test('fit-to-page is bounded by the tighter dimension (AC4)', () => {
  const m = new ViewerModel(pages(3));
  m.applyFit('page');
  // width would allow 2x (1190/595) but height only 1x (842/842): height binds.
  m.setViewport(1190, 842);
  assert.ok(Math.abs(m.zoom - 1.0) < 1e-9, `zoom ${m.zoom} should be ~1.0`);
});

test('fit mode recomputes on viewport resize (AC4)', () => {
  const m = new ViewerModel(pages(3));
  m.applyFit('width');
  m.setViewport(595, 800);
  assert.ok(Math.abs(m.zoom - 1.0) < 1e-9);
  m.setViewport(892.5, 800); // resize to 1.5x width
  assert.ok(Math.abs(m.zoom - 1.5) < 1e-9, 'zoom tracked the resize');
});

test('explicit zoom switches to custom and clamps to range', () => {
  const m = new ViewerModel(pages(3));
  m.setViewport(595, 800);
  m.applyFit('width');
  m.setZoom(3.0);
  assert.equal(m.fit, 'custom');
  assert.equal(m.zoom, 3.0);
  // A resize now must NOT move a custom zoom.
  m.setViewport(1190, 800);
  assert.equal(m.zoom, 3.0, 'custom zoom is not disturbed by resize');

  m.setZoom(99);
  assert.equal(m.zoom, MAX_ZOOM, 'zoom clamps to max');
  m.setZoom(0.01);
  assert.equal(m.zoom, MIN_ZOOM, 'zoom clamps to min');
});

test('zoomIn/zoomOut step through the discrete stops', () => {
  const m = new ViewerModel(pages(3));
  m.setZoom(1.0);
  m.zoomIn();
  assert.equal(m.zoom, 1.25, 'next stop above 1.0');
  m.zoomOut();
  assert.equal(m.zoom, 1.0);
  // Saturate at the ends.
  m.setZoom(MAX_ZOOM);
  m.zoomIn();
  assert.equal(m.zoom, MAX_ZOOM);
  m.setZoom(MIN_ZOOM);
  m.zoomOut();
  assert.equal(m.zoom, MIN_ZOOM);
});

test('renderRange in single mode is the current page plus look-ahead (T7)', () => {
  const m = new ViewerModel(pages(100));
  m.setMode('single');
  m.goToPage(50);
  const r = m.renderRange();
  assert.equal(r.first, 50 - LOOK_AHEAD);
  assert.equal(r.last, 50 + LOOK_AHEAD);
});

test('renderRange in continuous mode wraps the visible span with look-ahead', () => {
  const m = new ViewerModel(pages(100));
  m.setMode('continuous');
  const r = m.renderRange(40, 43);
  assert.equal(r.first, 40 - LOOK_AHEAD);
  assert.equal(r.last, 43 + LOOK_AHEAD);
});

test('renderRange clamps at document edges (T7/AC3)', () => {
  const m = new ViewerModel(pages(5));
  m.setMode('continuous');
  const r = m.renderRange(0, 4);
  assert.equal(r.first, 0, 'no negative pages');
  assert.equal(r.last, 4, 'no pages past the end');
});

test('clamp helper', () => {
  assert.equal(clamp(5, 0, 3), 3);
  assert.equal(clamp(-1, 0, 3), 0);
  assert.equal(clamp(2, 0, 3), 2);
  assert.equal(clamp(2, 5, 1), 5, 'inverted range returns lo');
});

test('ZOOM_STOPS are sorted and span MIN..MAX', () => {
  for (let i = 1; i < ZOOM_STOPS.length; i++) {
    assert.ok(ZOOM_STOPS[i] > ZOOM_STOPS[i - 1], 'stops strictly increasing');
  }
  assert.equal(ZOOM_STOPS[0], MIN_ZOOM);
  assert.equal(ZOOM_STOPS[ZOOM_STOPS.length - 1], MAX_ZOOM);
});
