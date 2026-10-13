// Drives the page geometry no look at the markup can settle: what the layout
// does at the four widths the design is held to, light and dark, with the
// longest words a built-in language has and with the page turned right to left.
// a11y.mjs owns focus, names and contrast; this owns where things land.
//
// Run it with `just test-browser`, which serves the demo config, the long site
// name, every built-in locale, example/ and the states fixture first.
//
// Usage: node scripts/layout.mjs [demo] [longname] [locales] [example] [states]
import { chromium } from "playwright";

const arg = (i, env, port) =>
  (process.argv[i] ?? process.env[env] ?? `http://127.0.0.1:${port}/`).replace(
    /\/?$/,
    "/",
  );
const DEMO = arg(2, "CAIRN_DEMO_URL", 8100);
const LONG = arg(3, "CAIRN_LONGNAME_URL", 8101);
const LOCALES = arg(4, "CAIRN_LOCALES_URL", 8102);
const SITE = arg(5, "CAIRN_URL", 8090);
const STATES = arg(6, "CAIRN_STATES_URL", 8095);

// The widths the design is held to: two phones, two laptops. 44rem is the
// line between them, so a phone here is anything at or under 704px.
const WIDTHS = [360, 390, 1280, 1440];
const PHONES = WIDTHS.filter((w) => w <= 704);
const WIDE = WIDTHS.filter((w) => w > 704);
// Dark twice: from the system, and from the toggle over a light system, since
// the stylesheet reaches dark by two selectors and either can be the one a
// rule forgot.
const THEMES = [
  { name: "light", scheme: "light" },
  { name: "dark", scheme: "dark" },
  { name: "dark by the toggle", scheme: "light", stored: "dark" },
];

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

const browser = await chromium.launch();

// A context per width and theme. The stored theme goes in before any page
// script runs, the way a returning visitor's localStorage would.
const contexts = new Map();
const contextFor = async (width, theme, extra = {}) => {
  const key = JSON.stringify([width, theme.name, extra]);
  if (!contexts.has(key)) {
    const ctx = await browser.newContext({
      viewport: { width, height: width > 704 ? 900 : 844 },
      colorScheme: theme.scheme,
      reducedMotion: "reduce",
      ...extra,
    });
    // Nothing leaves the machine. The demo and example/ draw their icons from
    // a CDN, and one stalled request holds the load event for the whole
    // timeout; an icon carries its width and height, so the layout measured
    // is the same without it.
    await ctx.route(
      (u) => u.hostname !== "127.0.0.1",
      (r) => r.abort(),
    );
    if (theme.stored) {
      await ctx.addInitScript((t) => {
        try {
          localStorage.setItem("theme", t);
        } catch (e) {}
      }, theme.stored);
    }
    contexts.set(key, ctx);
  }
  return contexts.get(key);
};
const open = async (url, width, theme = THEMES[0], extra) => {
  const page = await (await contextFor(width, theme, extra)).newPage();
  await page.goto(url);
  await page.evaluate(() => document.fonts.ready);
  return page;
};

// ---- nothing pushes a page sideways ----

// The document's own scroll width: the category trail scrolls inside its own
// row by design, and a scroller of its own does not widen the page.
const PAGES = [
  `${DEMO}fr/`,
  `${DEMO}fr/whoami/`,
  `${DEMO}fr/hosting/`,
  `${DEMO}fr/nowhere/`,
  `${LONG}fr/`,
  ...["de", "es", "pt", "sv", "ar"].flatMap((l) => [
    `${LOCALES}${l}/`,
    `${LOCALES}${l}/pad/`,
    `${LOCALES}${l}/hosting/`,
  ]),
];
for (const theme of THEMES) {
  for (const width of WIDTHS) {
    await check(
      `no page overflows at ${width}px in ${theme.name}`,
      async () => {
        const wide = [];
        for (const url of PAGES) {
          const p = await open(url, width, theme);
          const over = await p.evaluate(
            () =>
              document.documentElement.scrollWidth -
              document.documentElement.clientWidth,
          );
          if (over > 0) wide.push(`${url} by ${over}px`);
          await p.close();
        }
        if (wide.length) throw new Error(`pushed sideways: ${wide.join(", ")}`);
      },
    );
  }
}

// ---- what stays as it is ----

// A header with neither links nor a search box has no menu row to paint. The
// states fixture is that shape: search off, no links.
for (const width of WIDE) {
  await check(`a bare header paints no menu row at ${width}px`, async () => {
    const p = await open(`${STATES}en/`, width);
    const painted = await p.$eval(".menu", (m) => m.getClientRects().length);
    await p.close();
    if (painted) throw new Error("the empty menu row is painted");
  });
}

// A card with neither a hosting flag nor a pill draws no foot: example/ has no
// monitor, and its photos card carries no flag.
await check(
  "a card with nothing to say at its foot draws no foot row",
  async () => {
    for (const width of WIDTHS) {
      const p = await open(`${SITE}en/`, width);
      const h = await p.$$eval(".card", (cards) =>
        cards
          .filter((c) => !c.querySelector(".flag, .status-pill"))
          .map(
            (c) => c.querySelector(".card-foot")?.getClientRects().length ?? 0,
          ),
      );
      await p.close();
      if (h.length === 0)
        throw new Error(
          "no card without a flag or a pill on this page to check",
        );
      if (h.some((n) => n > 0))
        throw new Error(`a foot is painted around nothing at ${width}px`);
    }
  },
);

// Wide, the detail head reads as it always has: Open beside the name, at the
// end of the same row.
for (const width of WIDE) {
  await check(
    `the wide detail head keeps Open beside the name at ${width}px`,
    async () => {
      const p = await open(`${DEMO}fr/whoami/`, width);
      const m = await p.evaluate(() => {
        const r = (s) => document.querySelector(s).getBoundingClientRect();
        const h1 = r(".detail-top h1"),
          btn = r(".detail-top .btn");
        return {
          sameRow:
            Math.abs(h1.top + h1.height / 2 - (btn.top + btn.height / 2)) < 12,
          after: btn.left > h1.left,
        };
      });
      await p.close();
      if (!m.sameRow || !m.after)
        throw new Error(`Open is not beside the name: ${JSON.stringify(m)}`);
    },
  );
}

// A service that is not open has no button to open it, at every width, and
// its state still sits on the line under the name.
await check("a service that is not open shows no Open button", async () => {
  for (const width of WIDTHS) {
    const p = await open(`${LOCALES}en/soon/`, width);
    const m = await p.evaluate(() => {
      const r = (s) => document.querySelector(s)?.getBoundingClientRect();
      const h1 = r(".detail-top h1"),
        state = r(".detail-state");
      return {
        btn: !!document.querySelector(".detail-top .btn"),
        below: !!state && state.top >= h1.bottom - 1,
      };
    });
    await p.close();
    if (m.btn)
      throw new Error(
        `an Open button at ${width}px on a service that is not open`,
      );
    if (!m.below)
      throw new Error(
        `the state is not on the line under the name at ${width}px`,
      );
  }
});

for (const ctx of contexts.values()) await ctx.close();
await browser.close();
if (failures) {
  console.log(`\n${failures} failed`);
  process.exit(1);
}
console.log("\nall layout checks passed");
