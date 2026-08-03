// Regenerates every picture the README shows, from the running demo stack.
// Run it with `just shots`, which brings the demo up first.
//
// Why a script rather than `playwright screenshot`: the CLI has no device
// scale factor, and these have to be 2x or they look soft on a retina screen.
//
// Four things come out of here. The hero, light and dark, which the README
// swaps with a <picture>. A detail page and a phone, because the hero shows
// one screen and the two questions it leaves are "what is behind a card" and
// "what does this look like in a hand". And the social card, which is the
// picture GitHub puts on a link when somebody shares the repository: it is
// the only one that is composed rather than captured, since a page screenshot
// cropped to 2:1 loses either its header or its cards.

import { chromium } from 'playwright';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { readFileSync } from 'node:fs';

const OUT = join(dirname(fileURLToPath(import.meta.url)), '..', 'docs', 'assets');
const URL = process.env.CAIRN_URL ?? 'http://127.0.0.1:8080/en/';
// The same site, for the shots that open a page of their own.
const DEMO = URL.replace(/\/en\/$/, '/en/');
const SIZE = { width: 1440, height: 1000 };

const browser = await chromium.launch();

for (const scheme of ['light', 'dark']) {
  const context = await browser.newContext({
    colorScheme: scheme,
    viewport: SIZE,
    deviceScaleFactor: 2, // retina: the README is read on laptops
    // Reduced motion does two things here: no animation caught mid-frame, and
    // the hosting flags render with their labels out, which is the state the
    // stylesheet already gives reduced-motion and touch visitors. So the hero
    // explains what the little house means without staging a fake hover.
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

// The detail page and the phone: the two questions the hero leaves open.
for (const [name, url, viewport] of [
  ['detail', DEMO + 'whoami/', { width: 1100, height: 900 }],
  ['phone', DEMO, { width: 390, height: 844 }],
]) {
  const context = await browser.newContext({
    colorScheme: 'light',
    viewport,
    deviceScaleFactor: 2,
    reducedMotion: 'reduce',
    isMobile: name === 'phone',
    hasTouch: name === 'phone',
  });
  const page = await context.newPage();
  await page.goto(url, { waitUntil: 'networkidle' });
  // On a phone the welcome note fills the frame and the directory itself
  // barely shows. Dismissing it is what a returning visitor sees anyway, and
  // it is the half worth photographing: the chips and the cards.
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

// The social card. GitHub renders it at 1280x640 on a shared link, and a page
// screenshot cropped to that ratio loses either its header or its cards, so
// this one is composed: the mark, the sentence, and the page itself held at
// the edge of the frame.
//
// Everything is inlined as a data URI. A page built with setContent has an
// about:blank origin, which blocks every file:// subresource, so a src that
// points at the repository loads nothing and the card comes out with a broken
// image where the mark should be.
{
  const context = await browser.newContext({
    viewport: { width: 1280, height: 640 },
    deviceScaleFactor: 2,
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

await browser.close();
