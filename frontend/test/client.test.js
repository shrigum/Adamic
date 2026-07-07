// @ts-check
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { ReaderClient } from '../src/client.js';

/** A fake binding recording calls, standing in for the Wails-generated one. */
function fakeBinding() {
  const calls = [];
  const rec = (name) => (...args) => {
    calls.push({ name, args });
    if (name === 'Open') return Promise.resolve({ ok: true, doc: { id: 'd', path: args[0], pages: [], position: { page: 0, offsetY: 0 } } });
    if (name === 'GetPosition') return Promise.resolve({ page: 0, offsetY: 0 });
    if (name.startsWith('Render') || name === 'Thumbnail') return Promise.resolve('data:image/png;base64,AAAA');
    return Promise.resolve();
  };
  return {
    calls,
    Open: rec('Open'),
    RenderPage: rec('RenderPage'),
    RenderPageFit: rec('RenderPageFit'),
    Thumbnail: rec('Thumbnail'),
    SetPosition: rec('SetPosition'),
    GetPosition: rec('GetPosition'),
    Close: rec('Close'),
  };
}

test('client forwards every call to the binding with the same args', async () => {
  const b = fakeBinding();
  const c = new ReaderClient(b);

  const res = await c.open('/book.pdf');
  assert.equal(res.ok, true);
  assert.equal(res.doc.path, '/book.pdf');

  await c.renderPage('d', 3, 1.5);
  await c.renderPageFit('d', 3, 800, 600, true);
  await c.thumbnail('d', 2);
  await c.setPosition('d', 4, 0.5);
  const pos = await c.getPosition('d');
  assert.deepEqual(pos, { page: 0, offsetY: 0 });
  await c.close('d');

  const names = b.calls.map((c) => c.name);
  assert.deepEqual(names, [
    'Open', 'RenderPage', 'RenderPageFit', 'Thumbnail', 'SetPosition', 'GetPosition', 'Close',
  ]);
  // Spot-check argument pass-through.
  assert.deepEqual(b.calls[1].args, ['d', 3, 1.5]);
  assert.deepEqual(b.calls[2].args, ['d', 3, 800, 600, true]);
});
