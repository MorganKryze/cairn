// Records the short looping clip used on Reddit and anywhere else a still
// picture does not answer "what is this page for".
//
// Run it with `just clip`, which brings the demo up first. Out of here come
// docs/assets/demo.mp4 for Reddit and docs/assets/demo.gif for places that
// refuse video.
//
// The sequence answers one question in about fourteen seconds: a visitor
// lands, searches, opens a service page, and switches language. It ends where
// it started so the loop has no visible seam.
//
// Playwright records webm at the viewport size; ffmpeg does the rest. Waits
// are deliberate and generous: a clip that moves faster than a reader reads
// shows nothing at all.

import { chromium } from 'playwright';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { mkdtempSync, readdirSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { execFileSync } from 'node:child_process';

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..');
const OUT = join(ROOT, 'docs', 'assets');
const URL = process.env.CAIRN_URL ?? 'http://127.0.0.1:8080/en/';
const BASE = URL.endsWith('/') ? URL : URL + '/';
const SIZE = { width: 1280, height: 800 };

// Typing one character at a time, slowly enough to be read as typing.
const type = async (page, text) => {
  for (const ch of text) {
    await page.keyboard.type(ch);
    await page.waitForTimeout(110);
  }
};

const tmp = mkdtempSync(join(tmpdir(), 'cairn-clip-'));

const browser = await chromium.launch();
const context = await browser.newContext({
  viewport: SIZE,
  recordVideo: { dir: tmp, size: SIZE },
  // The language switcher sets a cookie; a fresh context each run keeps the
  // clip identical from one recording to the next.
  locale: 'en-US',
  colorScheme: 'light',
});
const page = await context.newPage();

await page.goto(BASE, { waitUntil: 'networkidle' });
// The pills arrive with the page, but give a reader time to see the shape of
// the thing before anything moves.
await page.waitForTimeout(2600);

// Search: the one interaction that shows this is a directory, not a poster.
await page.click('#q');
await page.waitForTimeout(400);
await type(page, 'who');
await page.waitForTimeout(1700);

// Clearing it brings every card back, which is also the bug fixed in 1.11.
for (let i = 0; i < 3; i++) {
  await page.keyboard.press('Backspace');
  await page.waitForTimeout(110);
}
await page.waitForTimeout(900);

// A service page: where a visitor reads what a service is before clicking it.
await page.click('a.card-more[href$="/whoami/"]');
await page.waitForLoadState('networkidle');
await page.waitForTimeout(2300);
await page.goBack({ waitUntil: 'networkidle' });
await page.waitForTimeout(900);

// French, because the multilingual part is the half nobody expects.
await page.click('.langs a[href^="/fr/"]');
await page.waitForLoadState('networkidle');
await page.waitForTimeout(2600);

// Back to English so the last frame matches the first and the loop is seamless.
await page.click('.langs a[href^="/en/"]');
await page.waitForLoadState('networkidle');
await page.waitForTimeout(1200);

await context.close();
await browser.close();

const webm = readdirSync(tmp).find((f) => f.endsWith('.webm'));
if (!webm) throw new Error('playwright recorded no video');
const src = join(tmp, webm);

// mp4 for Reddit: yuv420p and an even width or half the players refuse it.
execFileSync('ffmpeg', [
  '-y', '-i', src,
  '-vf', 'scale=1280:-2,fps=24',
  '-c:v', 'libx264', '-preset', 'slow', '-crf', '23',
  '-pix_fmt', 'yuv420p', '-movflags', '+faststart',
  join(OUT, 'demo.mp4'),
], { stdio: 'ignore' });

// gif for the places that still refuse video, palette-generated so the teal
// does not band.
const palette = join(tmp, 'palette.png');
execFileSync('ffmpeg', ['-y', '-i', src, '-vf', 'fps=12,scale=960:-1:flags=lanczos,palettegen', palette], { stdio: 'ignore' });
execFileSync('ffmpeg', [
  '-y', '-i', src, '-i', palette,
  '-lavfi', 'fps=12,scale=960:-1:flags=lanczos[x];[x][1:v]paletteuse',
  join(OUT, 'demo.gif'),
], { stdio: 'ignore' });

rmSync(tmp, { recursive: true, force: true });
console.log('wrote docs/assets/demo.mp4 and docs/assets/demo.gif');
