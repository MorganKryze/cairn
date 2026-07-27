// Regenerates the README hero, light and dark, from the running demo stack.
// Run it with `just shots`, which brings the demo up first.
//
// Why a script rather than `playwright screenshot`: the CLI has no device
// scale factor, and the hero has to be 2x or it looks soft on a retina screen.

import { chromium } from 'playwright';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const OUT = join(dirname(fileURLToPath(import.meta.url)), '..', 'docs', 'assets');
const URL = process.env.CAIRN_URL ?? 'http://127.0.0.1:8080/en/';
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

await browser.close();
