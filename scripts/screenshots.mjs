// Regenerates every picture the README shows, from the running demo stack.
// Run it with `just shots`, which brings the demo up first.
//
// A script rather than `playwright screenshot`: the CLI has no device scale
// factor, and these look soft on a retina screen below 2x.
//
// Out of here come the hero in light and dark, which the README swaps with a
// <picture>, a detail page, a phone, and the social card GitHub puts on a
// shared link. The social card is composed rather than captured: a page
// screenshot cropped to 2:1 loses either its header or its cards.

import { chromium } from 'playwright';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { readFileSync } from 'node:fs';

const OUT = join(dirname(fileURLToPath(import.meta.url)), '..', 'docs', 'assets');
const URL = process.env.CAIRN_URL ?? 'http://127.0.0.1:8080/en/';
// The shots below append a path, so this has to guarantee the trailing slash:
// without it, `CAIRN_URL=http://host/en` produces `http://host/enwhoami/`.
const DEMO = URL.endsWith('/') ? URL : URL + '/';
const SIZE = { width: 1440, height: 1000 };
const RETINA = 2;

const browser = await chromium.launch();

for (const scheme of ['light', 'dark']) {
  const context = await browser.newContext({
    colorScheme: scheme,
    viewport: SIZE,
    deviceScaleFactor: RETINA,
    // Reduced motion holds every animation finished and renders the hosting
    // flags with their labels out, the state the stylesheet gives reduced
    // motion and touch visitors. The hero names the little house without
    // staging a fake hover.
    reducedMotion: 'reduce',
  });
  const page = await context.newPage();

  const response = await page.goto(URL, { waitUntil: 'networkidle' }).catch(() => null);
  if (!response?.ok()) {
    console.error(`\nCannot reach ${URL}. Start the demo first: just demo\n`);
    process.exit(1);
  }
  await page.evaluate(() => document.fonts.ready);

  await page.waitForTimeout(250);

  const file = join(OUT, `home-${scheme}.png`);
  await page.screenshot({ path: file });
  console.log(`  ${scheme.padEnd(5)} -> docs/assets/home-${scheme}.png`);
  await context.close();
}

for (const [name, url, viewport] of [
  ['detail', DEMO + 'whoami/', { width: 1100, height: 900 }],
  ['phone', DEMO, { width: 390, height: 844 }],
]) {
  const context = await browser.newContext({
    colorScheme: 'light',
    viewport,
    deviceScaleFactor: RETINA,
    reducedMotion: 'reduce',
    isMobile: name === 'phone',
    hasTouch: name === 'phone',
  });
  const page = await context.newPage();
  await page.goto(url, { waitUntil: 'networkidle' });
  // On a phone the welcome note fills the frame and the directory barely
  // shows. Dismissed is what a returning visitor sees, and it leaves the
  // chips and the cards in shot.
  if (name === 'phone') {
    await page.locator('#about-x').click().catch(() => {});
    await page.waitForTimeout(150);
  }
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(250);
  await page.screenshot({ path: join(OUT, `${name}.png`) });
  console.log(`  ${name.padEnd(5)} -> docs/assets/${name}.png`);
  await context.close();
}

// The two views composed into one picture rather than left to a two-cell
// table. Their aspects are 1.22 and 0.46, and a table equalises neither the
// heights nor the captions: the phone ran on far past the detail page and left
// a hole beside it. Matched heights here hold at any reader width.
{
  const context = await browser.newContext({
    viewport: { width: 1400, height: 830 },
    deviceScaleFactor: RETINA,
  });
  const page = await context.newPage();
  const root = join(dirname(fileURLToPath(import.meta.url)), '..');
  const png = (p) =>
    `data:image/png;base64,${readFileSync(join(root, p)).toString('base64')}`;
  await page.setContent(`<!doctype html><meta charset="utf-8"><style>
    * { margin: 0; box-sizing: border-box; }
    body { width: 1400px; height: 830px; background: #eef0ea;
           display: flex; align-items: center; justify-content: center; gap: 34px; }
    img { height: 760px; width: auto; border-radius: 12px;
          border: 1px solid #d7dad2; box-shadow: 0 18px 44px rgba(20,24,26,.16); }
  </style>
  <img src="${png('docs/assets/detail.png')}" alt="">
  <img src="${png('docs/assets/phone.png')}" alt="">`);
  await page.waitForTimeout(250);
  await page.screenshot({ path: join(OUT, 'two-views.png') });
  console.log('  views  -> docs/assets/two-views.png');
  await context.close();
}

