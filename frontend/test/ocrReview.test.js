// @ts-check
// Tests for the OCR review model (ocr T12): run state machine, review
// gating, and the fit-text-to-box geometry that carries AC15.
import { test } from 'node:test';
import assert from 'node:assert/strict';

import {
  OCRReviewModel,
  unitRectPct,
  fitText,
  fontSizePx,
  needsAttention,
  LOW_CONFIDENCE,
  FONT_FIT,
} from '../src/ocrReview.js';

const unit = (over = {}) => ({
  text: 'Goedemorgen',
  engineText: 'Goedemorgen',
  corrected: false,
  box: { x: 59.7, y: 84.4, w: 119.4, h: 16.88 },
  confidence: 0.95,
  ...over,
});

test('recognize is offered only with candidates and no run in flight (A5)', () => {
  const m = new OCRReviewModel();
  assert.equal(m.canRecognize, false, 'no candidates yet');
  m.setCandidates([0, 1, 2, 3]);
  assert.equal(m.canRecognize, true);
  m.started();
  assert.equal(m.canRecognize, false, 'not while running');
  m.onDone({ ok: true, cancelled: false, pages: 4 });
  assert.equal(m.canRecognize, true, 'explicit re-run stays possible');
});

test('progress counts pages and failures toward the status line', () => {
  const m = new OCRReviewModel();
  m.setCandidates([0, 1, 2, 3]);
  m.started();
  m.onProgress({ page: 0, pageCount: 4, failed: false });
  m.onProgress({ page: 1, pageCount: 4, failed: true });
  assert.equal(m.statusText(), 'Recognizing… 2/4 (1 failed)');
  m.onDone({ ok: true, cancelled: false, pages: 4 });
  assert.equal(m.status, 'done');
  assert.match(m.statusText(), /1 page\(s\) failed/);
});

test('cancelled and failed runs read honestly', () => {
  const m = new OCRReviewModel();
  m.started();
  m.onDone({ ok: false, cancelled: true, pages: 1 });
  assert.equal(m.status, 'cancelled');
  assert.match(m.statusText(), /finished pages are kept/);

  m.started();
  m.onDone({ ok: false, cancelled: false, pages: 0, error: 'engine exploded' });
  assert.equal(m.status, 'failed');
  assert.equal(m.statusText(), 'engine exploded');
});

test('review is offered once a result exists (AC6)', () => {
  const m = new OCRReviewModel();
  assert.equal(m.canReview, false);
  m.setResult({ hasResult: true, pages: [{ page: 0, units: [unit()] }] });
  assert.equal(m.canReview, true);
  m.setResult({ hasResult: false });
  assert.equal(m.canReview, false, 'a no-result read clears review');
});

test('unitsForPage and failureForPage read the result per page', () => {
  const m = new OCRReviewModel();
  m.setResult({
    hasResult: true,
    pages: [
      { page: 0, units: [unit()] },
      { page: 2, failure: { kind: 'unreadable', message: 'boom' } },
    ],
  });
  assert.equal(m.unitsForPage(0).length, 1);
  assert.deepEqual(m.unitsForPage(1), [], 'page without a result draws nothing');
  assert.deepEqual(m.unitsForPage(2), [], 'failed page has no units');
  assert.equal(m.failureForPage(2)?.message, 'boom');
  assert.equal(m.failureForPage(0), null);
});

test('unitRectPct maps page points onto zoom-independent percentages (AC15/A14)', () => {
  const pageSize = { widthPt: 597, heightPt: 844 };
  const r = unitRectPct({ x: 59.7, y: 84.4, w: 119.4, h: 16.88 }, pageSize);
  assert.ok(Math.abs(r.left - 10) < 1e-9);
  assert.ok(Math.abs(r.top - 10) < 1e-9);
  assert.ok(Math.abs(r.width - 20) < 1e-9);
  assert.ok(Math.abs(r.height - 2) < 1e-9);
});

test('fitText fills the box in both dimensions (AC15)', () => {
  // Box displayed at 200x20 px; the text naturally measures 150 px at the
  // fitted font size -> stretched by 4/3. A wider run gets squeezed.
  const fit = fitText(200, 20, 150);
  assert.equal(fit.fontSize, fontSizePx(20));
  assert.ok(Math.abs(fit.scaleX - 200 / 150) < 1e-9);
  assert.ok(fitText(100, 20, 400).scaleX < 1, 'long text squeezes to the box');
  assert.equal(fitText(100, 20, 0).scaleX, 1, 'unmeasurable text is left unscaled');
});

test('fontSizePx tracks box height with a floor', () => {
  assert.equal(fontSizePx(20), 20 * FONT_FIT);
  assert.equal(fontSizePx(1), 4, 'tiny boxes stay legible enough to edit');
});

test('needsAttention flags only uncorrected low-confidence units', () => {
  assert.equal(needsAttention(unit({ confidence: LOW_CONFIDENCE - 0.01 })), true);
  assert.equal(needsAttention(unit({ confidence: LOW_CONFIDENCE })), false);
  assert.equal(
    needsAttention(unit({ confidence: 0.1, corrected: true })),
    false,
    'a corrected unit is resolved, not flagged',
  );
});
