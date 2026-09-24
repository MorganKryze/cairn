// Drives a static export in a real browser, served the way a static host
// serves one: files and nothing behind them. The Go tests hold what the export
// writes; lang.js choosing a language on the root page and the switcher
// pinning one only happen here.
//
// Run it with `just test-browser`, which exports scripts/fixtures/static under
// -base-path /tools first.
//
// Usage: node scripts/export.mjs <export directory>
import { createServer } from 'node:http';
import { readFile, stat } from 'node:fs/promises';
import { extname, join } from 'node:path';
import { chromium } from 'playwright';

const DIR = process.argv[2];
const BASE = '/tools';
if (!DIR) {
  console.error('usage: node scripts/export.mjs <export directory>');
  process.exit(2);
}

const TYPES = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript',
  '.css': 'text/css',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.ico': 'image/x-icon',
  '.woff2': 'font/woff2',
};

// The least a static host does: the file a path names, index.html for a
// directory, and for anything else the nearest 404.html up the tree, which is
// what Cloudflare Pages looks for.
const server = createServer(async (req, res) => {
  const send = async (file, status) => {
    const body = await readFile(file);
    res.writeHead(status, { 'content-type': TYPES[extname(file)] ?? 'application/octet-stream' });
    res.end(body);
  };
  const path = decodeURIComponent(new URL(req.url, 'http://host').pathname);
  if (!path.startsWith(BASE + '/')) return send(join(DIR, '404.html'), 404);
  let rel = path.slice(BASE.length);
  if (rel.endsWith('/')) rel += 'index.html';
  const file = join(DIR, rel);
  if (await stat(file).then(s => s.isFile(), () => false)) return send(file, 200);
  const near = join(DIR, rel.split('/')[1], '404.html');
  return send((await stat(near).then(() => true, () => false)) ? near : join(DIR, '404.html'), 404);
});
await new Promise(ok => server.listen(0, '127.0.0.1', ok));
const ORIGIN = `http://127.0.0.1:${server.address().port}`;

let failures = 0;
const check = async (what, fn) => {
  try {
    await fn();
    console.log(`  ok    ${what}`);
  } catch (e) {
    failures++;
    console.log(`  FAIL  ${what}\n        ${e.message}`);
  }
};
const eq = (got, want, label) => {
  const [g, w] = [JSON.stringify(got), JSON.stringify(want)];
  if (g !== w) throw new Error(`${label}: got ${g}, want ${w}`);
};

const browser = await chromium.launch();
const visit = async (options, fn) => {
  const context = await browser.newContext(options);
  try {
    await fn(await context.newPage(), context);
  } finally {
    await context.close();
  }
};
const landsOn = async (page, path) => {
  await page.goto(`${ORIGIN}${BASE}/`);
  await page.waitForURL(`${ORIGIN}${path}`, { timeout: 5000 });
};

console.log(`static export: ${DIR}`);

await check('a French browser lands on the French home', () =>
  visit({ locale: 'fr-FR' }, page => landsOn(page, `${BASE}/fr/`)));

await check('a browser in a language the site lacks lands on the first locale', () =>
  visit({ locale: 'ja-JP' }, page => landsOn(page, `${BASE}/en/`)));

await check('a language pinned earlier beats the browser', () =>
  visit({ locale: 'fr-FR' }, async (page, context) => {
    await context.addCookies([{ name: 'locale', value: 'en', url: `${ORIGIN}${BASE}/` }]);
    await landsOn(page, `${BASE}/en/`);
  }));

await check('the switcher pins the language, and drops ?choose from the address', () =>
  visit({ locale: 'fr-FR' }, async (page, context) => {
    await page.goto(`${ORIGIN}${BASE}/fr/pad/`);
    await page.click(`.langs a[href="${BASE}/en/pad/?choose"]`);
    await page.waitForURL(`${ORIGIN}${BASE}/en/pad/`, { timeout: 5000 });
    await page.waitForFunction(() => !location.search);
    const cookie = (await context.cookies()).find(c => c.name === 'locale');
    eq(cookie && [cookie.value, cookie.path], ['en', `${BASE}/`], 'locale cookie');
    await landsOn(page, `${BASE}/en/`);
  }));

await check('without JavaScript the root page lands on the first locale', () =>
  visit({ locale: 'fr-FR', javaScriptEnabled: false }, page => landsOn(page, `${BASE}/en/`)));

await check('an address that leads nowhere gets the 404 page in its language', () =>
  visit({ locale: 'en-GB' }, async page => {
    const res = await page.goto(`${ORIGIN}${BASE}/fr/nope/`);
    eq(res.status(), 404, 'status');
    eq(await page.textContent('h1'), 'Page introuvable', 'heading');
  }));

await browser.close();
server.close();
if (failures) {
  console.log(`\n${failures} failure(s)`);
  process.exit(1);
}
