// Drives the page geometry no look at the markup can settle: what the layout
// does at the four widths the design is held to, light and dark, with the
// longest words a built-in language has and with the page turned right to left.
// a11y.mjs owns focus and contrast; this owns where things land, and the one
// name check that needs the pages only this script serves: the language menu
// and the phone's menu button say the words they show.
//
// Run it with `just test-browser`, which serves the demo config, the long site
// name, every built-in locale, example/ and the states fixture first.
//
// Usage: node scripts/layout.mjs [demo] [longname] [locales] [example] [states]
import { readFileSync } from "node:fs";
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

// ---- the header ----

// The site's mark is 34px high at every width; the name is 1.5rem wide and
// keeps 1.3rem on a phone.
await check(
  "the mark is 34px high and the name sized for the width",
  async () => {
    for (const url of [`${DEMO}fr/`, `${LONG}fr/`]) {
      for (const width of WIDTHS) {
        const p = await open(url, width);
        const m = await p.evaluate(() => ({
          logo: document.querySelector(".brand img").getBoundingClientRect()
            .height,
          size: parseFloat(
            getComputedStyle(document.querySelector(".brand")).fontSize,
          ),
        }));
        await p.close();
        if (Math.abs(m.logo - 34) > 0.5)
          throw new Error(
            `the mark is ${m.logo}px high at ${width}px on ${url}`,
          );
        const want = width > 704 ? 24 : 20.8;
        if (Math.abs(m.size - want) > 0.05)
          throw new Error(
            `the name is ${m.size}px at ${width}px on ${url}, not ${want}px`,
          );
      }
    }
  },
);