// The social card: the mark, the sentence, and the page itself held at the
// edge of the frame. GitHub renders it at 1280x640.
//
// Everything is inlined as a data URI. A page built with setContent has an
// about:blank origin, which blocks every file:// subresource, so a src
// pointing at the repository loads nothing.
{
  const context = await browser.newContext({
    viewport: { width: 1280, height: 640 },
    deviceScaleFactor: RETINA,
  });
  const page = await context.newPage();
  const root = join(dirname(fileURLToPath(import.meta.url)), '..');
  const data = (p, mime) =>
    `data:${mime};base64,${readFileSync(join(root, p)).toString('base64')}`;
  const mark = data('docs/assets/brand/cairn.svg', 'image/svg+xml');
  const shot = data('docs/assets/home-light.png', 'image/png');
  const face = data('src/internal/render/assets/fonts/fraunces.woff2', 'font/woff2');

  await page.setContent(`<!doctype html><meta charset="utf-8"><style>
    @font-face { font-family: Fraunces; src: url("${face}") format("woff2"); }
    * { margin: 0; box-sizing: border-box; }
    body { width: 1280px; height: 640px; display: flex; overflow: hidden;
           background: #eef0ea; color: #14181a; font-family: system-ui, sans-serif; }
    .say { flex: 0 0 545px; padding: 0 0 0 74px; display: flex;
           flex-direction: column; justify-content: center; gap: 20px; }
    .name { display: flex; align-items: center; gap: 15px; }
    .name img { width: 50px; height: 50px; }
    .name span { font-family: Fraunces, serif; font-size: 56px; font-weight: 600;
                 letter-spacing: -.01em; line-height: 1; }
    h1 { font-family: Fraunces, serif; font-size: 31px; line-height: 1.3;
         font-weight: 500; max-width: 17ch; }
    h1 em { font-style: italic; color: #247b7b; }
    .facts { display: flex; gap: 9px; flex-wrap: wrap; margin-top: 2px; }
    .facts b { font: 500 14px/1 system-ui, sans-serif;
               background: #fbfbf9; border: 1px solid #d7dad2;
               border-radius: 999px; padding: 9px 13px; }
    .show { flex: 1; position: relative; }
    .show img { position: absolute; top: 62px; left: 0; width: 900px;
                border-radius: 14px; border: 1px solid #d7dad2;
                box-shadow: 0 26px 64px rgba(20,24,26,.20); }
  </style>
  <div class="say">
    <div class="name"><img src="${mark}" alt=""><span>cairn</span></div>
    <h1>The directory page for the people you host services <em>for</em>.</h1>
    <div class="facts"><b>No account</b><b>Multilingual</b><b>Live status</b><b>4.4 MB image</b></div>
  </div>
  <div class="show"><img src="${shot}" alt=""></div>`);
  await page.evaluate(() => document.fonts.ready);
  await page.waitForTimeout(300);
  await page.screenshot({ path: join(OUT, 'social-card.png') });
  console.log('  social -> docs/assets/social-card.png');
  await context.close();
}

// Rounded corners, baked in. GitHub strips the style attribute from a README,
// so the only place the corner can live is the file. Transparent outside the
// radius, so the same file sits right on a light canvas and a dark one.
//
// Last, so the social card keeps the square source it draws in its own frame.
for (const [file, radius] of [
  ['home-light.png', 26],
  ['home-dark.png', 26],
  ['two-views.png', 26],
]) {
  // 1, not RETINA: these sources are already 2x.
  const context = await browser.newContext({ deviceScaleFactor: 1 });
  const page = await context.newPage();
  const src = `data:image/png;base64,${readFileSync(join(OUT, file)).toString('base64')}`;
  await page.setContent(
    `<style>html,body{margin:0;background:none}` +
      `img{display:block;border-radius:${radius}px}</style><img src="${src}">`,
  );
  const img = page.locator('img');
  await img.waitFor();
  await img.screenshot({ path: join(OUT, file), omitBackground: true });
  console.log(`  round  -> docs/assets/${file}`);
  await context.close();
}

await browser.close();
