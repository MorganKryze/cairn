// Drives the search in a real browser against a running cairn. The Go tests
// assert the markup that ships, not what happens once someone types into it.
//
// Run it with `just test-browser`, which builds cairn and serves the example
// config on a scratch port first.
//
// Usage: node scripts/search.mjs [url]
import { chromium } from 'playwright';

const URL = process.argv[2] ?? process.env.CAIRN_URL ?? 'http://127.0.0.1:8090/en/';
// example/ has no row a query reorders, so the order checks read this one.
const ORDER_URL = process.argv[3] ?? process.env.CAIRN_MANY_URL ?? 'http://127.0.0.1:8091/en/';

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

// Tab walks the matches like the arrows, with focus kept in the box, and gives
// focus back to the browser at either end. Looping would trap a keyboard user
// in the field, so the two exits are checked as hard as the walk.
const onScreen = () =>
  page.$$eval('.card', els =>
    els
      .filter(e => e.getClientRects().length > 0)
      .map(e => ({ r: e.getBoundingClientRect(), sel: e.classList.contains('sel'), n: e.querySelector('.card-name').textContent.trim() }))
      .sort((a, b) => a.r.top - b.r.top || a.r.left - b.r.left)
      .map(c => ({ n: c.n, sel: c.sel })),
  );
const selIndex = async () => (await onScreen()).findIndex(c => c.sel);
const inBox = () => page.evaluate(() => document.activeElement?.id === 'q');
let many = '';
for (const cand of ['e', 'a', 'o', 'i']) {
  await q.fill(cand);
  if ((await visible()) >= 3) { many = cand; break; }
}

await check('Tab moves the pick to the next match, and the box keeps focus', async () => {
  if (!many) throw new Error('no one-letter query matches three cards, so this proves nothing');
  await q.fill(many);
  const list = await onScreen();
  let i = await selIndex();
  if (i === list.length - 1) { await q.press('Shift+Tab'); i = await selIndex(); }
  await q.press('Tab');
  eq(await selIndex(), i + 1, 'the pick after Tab');
  eq(await inBox(), true, 'focus in the box');
  const said = (await page.locator('#count').textContent()).trim();
  if (!said.endsWith(list[i + 1].n)) throw new Error(`the live region said ${JSON.stringify(said)}, not the new pick`);
});

await check('Shift+Tab moves it back', async () => {
  await q.fill(many);
  let i = await selIndex();
  if (i === 0) { await q.press('Tab'); i = await selIndex(); }
  await q.press('Shift+Tab');
  eq(await selIndex(), i - 1, 'the pick after Shift+Tab');
  eq(await inBox(), true, 'focus in the box');
});

await check('Tab on the last match leaves the box instead of looping', async () => {
  await q.fill(many);
  const n = (await onScreen()).length;
  for (let i = await selIndex(); i < n - 1; i++) await q.press('Tab');
  eq(await selIndex(), n - 1, 'the pick before the last Tab');
  await page.keyboard.press('Tab');
  eq(await inBox(), false, 'focus in the box after Tab on the last match');
  await q.focus();
});

await check('Shift+Tab on the first match leaves the box backwards', async () => {
  await q.fill(many);
  for (let i = await selIndex(); i > 0; i--) await q.press('Shift+Tab');
  eq(await selIndex(), 0, 'the pick before the last Shift+Tab');
  await page.keyboard.press('Shift+Tab');
  eq(await inBox(), false, 'focus in the box after Shift+Tab on the first match');
  await q.focus();
});

await check('with nothing typed, Tab leaves the box at once', async () => {
  await q.fill('');
  await q.focus();
  await page.keyboard.press('Tab');
  eq(await inBox(), false, 'focus in the box');
});

// Keyboard and screen-reader order is the DOM's. A match that moves to the
// front of its row has to move there in the DOM, or Tab and a screen reader
// walk a sequence the screen no longer shows: CSS order did exactly that.
const op = await browser.newPage();
await op.goto(ORDER_URL);
const oq = op.locator('#q');
const domAndScreen = () =>
  op.$$eval('.card', els => {
    const vis = els.filter(e => e.getClientRects().length > 0);
    const name = e => e.querySelector('.card-name').textContent.trim();
    const screen = [...vis].sort((a, b) => {
      const x = a.getBoundingClientRect(), y = b.getBoundingClientRect();
      return x.top - y.top || x.left - y.left;
    });
    return { dom: vis.map(name), screen: screen.map(name) };
  });
const original = (await domAndScreen()).dom;

await check('after any one-letter query, Tab order is screen order', async () => {
  let moved = 0;
  for (const letter of 'abcdefghijklmnopqrstuvwxyz') {
    await oq.fill(letter);
    const { dom, screen } = await domAndScreen();
    eq(dom, screen, `DOM order against screen order for "${letter}"`);
    // A card that now sits ahead of one it used to follow: the case at stake.
    const rank = n => original.indexOf(n);
    if (dom.some((n, i) => i > 0 && rank(n) < rank(dom[i - 1]))) moved++;
  }
  if (!moved) throw new Error('no letter reordered a row, so this proves nothing');
});

// Cleared from a query that actually moved a card. Clearing after one that
// moved nothing finds the original order whether or not anything restores it:
// the first version of this check typed a vowel last and stayed green with
// the restore deleted.
await check('clearing the box puts every card back where it was', async () => {
  const rank = n => original.indexOf(n);
  let moved = '';
  for (const letter of 'abcdefghijklmnopqrstuvwxyz') {
    await oq.fill(letter);
    const { dom } = await domAndScreen();
    if (dom.some((n, i) => i > 0 && rank(n) < rank(dom[i - 1]))) { moved = letter; break; }
  }
  if (!moved) throw new Error('no letter reordered a row, so clearing proves nothing');
  await oq.fill('');
  eq((await domAndScreen()).dom, original, `DOM order after clearing "${moved}"`);
});

await browser.close();
console.log(failures ? `\n${failures} failed` : '\nall passed');
process.exit(failures ? 1 : 0);