// Wide, the header rule runs on the content column only, opened by an 8px
// diamond in the site's accent; the footer rule spans the same column.
// The content column is where main's .wrap lays its content.
const rules = () => {
  const w = document.querySelector("main .wrap");
  const wr = w.getBoundingClientRect(),
    wcs = getComputedStyle(w);
  const col = {
    start: wr.left + parseFloat(wcs.paddingLeft),
    end: wr.right - parseFloat(wcs.paddingRight),
  };
  const menu = document.querySelector(".menu");
  const mr = menu.getBoundingClientRect();
  const before = getComputedStyle(menu, "::before"),
    after = getComputedStyle(menu, "::after");
  const foot = document.querySelector(".foot");
  const fr = foot.getBoundingClientRect();
  const fb = getComputedStyle(foot, "::before");
  const probe = document.createElement("i");
  probe.style.color = "var(--accent)";
  document.body.append(probe);
  const accent = getComputedStyle(probe).color;
  probe.remove();
  return {
    col,
    menuBorder: parseFloat(getComputedStyle(menu).borderTopWidth),
    rule:
      before.content === "none"
        ? null
        : {
            start: mr.left + parseFloat(before.left),
            end: mr.right - parseFloat(before.right),
            line: parseFloat(before.borderTopWidth),
          },
    diamond:
      after.content === "none"
        ? null
        : {
            start: mr.left + parseFloat(after.left),
            width: parseFloat(after.width),
            height: parseFloat(after.height),
            transform: after.transform,
            color: after.backgroundColor,
          },
    accent,
    footBorder: parseFloat(
      getComputedStyle(document.querySelector("footer")).borderTopWidth,
    ),
    footRule:
      fb.content === "none"
        ? null
        : {
            start: fr.left + parseFloat(fb.left),
            end: fr.right - parseFloat(fb.right),
            line: parseFloat(fb.borderTopWidth),
          },
  };
};
for (const width of WIDE) {
  await check(
    `the header rule opens with the accent diamond on the content column at ${width}px`,
    async () => {
      const p = await open(`${DEMO}fr/`, width);
      const m = await p.evaluate(rules);
      await p.close();
      const near = (a, b) => Math.abs(a - b) <= 1;
      if (m.menuBorder)
        throw new Error("the menu row still draws its own border");
      if (!m.diamond) throw new Error("no diamond opens the header rule");
      if (!m.rule || m.rule.line !== 1)
        throw new Error("no 1px header rule on the column");
      if (!near(m.diamond.start, m.col.start))
        throw new Error(
          `the diamond starts at ${m.diamond.start}, the column at ${m.col.start}`,
        );
      if (!near(m.rule.end, m.col.end))
        throw new Error(
          `the rule ends at ${m.rule.end}, the column at ${m.col.end}`,
        );
      if (m.rule.start < m.diamond.start + m.diamond.width)
        throw new Error("the rule runs under the diamond");
      if (m.diamond.width !== 8 || m.diamond.height !== 8)
        throw new Error(
          `the diamond is ${m.diamond.width}x${m.diamond.height}`,
        );
      if (!/^matrix\(0\.707/.test(m.diamond.transform))
        throw new Error(`the diamond is not turned: ${m.diamond.transform}`);
      if (m.diamond.color !== m.accent)
        throw new Error(
          `the diamond is ${m.diamond.color}, the accent ${m.accent}`,
        );
      const want = { 1280: [129, 1137], 1440: [209, 1217] }[width];
      if (!near(m.col.start, want[0]) || !near(m.col.end, want[1]))
        throw new Error(`the column is ${m.col.start} to ${m.col.end}`);
      if (m.footBorder)
        throw new Error("the footer still draws a full-width border");
      if (!m.footRule || m.footRule.line !== 1)
        throw new Error("no 1px footer rule");
      if (
        !near(m.footRule.start, m.col.start) ||
        !near(m.footRule.end, m.col.end)
      )
        throw new Error(
          `the footer rule spans ${m.footRule.start} to ${m.footRule.end}`,
        );
    },
  );
}

// theme.accent reaches the diamond in every theme.
await check(
  "the diamond takes the site's own accent in every theme",
  async () => {
    for (const theme of THEMES) {
      const p = await open(`${LOCALES}en/`, 1280, theme);
      const m = await p.evaluate(rules);
      await p.close();
      if (m.diamond?.color !== "rgb(138, 79, 125)")
        throw new Error(`the diamond is ${m.diamond?.color} in ${theme.name}`);
    }
  },
);

// Where a name fits in two lines beside the tools, the brand, the theme
// toggle and the languages share one row on a phone.
const centres = () => {
  const mid = (s) => {
    const r = document.querySelector(s).getBoundingClientRect();
    return r.top + r.height / 2;
  };
  return [mid(".brand"), mid(".theme-toggle"), mid(".langs")];
};
for (const width of PHONES) {
  await check(
    `the brand and the tools share one row at ${width}px`,
    async () => {
      for (const url of [`${DEMO}fr/`, `${SITE}fr/`]) {
        const p = await open(url, width);
        const c = await p.evaluate(centres);
        await p.close();
        if (Math.max(...c) - Math.min(...c) > 4)
          throw new Error(`centres ${c.join(", ")} on ${url}`);
      }
    },
  );
}

// A name too long for two lines beside the tools takes the whole row, whole
// and on at most two lines, with the tools on a row below it; the header is
// no taller than it was before the name could move.
const brandLines = () => {
  const b = document.querySelector(".brand");
  const texts = [...b.childNodes].filter(
    (n) => n.nodeType === 3 && n.data.trim(),
  );
  const r = document.createRange();
  r.setStart(texts[0], 0);
  const last = texts[texts.length - 1];
  r.setEnd(last, last.length);
  const tops = [...r.getClientRects()].map((x) => x.top).sort((a, c) => a - c);
  let lines = 0,
    prev = -Infinity;
  for (const t of tops) {
    if (t - prev > 2) lines++;
    prev = t;
  }
  const br = b.getBoundingClientRect(),
    tr = document.querySelector(".topbar-tools").getBoundingClientRect();
  return {
    lines,
    // Every piece of the text inside the brand's box and off the tools: a
    // word with no break in it can outgrow the box and run under them.
    clear: [...r.getClientRects()].every(
      (x) =>
        x.left >= br.left - 1 &&
        x.right <= br.right + 1 &&
        !(
          x.left < tr.right &&
          x.right > tr.left &&
          x.top < tr.bottom &&
          x.bottom > tr.top
        ),
    ),
    whole:
      b.scrollWidth <= b.clientWidth &&
      b.scrollHeight <= b.clientHeight &&
      getComputedStyle(b).textOverflow !== "ellipsis",
    toolsBelow: tr.top >= br.bottom - 1,
    beside: tr.top < br.bottom && tr.bottom > br.top && tr.left >= br.right - 1,
    header: document.querySelector("header").getBoundingClientRect().height,
    over:
      document.documentElement.scrollWidth -
      document.documentElement.clientWidth,
  };
};
for (const width of PHONES) {
  await check(
    `a long site name gets its own row, whole, at ${width}px`,
    async () => {
      const p = await open(`${LONG}fr/`, width);
      const m = await p.evaluate(brandLines);
      await p.close();
      if (!m.whole) throw new Error("the name is cut");
      if (m.lines > 2) throw new Error(`the name takes ${m.lines} lines`);
      if (!m.toolsBelow)
        throw new Error("the tools are not on a row below the name");
      if (m.header > 215) throw new Error(`the header is ${m.header}px high`);
    },
  );
}

// A name with nowhere to break, a host name or a compound word, is wider than
// its share of the row in two lines or one. It takes the whole row too, rather
// than running under the tools. The title is swapped in the page and the
// header measured again, as a resize would.
const rename = (name) => {
  const b = document.querySelector(".brand");
  const texts = [...b.childNodes].filter(
    (n) => n.nodeType === 3 && n.data.trim(),
  );
  texts[0].data = name;
  for (const t of texts.slice(1)) t.remove();
  dispatchEvent(new Event("resize"));
};
// The resize handler measures on the next frame.
const settle = () =>
  new Promise((done) =>
    requestAnimationFrame(() => requestAnimationFrame(done)),
  );
for (const width of PHONES) {
  await check(
    `a name with no break in it gets its own row at ${width}px`,
    async () => {
      const bad = [];
      for (const name of ["dash.morgankryze.com", "Familienselbsthosting"]) {
        const p = await open(`${LONG}fr/`, width);
        await p.evaluate(rename, name);
        await p.evaluate(settle);
        const m = await p.evaluate(brandLines);
        await p.close();
        if (!m.clear) bad.push(`${name} runs under the tools`);
        if (!m.whole) bad.push(`${name} is cut`);
        if (m.lines > 1) bad.push(`${name} takes ${m.lines} lines`);
        if (!m.toolsBelow) bad.push(`the tools sit beside ${name}`);
      }
      if (bad.length) throw new Error(bad.join("; "));
    },
  );
}
await check(
  "without JavaScript a long name stays beside the tools at 360px",
  async () => {
    const p = await open(`${LONG}fr/`, 360, THEMES[0], {
      javaScriptEnabled: false,
    });
    const m = await p.evaluate(brandLines);
    await p.close();
    if (!m.whole) throw new Error("the name is cut");
    if (!m.beside) throw new Error("the tools are not beside the name");
    if (m.header > 215) throw new Error(`the header is ${m.header}px high`);
  },
);

// A word wider than the whole phone row breaks inside the name rather than
// pushing the page sideways, and without JavaScript, where nothing can give
// the name its own row, any word too wide for its share breaks beside the
// tools rather than running under them.
const LONG_WORD = "Familiendienstverwaltungsportal Müller";
for (const width of PHONES) {
  await check(
    `a word wider than the row breaks inside the name at ${width}px`,
    async () => {
      const p = await open(`${LONG}fr/`, width);
      await p.evaluate(rename, LONG_WORD);
      await p.evaluate(settle);
      const m = await p.evaluate(brandLines);
      await p.close();
      const bad = [];
      if (!m.clear) bad.push("the name runs out of its box");
      if (!m.whole) bad.push("the name is cut");
      if (m.over > 0) bad.push(`the page is pushed sideways by ${m.over}px`);
      if (!m.toolsBelow) bad.push("the tools sit beside the name");
      if (m.header > 215) bad.push(`the header is ${m.header}px high`);
      if (bad.length) throw new Error(bad.join("; "));
    },
  );
  await check(
    `without JavaScript a name with no break in it stays clear of the tools at ${width}px`,
    async () => {
      const bad = [];
      for (const name of ["dash.morgankryze.com", LONG_WORD]) {
        const p = await open(`${LONG}fr/`, width, THEMES[0], {
          javaScriptEnabled: false,
        });
        await p.evaluate(rename, name);
        const m = await p.evaluate(brandLines);
        await p.close();
        if (!m.clear) bad.push(`${name} runs under the tools`);
        if (!m.whole) bad.push(`${name} is cut`);
        if (m.over > 0) bad.push(`${name} pushes the page by ${m.over}px`);
        if (m.header > 215) bad.push(`${name} makes the header ${m.header}px`);
      }
      if (bad.length) throw new Error(bad.join("; "));
    },
  );
}

// Every header control is a 44px target: the theme toggle, the language
// links, the language menu and the header links or the menu button.
for (const width of WIDTHS) {
  await check(`every header control is 44px at ${width}px`, async () => {
    const small = [];
    for (const url of [`${DEMO}fr/`, `${SITE}fr/`, `${LOCALES}en/`]) {
      const p = await open(url, width);
      small.push(
        ...(
          await p.$$eval(
            ".theme-toggle, .langs > a, .langs-menu > summary, .menu-link, .nav-burger",
            (els) =>
              els
                .filter((e) => e.checkVisibility({ visibilityProperty: true }))
                .map((e) => {
                  const r = e.getBoundingClientRect();
                  return {
                    e: e.className || e.textContent.trim(),
                    w: r.width,
                    h: r.height,
                  };
                })
                .filter((t) => t.w < 43.99 || t.h < 43.99)
                .map((t) => `${t.e} ${Math.round(t.w)}x${Math.round(t.h)}`),
          )
        ).map((s) => `${s} on ${url}`),
      );
      await p.close();
    }
    if (small.length) throw new Error(small.join("; "));
  });
}

// A control's name carries the words it shows (WCAG 2.5.3): a speech user
// says what they see. Only an aria-label can drift from the text.
await check(
  "every labelled control's name contains its visible text",
  async () => {
    const bad = [];
    for (const width of WIDTHS) {
      for (const url of PAGES) {
        const p = await open(url, width);
        bad.push(
          ...(
            await p.$$eval(
              "a[aria-label], button[aria-label], summary[aria-label]",
              (els) =>
                els
                  .filter((e) =>
                    e.checkVisibility({ visibilityProperty: true }),
                  )
                  .map((e) => ({
                    name: e.getAttribute("aria-label").toLowerCase(),
                    text: e.innerText.replace(/\s+/g, " ").trim().toLowerCase(),
                    cls: e.className || e.tagName,
                  }))
                  .filter((t) => t.text && !t.name.includes(t.text))
                  .map((t) => `${t.cls} shows "${t.text}" named "${t.name}"`),
            )
          ).map((s) => `${s} at ${width}px on ${url}`),
        );
        await p.close();
      }
    }
    if (bad.length) throw new Error([...new Set(bad)].slice(0, 8).join("; "));
  },
);

// ---- the welcome note folds on a phone ----

// A phone shows the note as one row and opens it on demand, so the first card
// is on the first screen. The fold is a <details>: it works with no script,
// remembers nothing, and a wide screen or a printout shows the note whole.
const FOLDS = [
  [`${DEMO}fr/`, "À propos de ce site"],
  [`${LOCALES}de/`, "Über diese Seite"],
];
const fold = () => {
  const sum = document.querySelector(".about-sum");
  const body = document.querySelector(".about-fold > :not(summary)");
  const r = sum.getBoundingClientRect();
  return {
    open: document.querySelector(".about-fold").open,
    row: sum.checkVisibility(),
    words: sum.innerText.trim(),
    height: r.height,
    shown: body.checkVisibility(),
    section: document.querySelector(".about").checkVisibility(),
  };
};
// A context of its own for whatever writes state, so a dismissed note or a
// cookie never leaks into the checks that share the cached contexts.
const fresh = async (width, extra = {}) => {
  const ctx = await browser.newContext({
    viewport: { width, height: width > 704 ? 900 : 844 },
    reducedMotion: "reduce",
    ...extra,
  });
  await ctx.route(
    (u) => u.hostname !== "127.0.0.1",
    (r) => r.abort(),
  );
  return ctx;
};

for (const width of PHONES) {
  await check(
    `the welcome note arrives folded to one row at ${width}px`,
    async () => {
      const bad = [];
      for (const [url, word] of FOLDS) {
        const p = await open(url, width);
        const f = await p.evaluate(fold);
        await p.close();
        if (f.open) bad.push(`${url} arrives open`);
        if (!f.row) bad.push(`${url} shows no row`);
        if (f.words !== word)
          bad.push(`${url} reads "${f.words}", want "${word}"`);
        if (f.height > 46) bad.push(`${url} row is ${f.height}px high`);
        if (f.shown) bad.push(`${url} shows the folded note`);
      }
      if (bad.length) throw new Error(bad.join("; "));
    },
  );
  for (const js of [true, false]) {
    await check(
      `a click opens the note and a second folds it at ${width}px, JavaScript ${js ? "on" : "off"}`,
      async () => {
        const ctx = await fresh(width, { javaScriptEnabled: js });
        const p = await ctx.newPage();
        await p.goto(`${DEMO}fr/`);
        await p.click(".about-sum");
        const opened = await p.evaluate(fold);
        await p.click(".about-sum");
        const closed = await p.evaluate(fold);
        await ctx.close();
        const bad = [];
        if (!opened.open || !opened.shown)
          bad.push("the first click does not show the note");
        if (!opened.row) bad.push("the row is gone once open");
        if (closed.open || closed.shown)
          bad.push("the second click does not fold it");
        if (bad.length) throw new Error(bad.join("; "));
      },
    );
  }
  await check(
    `Enter opens the note and Space folds it at ${width}px`,
    async () => {
      const p = await open(`${DEMO}fr/`, width);
      await p.focus(".about-sum");
      await p.keyboard.press("Enter");
      const opened = await p.evaluate(fold);
      await p.keyboard.press("Space");
      const closed = await p.evaluate(fold);
      await p.close();
      if (!opened.open || !opened.shown)
        throw new Error("Enter does not open the note");
      if (closed.open || closed.shown)
        throw new Error("Space does not fold the note");
    },
  );
  await check(
    `folding the note writes no cookie and no storage at ${width}px`,
    async () => {
      const ctx = await fresh(width);
      const p = await ctx.newPage();
      await p.goto(`${DEMO}fr/`);
      await p.click(".about-sum");
      await p.click(".about-sum");
      await p.click(".about-sum");
      const kept = await p.evaluate(() => ({
        cookie: document.cookie,
        storage: localStorage.length,
      }));
      const cookies = await ctx.cookies();
      await p.reload();
      const again = await p.evaluate(fold);
      await ctx.close();
      const bad = [];
      if (kept.cookie || cookies.length)
        bad.push(`cookies written: ${cookies.map((c) => c.name).join(", ")}`);
      if (kept.storage) bad.push(`${kept.storage} storage entries written`);
      if (again.open || again.shown)
        bad.push("the note comes back open after a reload");
      if (bad.length) throw new Error(bad.join("; "));
    },
  );
  await check(
    `without JavaScript there is no dismiss button at ${width}px`,
    async () => {
      const p = await open(`${DEMO}fr/`, width, THEMES[0], {
        javaScriptEnabled: false,
      });
      const shown = await p.$eval("#about-x", (x) => x.checkVisibility());
      await p.close();
      if (shown) throw new Error("a button that cannot work is on the page");
    },
  );
  await check(
    `printed at ${width}px the note is whole and the row hidden`,
    async () => {
      const p = await open(`${DEMO}fr/`, width);
      await p.emulateMedia({ media: "print" });
      const f = await p.evaluate(fold);
      await p.close();
      if (f.row) throw new Error("the row is printed");
      if (!f.shown) throw new Error("the folded note is left off the paper");
    },
  );
}

for (const width of WIDE) {
  await check(
    `a wide screen shows the note whole and no row at ${width}px`,
    async () => {
      const bad = [];
      for (const [url] of FOLDS) {
        const p = await open(url, width);
        const f = await p.evaluate(fold);
        await p.close();
        if (f.row) bad.push(`${url} shows the row`);
        if (!f.shown) bad.push(`${url} hides the note`);
      }
      if (bad.length) throw new Error(bad.join("; "));
    },
  );
}

for (const width of WIDTHS) {
  await check(
    `dismissing hides the whole note, and it stays hidden after a reload, at ${width}px`,
    async () => {
      const ctx = await fresh(width);
      const p = await ctx.newPage();
      await p.goto(`${DEMO}fr/`);
      await p.click("#about-x");
      const now = await p.evaluate(fold);
      await p.reload();
      const later = await p.evaluate(fold);
      await ctx.close();
      if (now.section) throw new Error("the note is still on the page");
      if (later.section) throw new Error("the note is back after a reload");
    },
  );
}

// The point of the fold: on the demo at 390 by 844 the first card sat 800px
// down with the note open, under the bottom of the first screen.
await check(
  "the first card starts within the first half screen at 390 by 844",
  async () => {
    const p = await open(`${DEMO}fr/`, 390);
    const top = await p.$eval(
      ".card",
      (c) => c.getBoundingClientRect().top + scrollY,
    );
    await p.close();
    if (top > 420)
      throw new Error(`the first card starts at ${top.toFixed(1)}px`);
  },
);

// ---- the card foot ----

// The hosting word is the point of the flag, so it is painted at rest, with
// no hover and with motion allowed: a word clipped behind a fold was invisible
// on a phone until a tap, and a tap on a card leaves the page.
const NO_FOLD = { reducedMotion: "no-preference" };
const FOOTED = [`${DEMO}fr/`, `${LOCALES}en/`, `${LOCALES}ar/`];
for (const width of WIDTHS) {
  await check(`the hosting word shows at rest at ${width}px`, async () => {
    const bad = [];
    for (const url of FOOTED) {
      const p = await open(url, width, THEMES[0], NO_FOLD);
      bad.push(
        ...(
          await p.$$eval(".card .flag", (flags) =>
            flags.map((f) => {
              const l = f.querySelector(".flag-label");
              const fr = f.getBoundingClientRect(),
                lr = l.getBoundingClientRect();
              const opacity = Number(getComputedStyle(l).opacity);
              const inside =
                lr.width > 0 &&
                lr.left >= fr.left - 0.5 &&
                lr.right <= fr.right + 0.5;
              return opacity === 1 && inside
                ? null
                : `"${l.textContent}" opacity ${opacity}, ${inside ? "inside" : "clipped by"} its flag`;
            }),
          )
        )
          .filter(Boolean)
          .map((s) => `${s} on ${url}`),
      );
      await p.close();
    }
    if (bad.length) throw new Error([...new Set(bad)].slice(0, 6).join("; "));
  });
}

// The foot is a row of its own under the icon and the text, from the card's
// content start to its content end: under the text alone it left the two
// chips 54px short, and German no longer fitted one row at 360.
const footBox = () =>
  [...document.querySelectorAll(".card")]
    .filter((c) => c.querySelector(".card-foot")?.getClientRects().length)
    .map((c) => {
      const box = (el) => {
        const r = el.getBoundingClientRect(),
          s = getComputedStyle(el);
        return {
          left:
            r.left + parseFloat(s.borderLeftWidth) + parseFloat(s.paddingLeft),
          right:
            r.right -
            parseFloat(s.borderRightWidth) -
            parseFloat(s.paddingRight),
          top: r.top,
          bottom: r.bottom,
        };
      };
      const card = box(c),
        foot = box(c.querySelector(".card-foot")),
        tile = c.querySelector(".tile").getBoundingClientRect(),
        text = c.querySelector(".card-text").getBoundingClientRect();
      const name = c.querySelector(".card-name").textContent.trim();
      const bad = [];
      if (Math.abs(foot.left - card.left) > 0.5)
        bad.push(`starts ${(foot.left - card.left).toFixed(1)}px off`);
      if (Math.abs(foot.right - card.right) > 0.5)
        bad.push(`ends ${(card.right - foot.right).toFixed(1)}px off`);
      if (foot.top < Math.max(tile.bottom, text.bottom) - 0.5)
        bad.push("rises beside the icon or the text");
      return bad.length ? `${name}: ${bad.join(", ")}` : null;
    })
    .filter(Boolean);
for (const width of WIDTHS) {
  await check(
    `the card foot spans the card under the icon and the text at ${width}px`,
    async () => {
      const bad = [];
      for (const url of FOOTED) {
        const p = await open(url, width);
        bad.push(...(await p.evaluate(footBox)).map((s) => `${s} on ${url}`));
        await p.close();
      }
      if (bad.length) throw new Error(bad.slice(0, 6).join("; "));
    },
  );
}

// Every hosting word beside every status word of the same language keeps the
// foot on one row at 360: the longest pair, German's "Selbst gehostet" and
// "Beeinträchtigt", is the one a narrower foot pushed onto two.
const FOOT_WORDS = JSON.parse(
  readFileSync(new URL("fixtures/locales/foot-words.json", import.meta.url)),
);
// The first card carrying both chips, its words swapped for the pair.
const setFoot = ([host, status]) => {
  const foot = [...document.querySelectorAll(".card-foot")].find(
    (f) =>
      f.querySelector(".flag .flag-label") && f.querySelector(".status-pill"),
  );
  foot.querySelector(".flag-label").textContent = host;
  const pill = foot.querySelector(".status-pill");
  pill.lastChild.data = status;
  const f = foot.getBoundingClientRect(),
    a = foot.querySelector(".flag").getBoundingClientRect(),
    b = pill.getBoundingClientRect();
  return {
    oneRow: Math.abs(a.top - b.top) < 1,
    inside:
      Math.min(a.left, b.left) >= f.left - 0.5 &&
      Math.max(a.right, b.right) <= f.right + 0.5,
  };
};
await check(
  "every hosting and status word of a language share one foot row at 360px",
  async () => {
    const bad = [];
    for (const [loc, words] of Object.entries(FOOT_WORDS)) {
      const p = await open(`${LOCALES}${loc}/`, 360);
      for (const host of words.host) {
        for (const status of words.status) {
          const m = await p.evaluate(setFoot, [host, status]);
          if (!m.oneRow || !m.inside)
            bad.push(
              `${loc} "${host}" + "${status}" ${m.oneRow ? "overflows" : "takes two rows"}`,
            );
        }
      }
      await p.close();
    }
    if (bad.length) throw new Error(bad.slice(0, 6).join("; "));
  },
);

// Where the two chips do not fit, the second moves to a row of its own, whole:
// each chip stays one line, its glyph beside its word. The foot is narrowed in
// the page to just past its wider chip, which no real card is, so the wrap is
// forced whatever the words; then to 40px, narrower than either chip, where
// a chip free to break would put "Selbst gehostet" on two lines.
const wrapFoot = () => {
  const out = [];
  for (const foot of document.querySelectorAll(".card-foot")) {
    const flag = foot.querySelector(".flag"),
      pill = foot.querySelector(".status-pill");
    if (!flag || !pill) continue;
    const h0 = [flag, pill].map((e) => e.getBoundingClientRect().height);
    const w = Math.max(
      ...[flag, pill].map((e) => e.getBoundingClientRect().width),
    );
    foot.style.maxWidth = `${w + 8}px`;
    const a = flag.getBoundingClientRect(),
      b = pill.getBoundingClientRect();
    const together = (chip, glyph, word) => {
      const r = document.createRange();
      r.selectNodeContents(word);
      const lines = new Set(
        [...r.getClientRects()].map((x) => Math.round(x.top)),
      );
      const g = glyph.getBoundingClientRect(),
        t = r.getBoundingClientRect(),
        c = chip.getBoundingClientRect();
      return (
        lines.size === 1 &&
        g.top < t.bottom &&
        g.bottom > t.top &&
        t.right <= c.right + 0.5 &&
        t.left >= c.left - 0.5
      );
    };
    const bad = [];
    if (b.top < a.bottom - 0.5) bad.push("the second chip did not move down");
    foot.style.maxWidth = "40px";
    const grew = [flag, pill].some(
      (e, i) => Math.abs(e.getBoundingClientRect().height - h0[i]) > 0.5,
    );
    if (Math.abs(a.height - h0[0]) > 0.5 || Math.abs(b.height - h0[1]) > 0.5)
      bad.push("a chip grew a line");
    else if (grew) bad.push("a chip grew a line once squeezed");
    if (
      !together(
        flag,
        flag.querySelector("svg"),
        flag.querySelector(".flag-label"),
      )
    )
      bad.push("the flag broke");
    if (!together(pill, pill.querySelector(".dot"), pill.lastChild))
      bad.push("the pill broke");
    if (bad.length)
      out.push(
        `${foot.closest(".card").querySelector(".card-name").textContent.trim()}: ${bad.join(", ")}`,
      );
  }
  return out;
};
for (const width of WIDTHS) {
  await check(
    `a foot too narrow for both chips moves the second whole at ${width}px`,
    async () => {
      const bad = [];
      for (const url of [`${LOCALES}de/`, `${LOCALES}ar/`]) {
        const p = await open(url, width);
        bad.push(...(await p.evaluate(wrapFoot)).map((s) => `${s} on ${url}`));
        await p.close();
      }
      if (bad.length) throw new Error(bad.slice(0, 6).join("; "));
    },
  );
}

// A long name wraps over balanced lines, whole, and the sign-in lock follows
// its last word on its last line. On a right-to-left page that is the line's
// left end, whatever the direction of the name's own words.
const longName = () => {
  const lock = document.querySelector(".card-lock");
  const name = lock.closest(".card-name");
  const s = getComputedStyle(name);
  const walk = document.createTreeWalker(name, NodeFilter.SHOW_TEXT);
  const words = [];
  for (let n = walk.nextNode(); n; n = walk.nextNode()) {
    if (lock.contains(n) || !n.data.trim()) continue;
    const r = document.createRange();
    r.selectNodeContents(n);
    words.push(...[...r.getClientRects()].filter((x) => x.width > 0));
  }
  const bottom = Math.max(...words.map((x) => x.top));
  const line = words.filter((x) => Math.abs(x.top - bottom) < 1);
  const l = lock.getBoundingClientRect(),
    card = name.closest(".card").getBoundingClientRect();
  const gap =
    s.direction === "rtl"
      ? Math.min(...line.map((x) => x.left)) - l.right
      : l.left - Math.max(...line.map((x) => x.right));
  return {
    wrap: s.textWrapStyle || s.textWrap,
    lines: new Set(words.map((x) => Math.round(x.top))).size,
    whole:
      s.textOverflow !== "ellipsis" &&
      s.webkitLineClamp === "none" &&
      words.every((x) => x.left >= card.left && x.right <= card.right),
    after:
      l.top < line[0].bottom && l.bottom > line[0].top && gap >= 0 && gap < 10,
  };
};
for (const width of WIDTHS) {
  await check(
    `a long card name wraps whole, balanced, with the lock after its last word at ${width}px`,
    async () => {
      const bad = [];
      for (const url of [`${LOCALES}en/`, `${LOCALES}ar/`]) {
        const p = await open(url, width);
        const m = await p.evaluate(longName);
        await p.close();
        if (!/balance/.test(m.wrap)) bad.push(`wraps "${m.wrap}" on ${url}`);
        if (m.lines < 2) bad.push(`one line on ${url}`);
        if (!m.whole) bad.push(`cut on ${url}`);
        if (!m.after) bad.push(`the lock is not after the last word on ${url}`);
      }
      if (bad.length) throw new Error(bad.join("; "));
    },
  );
}

// What the foot row costs: the demo's page at 390 was 2,644px with no card
// over 155px before the foot moved and the hosting word showed.
await check("the demo's home page stays short at 390px", async () => {
  const p = await open(`${DEMO}fr/`, 390);
  const m = await p.evaluate(() => ({
    page: document.documentElement.scrollHeight,
    card: Math.max(
      ...[...document.querySelectorAll(".card")].map(
        (c) => c.getBoundingClientRect().height,
      ),
    ),
  }));
  await p.close();
  const bad = [];
  if (m.page > 2250) bad.push(`the page is ${m.page}px tall`);
  if (m.card > 135) bad.push(`a card is ${m.card.toFixed(1)}px tall`);
  if (bad.length) throw new Error(bad.join("; "));
});

// Each target in a card answers within its own hit area, the box or an
// absolute ::before around it, and no two of them share a point: a tap there
// would go to whichever was painted last. Wrapped too, where the foot's row
// gap is all that parts the two chips' bands.
const overlaps = (wrap) => {
  const hit = (el) => {
    const r = el.getBoundingClientRect(),
      s = getComputedStyle(el),
      b = getComputedStyle(el, "::before");
    const box = { l: r.left, t: r.top, r: r.right, b: r.bottom };
    if (b.content === "none" || b.position !== "absolute") return box;
    const pad = {
      l: r.left + parseFloat(s.borderLeftWidth),
      t: r.top + parseFloat(s.borderTopWidth),
      r: r.right - parseFloat(s.borderRightWidth),
      b: r.bottom - parseFloat(s.borderBottomWidth),
    };
    return {
      l: Math.min(box.l, pad.l + parseFloat(b.left)),
      t: Math.min(box.t, pad.t + parseFloat(b.top)),
      r: Math.max(box.r, pad.r - parseFloat(b.right)),
      b: Math.max(box.b, pad.b - parseFloat(b.bottom)),
    };
  };
  const out = [];
  for (const card of document.querySelectorAll(".card")) {
    const foot = card.querySelector(".card-foot");
    if (wrap && foot) {
      const w = Math.max(
        ...[...foot.querySelectorAll(".flag, .status-pill")].map(
          (e) => e.getBoundingClientRect().width,
        ),
      );
      foot.style.maxWidth = `${w + 8}px`;
    }
    const els = [
      ...card.querySelectorAll(".card-more, a.flag, a.status-pill"),
    ].filter((e) => e.getClientRects().length);
    for (let i = 0; i < els.length; i++) {
      for (let j = i + 1; j < els.length; j++) {
        const a = hit(els[i]),
          b = hit(els[j]);
        const x = Math.min(a.r, b.r) - Math.max(a.l, b.l),
          y = Math.min(a.b, b.b) - Math.max(a.t, b.t);
        if (x > 0.5 && y > 0.5)
          out.push(
            `${els[i].className} and ${els[j].className} share ${x.toFixed(1)}x${y.toFixed(1)}px on ${card.querySelector(".card-name").textContent.trim()}`,
          );
      }
    }
  }
  return out;
};
for (const width of WIDTHS) {
  for (const wrap of [false, true]) {
    await check(
      `no two hit areas in a card overlap at ${width}px${wrap ? ", the foot wrapped" : ""}`,
      async () => {
        const bad = [];
        for (const url of [`${DEMO}fr/`, `${LOCALES}de/`, `${LOCALES}ar/`]) {
          const p = await open(url, width);
          bad.push(
            ...(await p.evaluate(overlaps, wrap)).map((s) => `${s} on ${url}`),
          );
          await p.close();
        }
        if (bad.length) throw new Error(bad.slice(0, 6).join("; "));
      },
    );
  }
}

// A thumb that lands anywhere in the 44px band centred on a linked flag
// follows the flag, not the card around it, which leaves for the service.
// The click is caught and cancelled, so nothing navigates. The centre is read
// again before every tap: the scroll that brings the flag into view moves the
// header, and a centre read once was 3px off by the first tap.
const flagCentre = () => {
  const r = document.querySelector(".card-foot a.flag").getBoundingClientRect();
  return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
};
for (const width of PHONES) {
  await check(
    `a tap within 21px of a linked flag's centre follows the flag at ${width}px`,
    async () => {
      const ctx = await fresh(width, { hasTouch: true, isMobile: true });
      const p = await ctx.newPage();
      await p.goto(`${LOCALES}en/`);
      await p.evaluate(() => {
        addEventListener(
          "click",
          (e) => {
            const a = e.target.closest("a");
            window.tapped = a ? a.className : e.target.tagName;
            e.preventDefault();
          },
          true,
        );
        document
          .querySelector(".card-foot a.flag")
          .scrollIntoView({ block: "center" });
      });
      await p.evaluate(settle);
      const missed = [];
      for (let dy = -21; dy <= 21; dy += 3) {
        const c = await p.evaluate(flagCentre);
        await p.evaluate(() => (window.tapped = null));
        await p.touchscreen.tap(c.x, c.y + dy);
        const got = await p.evaluate(() => window.tapped);
        if (!/\bflag\b/.test(got ?? "")) missed.push(`${dy}px went to ${got}`);
      }
      await ctx.close();
      if (missed.length) throw new Error(missed.join("; "));
    },
  );
}

for (const ctx of contexts.values()) await ctx.close();
await browser.close();
if (failures) {
  console.log(`\n${failures} failed`);
  process.exit(1);
}
console.log("\nall layout checks passed");
