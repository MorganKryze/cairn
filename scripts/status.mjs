// Drives the pill refresh in a real browser against a running cairn. The Go
// tests assert that the slots and the script ship; a pill changing in a page
// nobody reloaded is only visible here. cairn reconnects to Gatus, a tab left
// open on a wall display does not.
//
// cairn is pointed at a Gatus that does not exist, so every pill starts
// unknown; the test intercepts the fetch the script makes and hands back the
// page cairn would have rendered once Gatus answered. No second server to
// stand up, and the same bytes either way, since what the script parses is a
// whole cairn page.
//
// Run it with `just test-browser`, which serves scripts/fixtures/status first.
//
// Usage: node scripts/status.mjs [url]
import { chromium } from 'playwright';

const URL = process.argv[2] ?? process.env.CAIRN_STATUS_URL ?? 'http://127.0.0.1:8092/en/';

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
const page = await browser.newPage();
// Fake time, so a five-second poll is not five seconds of test. It has to be
// installed before the navigation.
await page.clock.install();
await page.goto(URL);

// The page as cairn first served it, and the only source of the markup fed
// back in: handwritten HTML would prove the test's markup works, not cairn's.
const original = await page.content();
let answer = original;
await page.route(URL, async route => {
  // Only the script's own request: a reload during the run must still reach
  // cairn.
  if (route.request().resourceType() !== 'fetch') return route.fallback();
  await route.fulfill({ status: 200, contentType: 'text/html; charset=utf-8', body: answer });
});

const pillOf = id =>
  page.$eval(`.status-slot[data-status="${id}"]`, el => el.firstElementChild?.className ?? '');
// An empty foot that still draws its padding is the regression the :has()
// rule stops.
const footHeight = id =>
  page.$eval(`.status-slot[data-status="${id}"]`, el => {
    const foot = el.closest('.card-foot');
    return foot ? foot.getBoundingClientRect().height : -1;
  });
// One poll interval and the microtasks the fetch resolves through.
const tick = async () => {
  await page.clock.runFor(6000);
  await page.waitForTimeout(200);
};

await check('every service starts with a slot, and a pill in it', async () => {
  eq(await page.locator('.status-slot').count(), 2, 'slots');
  eq(await pillOf('alpha'), 'status-pill status-unknown', "alpha's pill");
  eq(await pillOf('bravo'), 'status-pill status-unknown', "bravo's pill");
});

await check('a pill that changed is swapped without a reload', async () => {
  const before = await page.evaluate(() => performance.getEntriesByType('navigation').length);
  answer = original.replace(
    /(<span class="status-slot" data-status="alpha">.*?status-pill )status-unknown/s,
    '$1status-up',
  );
  await tick();
  eq(await pillOf('alpha'), 'status-pill status-up', "alpha's pill");
  eq(
    await page.evaluate(() => performance.getEntriesByType('navigation').length),
    before,
    'navigations',
  );
});

await check('a pill the status page dropped goes away, and takes its space', async () => {
  answer = answer.replace(
    /(<span class="status-slot" data-status="bravo">).*?(<\/span><\/div>)/s,
    '$1$2',
  );
  await tick();
  eq(await pillOf('bravo'), '', "bravo's pill");
  // alpha keeps a host flag, so its foot stays; bravo's now holds nothing.
  if ((await footHeight('bravo')) !== 0) {
    throw new Error(`an emptied card foot is ${await footHeight('bravo')}px tall, want 0`);
  }
  if ((await footHeight('alpha')) <= 0) {
    throw new Error('the foot of a card that still has a flag collapsed too');
  }
});

await check('a pill under the keyboard is left alone until focus moves on', async () => {
  answer = original.replace(
    /(<span class="status-slot" data-status="alpha">.*?status-pill )status-unknown/s,
    '$1status-down',
  );
  await page.locator('.status-slot[data-status="alpha"] .status-pill').focus();
  await tick();
  eq(await pillOf('alpha'), 'status-pill status-up', "the focused pill's state");
  eq(
    await page.evaluate(() => document.activeElement?.className ?? ''),
    'status-pill status-up',
    'what the keyboard is on',
  );

  await page.locator('.card-name').first().focus();
  await tick();
  eq(await pillOf('alpha'), 'status-pill status-down', "alpha's pill");
});

await browser.close();
console.log(failures ? `\n${failures} failed` : '\nall passed');
process.exit(failures ? 1 : 0);
