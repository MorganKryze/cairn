// Every icon cairn ships, generated from the one drawing below, so none of the
// set can drift from the rest. Run it with `just icons`.
//
//   src/internal/render/assets/     compiled into the binary and served
//     favicon.svg             the tab icon; every current browser uses this
//     favicon.ico             16/32/48, for the tools that fetch /favicon.ico
//                             and never read the html
//     touch-icon.png    180   apple-touch-icon: an iPhone or iPad home screen
//     icon-192.png      192   Chromium offers "install" only with both of
//     icon-512.png      512   these present; also the Android splash
//
//   docs/assets/brand/              not embedded, not served: for the icon
//     cairn.svg               collections that ask for a copy of the mark,
//     cairn-light.svg         cropped to its own edges the way they require.
//     cairn-dark.svg
//
// The suffix names the ink: -light is the pale one, for a dark background,
// -dark the deep one, for a light background. Reading it as "for the light
// theme" is the mistake those collections reject most often.
//
// The mark is also inlined in templates/layout.tmpl, as the icon a service
// falls back to when it declares none. That copy is pinned to favicon.svg by
// TestTheMarkIsDrawnTheSameInBothPlaces.
import { chromium } from 'playwright';
import { mkdirSync, writeFileSync, readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..');
const ASSETS = join(ROOT, 'src', 'internal', 'render', 'assets');
const BRAND = join(ROOT, 'docs', 'assets', 'brand');
mkdirSync(BRAND, { recursive: true });

// Four stones of the same gauge, the top one set down at nine degrees. Four
// rather than five: five merge into a cone at 16px.
const MARK =
  '<rect x="3.2" y="23.6" width="25" height="5.4" rx="2.4"/>' +
  '<rect x="7.8" y="16.2" width="17.4" height="5.8" rx="2.6"/>' +
  '<rect x="9.4" y="9.4" width="13" height="5.4" rx="2.5"/>' +
  '<rect x="12.4" y="3" width="9.4" height="4.8" rx="2.2" transform="rotate(-9 17.1 5.4)"/>';

const BRAND_INK = '#247b7b'; // the accent, and the only ink the product uses
const ON_DARK = '#5fc4c0';   // ships as cairn-light.svg
const ON_LIGHT = '#1a5c5c';  // ships as cairn-dark.svg
const PAGE = '#eef0ea';      // the light page colour, under every app icon

// How much of the square the mark covers in an app icon: as full as it can be
// while its far corners still clear Android's crop circle. Checked below.
const FILL = 0.69;

const doc = (viewBox, fill) =>
  `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${viewBox}" fill="${fill}">${MARK}</svg>\n`;

const browser = await chromium.launch();

// A null background leaves the png transparent: what a favicon wants, and what
// a home-screen icon must never be.
async function png(size, fill, bg) {
  const p = await browser.newPage({ viewport: { width: size, height: size }, deviceScaleFactor: 1 });
  await p.setContent(`<!doctype html><meta charset="utf-8"><style>
    html,body { margin:0; width:${size}px; height:${size}px; ${bg ? `background:${bg};` : ''} }
    body { display:grid; place-items:center; }
    svg { width:${size * fill}px; height:${size * fill}px; display:block; }
  </style>${doc('0 0 32 32', BRAND_INK)}`);
  const buf = await p.screenshot({ omitBackground: !bg });
  await p.close();
  return buf;
}

// An ICO is a small header plus, since Vista, plain pngs.
function ico(images) {
  const dir = Buffer.alloc(6);
  dir.writeUInt16LE(1, 2); // type: icon
  dir.writeUInt16LE(images.length, 4);
  let offset = 6 + images.length * 16;
  const entries = images.map(({ size, data }) => {
    const e = Buffer.alloc(16);
    e.writeUInt8(size === 256 ? 0 : size, 0); // width, 0 means 256
    e.writeUInt8(size === 256 ? 0 : size, 1); // height
    e.writeUInt16LE(1, 4);  // colour planes
    e.writeUInt16LE(32, 6); // bits per pixel
    e.writeUInt32LE(data.length, 8);
    e.writeUInt32LE(offset, 12);
    offset += data.length;
    return e;
  });
  return Buffer.concat([dir, ...entries, ...images.map((i) => i.data)]);
}

// The ink's own box in viewBox units, so the brand files can be cropped to the
// mark rather than to its padded square. Off a raster rather than computed:
// the tilted stone and its rounded corners put an analytic box out by about a
// third of a unit.
const probe = await browser.newPage({ viewport: { width: 600, height: 600 } });
const inkBox = async (markup) => probe.evaluate(async (svg) => {
  const S = 512;
  const img = new Image();
  await new Promise((r) => { img.onload = r; img.src = 'data:image/svg+xml;base64,' + btoa(svg); });
  const c = document.createElement('canvas');
  c.width = c.height = S;
  const ctx = c.getContext('2d');
  ctx.drawImage(img, 0, 0, S, S);
  const { data } = ctx.getImageData(0, 0, S, S);
  let x0 = S, y0 = S, x1 = 0, y1 = 0;
  for (let y = 0; y < S; y++) {
    for (let x = 0; x < S; x++) {
      if (data[(y * S + x) * 4 + 3] > 8) {
        if (x < x0) x0 = x; if (x > x1) x1 = x;
        if (y < y0) y0 = y; if (y > y1) y1 = y;
      }
    }
  }
  const u = 32 / S;
  return { x: x0 * u, y: y0 * u, w: (x1 - x0 + 1) * u, h: (y1 - y0 + 1) * u };
}, markup);

// --- the tab icon ---------------------------------------------------------
writeFileSync(join(ASSETS, 'favicon.svg'), doc('0 0 32 32', BRAND_INK));
writeFileSync(join(ASSETS, 'favicon.ico'), ico(
  await Promise.all([16, 32, 48].map(async (size) => ({ size, data: await png(size, 1, null) }))),
));

// --- the home-screen icons ------------------------------------------------
writeFileSync(join(ASSETS, 'touch-icon.png'), await png(180, FILL, PAGE));
writeFileSync(join(ASSETS, 'icon-192.png'), await png(192, FILL, PAGE));
writeFileSync(join(ASSETS, 'icon-512.png'), await png(512, FILL, PAGE));

// --- the brand files, cropped to the ink ----------------------------------
const box = await inkBox(doc('0 0 32 32', BRAND_INK));
const r3 = (n) => Number(n.toFixed(3));
const tight = `${r3(box.x)} ${r3(box.y)} ${r3(box.w)} ${r3(box.h)}`;
writeFileSync(join(BRAND, 'cairn.svg'), doc(tight, BRAND_INK));
writeFileSync(join(BRAND, 'cairn-light.svg'), doc(tight, ON_DARK));
writeFileSync(join(BRAND, 'cairn-dark.svg'), doc(tight, ON_LIGHT));

// --- and the guarantee the manifest makes ---------------------------------
// The app icons are declared "any maskable", which promises Android that no
// ink lives outside a circle of 80% of the canvas. That holds by FILL and the
// mark's proportions, so a redrawn mark can break it silently.
let failed = false;
for (const [name, size] of [['icon-192.png', 192], ['icon-512.png', 512], ['touch-icon.png', 180]]) {
  const far = await probe.evaluate(async ([src, S]) => {
    const img = new Image();
    await new Promise((r) => { img.onload = r; img.src = src; });
    const c = document.createElement('canvas');
    c.width = c.height = S;
    const ctx = c.getContext('2d');
    ctx.drawImage(img, 0, 0);
    const { data } = ctx.getImageData(0, 0, S, S);
    const bg = [data[0], data[1], data[2]];
    let d = 0;
    for (let y = 0; y < S; y++) {
      for (let x = 0; x < S; x++) {
        const i = (y * S + x) * 4;
        if (Math.abs(data[i] - bg[0]) + Math.abs(data[i + 1] - bg[1]) + Math.abs(data[i + 2] - bg[2]) <= 40) continue;
        d = Math.max(d, Math.hypot(x + 0.5 - S / 2, y + 0.5 - S / 2));
      }
    }
    return d;
  }, ['data:image/png;base64,' + readFileSync(join(ASSETS, name)).toString('base64'), size]);

  const safe = 0.4 * size;
  const ok = far <= safe;
  failed ||= !ok;
  console.log(`  ${name.padEnd(16)} farthest ink ${far.toFixed(1)}px / safe radius ${safe.toFixed(1)}px  ${ok ? 'ok' : 'OUTSIDE THE CROP'}`);
}

await browser.close();
if (failed) {
  console.error('\nan app icon reaches outside the maskable safe zone; lower FILL or redraw');
  process.exit(1);
}
console.log(`\nbrand files cropped to viewBox="${tight}"`);
