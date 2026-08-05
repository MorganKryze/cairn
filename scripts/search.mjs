// Drives the search in a real browser against a running cairn. The Go tests
// assert the markup that ships, not what happens once someone types into it.
//
// Run it with `just test-browser`, which builds cairn and serves the example
// config on a scratch port first.
//
// Usage: node scripts/search.mjs [url]
import { chromium } from 'playwright';

const URL = process.argv[2] ?? process.env.CAIRN_URL ?? 'http://127.0.0.1:8090/en/';

const browser = await chromium.launch();
const page = await browser.newPage();
await page.goto(URL);

const q = page.locator('#q');
const empty = page.locator('#empty');
// Painted, not merely marked. `.card:not([hidden])` reads the attribute, and
// the attribute was right the whole time an author `display` rule overrode
// [hidden] and left those cards on screen. getClientRects asks what the layout
// did.
const painted = sel =>
  page.$$eval(sel, els =>
    els.filter(e => e.getClientRects().length > 0).map(e => e.textContent.trim()),
  );
const visible = async () => (await painted('.card')).length;
const names = () => painted('.card .card-name');

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

const total = await visible();
if (total < 2) throw new Error(`the page under test has ${total} cards; it needs several`);
const first = (await names())[0];
const word = first.trim().split(/\s+/)[0].toLowerCase();

await check('an untouched page shows every card and no message', async () => {
  eq(await visible(), total, 'cards');
  eq(await empty.isHidden(), true, 'the empty message is hidden');
});

// What the live region says and what the page shows have to be the same
// number. With the [hidden] override in place, filtering on a category that
// also held a non-match announced "1 result" and painted two, and a
// count-is-smaller check still passed.
await check('the page shows exactly as many cards as it announces', async () => {
  await q.fill(word);
  const said = (await page.locator('#count').textContent()).trim();
  const announced = Number(said.match(/\d+/)?.[0]);
  if (!Number.isFinite(announced)) throw new Error(`the live region said ${JSON.stringify(said)}`);
  eq(await visible(), announced, `cards painted vs announced in ${JSON.stringify(said)}`);
  await q.fill('');
});

await check('a query that matches narrows the list', async () => {
  await q.fill(word);
  const n = await visible();
  if (n === 0 || n >= total) throw new Error(`${n} cards visible out of ${total}`);
  eq(await empty.isHidden(), true, 'the empty message is hidden');
});

await check('a query that matches nothing says so', async () => {
  await q.fill('ejbhdoehfauhefouah');
  eq(await visible(), 0, 'cards');
  eq(await empty.isHidden(), false, 'the empty message is shown');
});

// Clearing the box is a return to the full list, not a search that found
// nothing. It used to leave "no results" on screen, and announce it.
await check('clearing the box restores every card, with no message', async () => {
  await q.fill('ejbhdoehfauhefouah');
  await q.fill('');
  eq(await visible(), total, 'cards');
  eq(await empty.isHidden(), true, 'the empty message is hidden');
});

await check('so does clearing a query that did match', async () => {
  await q.fill(word);
  await q.fill('');
  eq(await visible(), total, 'cards');
  eq(await empty.isHidden(), true, 'the empty message is hidden');
});

await check('whitespace alone is not a search', async () => {
  await q.fill('   ');
  eq(await visible(), total, 'cards');
  eq(await empty.isHidden(), true, 'the empty message is hidden');
});

await check('Escape clears the field and the filtering with it', async () => {
  await q.fill(word);
  await q.press('Escape');
  eq(await q.inputValue(), '', 'the field');
  eq(await visible(), total, 'cards');
  eq(await empty.isHidden(), true, 'the empty message is hidden');
});

await check('a name match is selected, and reads first', async () => {
  await q.fill(word);
  const sel = page.locator('.card.sel');
  eq(await sel.count(), 1, 'selected cards');
  const picked = (await sel.locator('.card-name').textContent()).trim();
  if (!picked.toLowerCase().includes(word)) throw new Error(`selected ${picked}, which does not match ${word}`);
});

await check('the live region reports the count', async () => {
  await q.fill(word);
  const said = await page.locator('#count').textContent();
  if (!said.trim()) throw new Error('the live region stayed empty');
});

await check('and falls silent when the box is cleared', async () => {
  await q.fill(word);
  await q.fill('');
  eq((await page.locator('#count').textContent()).trim(), '', 'the live region');
});

await browser.close();
console.log(failures ? `\n${failures} failed` : '\nall passed');
process.exit(failures ? 1 : 0);
