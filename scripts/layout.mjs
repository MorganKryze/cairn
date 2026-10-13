// Drives the page geometry no look at the markup can settle: what the layout
// does at the four widths the design is held to, light and dark, with the
// longest words a built-in language has and with the page turned right to left.
// a11y.mjs owns focus and contrast; this owns where things land, and the one
// name check that needs the pages only this script serves: the language menu
// and the phone's menu button say the words they show.
//
// Run it with `just test-browser`, which serves the demo config, the long site
// name, every built-in locale, example/, the states fixture, the nine
// categories with a header burger and the leaving dialog first.
//
// Usage: node scripts/layout.mjs [demo] [longname] [locales] [example] [states] [many] [leave]
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
const MANY = arg(7, "CAIRN_MANY_URL", 8091);
const LEAVE = arg(8, "CAIRN_LEAVE_URL", 8094);

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
// its state still sits on the line under the name, from the tile's edge.
await check("a service that is not open shows no Open button", async () => {
  for (const width of WIDTHS) {
    const p = await open(`${LOCALES}en/soon/`, width);
    const m = await p.evaluate(() => {
      const r = (s) => document.querySelector(s)?.getBoundingClientRect();
      const h1 = r(".detail-top h1"),
        tile = r(".detail-top .tile"),
        state = r(".detail-state");
      return {
        btn: !!document.querySelector(".detail-top .btn"),
        below: !!state && state.top >= h1.bottom - 1,
        start: state ? state.left - tile.left : NaN,
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
    if (!(Math.abs(m.start) <= 1))
      throw new Error(
        `the state starts ${m.start}px from the tile's edge at ${width}px`,
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

// ---- the search field ----

// The placeholder is the only word in the field, so it shows whole. The
// browser's own overflow says so, with the placeholder put in as the value: a
// count of the room inside the padding misses the clear button a search field
// keeps space for even while it is hidden, which in Chromium cut "Chercher un
// outil…" to "outil.." with 130.6px of padding-box room for its 122.7.
// The room is measured the same way, as what a long value overflows by.
const searchField = () => {
  const i = document.querySelector(".search-box input");
  const s = getComputedStyle(i);
  const ctx = document.createElement("canvas").getContext("2d");
  ctx.font = `${s.fontStyle} ${s.fontWeight} ${s.fontSize} ${s.fontFamily}`;
  i.value = "m".repeat(60);
  const room = i.clientWidth - (i.scrollWidth - ctx.measureText(i.value).width);
  i.value = i.placeholder;
  const whole = i.scrollWidth <= i.clientWidth;
  i.value = "";
  return {
    placeholder: i.placeholder,
    room,
    whole,
    text: ctx.measureText(i.placeholder).width,
    ellipsis: s.textOverflow,
    hint: [...document.querySelectorAll(".search-kbd")].some((k) =>
      k.checkVisibility({ visibilityProperty: true }),
    ),
  };
};
const ENFR = ["en", "fr"];
// search.js names the shortcut after the platform, and "Ctrl K" off a Mac
// takes 13px more of the field than "⌘K": with it, "Chercher un outil…" had
// 119.8px of the 135.8 it takes in DejaVu Sans, the system-ui of a Linux
// desktop. Both words are measured wherever this runs.
const PLATFORMS = ["MacIntel", "Win32"];
const openOn = async (url, width, platform) => {
  const page = await (await contextFor(width, THEMES[0])).newPage();
  await page.addInitScript((p) => {
    Object.defineProperty(Navigator.prototype, "platform", { get: () => p });
  }, platform);
  await page.goto(url);
  await page.evaluate(() => document.fonts.ready);
  return page;
};
for (const width of WIDE) {
  await check(
    `the English and French placeholders show whole at ${width}px`,
    async () => {
      const cut = [];
      for (const base of [DEMO, SITE]) {
        for (const l of ENFR) {
          for (const platform of PLATFORMS) {
            const p = await openOn(`${base}${l}/`, width, platform);
            const f = await p.evaluate(searchField);
            const hint = await p.locator(".search-kbd").textContent();
            await p.close();
            if (!f.whole)
              cut.push(
                `"${f.placeholder}" beside "${hint}" needs ${f.text.toFixed(1)}px of ${f.room.toFixed(1)} on ${base}${l}/`,
              );
          }
        }
      }
      if (cut.length) throw new Error(cut.join("; "));
    },
  );
  // 116.6px is what the field left the placeholder before it grew taller; no
  // language may lose a letter it showed then.
  await check(
    `no language's search field has less room than before at ${width}px`,
    async () => {
      const short = [];
      for (const l of ["en", "de", "es", "pt", "sv", "fr", "it", "nl", "ar"]) {
        const p = await open(`${LOCALES}${l}/`, width);
        const f = await p.evaluate(searchField);
        await p.close();
        if (f.room < 116.6) short.push(`${l} ${f.room.toFixed(1)}px`);
      }
      if (short.length) throw new Error(short.join(", "));
    },
  );
}
// On a phone the Cmd-K hint has no keyboard to answer to, and its 51px are
// what "Chercher un outil…" was missing at 360 on the demo: 109px of room for
// 123. A placeholder still longer than the field ends on an ellipsis rather
// than in the middle of a letter.
for (const width of PHONES) {
  await check(
    `a phone drops the Cmd-K hint and shows the placeholder whole or ellipsed at ${width}px`,
    async () => {
      const bad = [];
      for (const l of ENFR) {
        const p = await open(`${DEMO}${l}/`, width);
        const f = await p.evaluate(searchField);
        await p.close();
        if (f.hint) bad.push(`the Cmd-K hint is painted on ${l}/`);
        if (f.ellipsis !== "ellipsis")
          bad.push(`text-overflow is ${f.ellipsis} on ${l}/`);
        if (width === 360 && !f.whole)
          bad.push(
            `"${f.placeholder}" needs ${f.text.toFixed(1)}px of ${f.room.toFixed(1)}`,
          );
      }
      if (bad.length) throw new Error(bad.join("; "));
    },
  );
}

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
  // The open fold takes padding on a phone screen to make room for its row,
  // and paper has no row: opened or not, the note prints in the same place.
  await check(
    `printed at ${width}px an opened note sits where a folded one does`,
    async () => {
      const p = await open(`${DEMO}fr/`, width);
      const at = () =>
        p.$eval(".about-fold > :not(summary)", (b) => {
          const r = b.getBoundingClientRect();
          return { x: r.left, y: r.top, w: r.width };
        });
      await p.emulateMedia({ media: "print" });
      const folded = await at();
      await p.emulateMedia({ media: "screen" });
      await p.click(".about-sum");
      await p.emulateMedia({ media: "print" });
      const opened = await at();
      await p.close();
      const off = ["x", "y", "w"].filter(
        (k) => Math.abs(opened[k] - folded[k]) > 0.5,
      );
      if (off.length)
        throw new Error(
          off
            .map(
              (k) =>
                `${k} ${opened[k].toFixed(1)} opened, ${folded[k].toFixed(1)} folded`,
            )
            .join("; "),
        );
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

// The detail glyph's band stops short of the description's words: a tap on
// the end of its first line opened the detail page instead of the service,
// 33.1x7.1px of "Ancien forum" at 360. Measured against the line boxes of the
// text itself, not the paragraph, which runs under the glyph by design.
const descUnderBand = () => {
  const out = [];
  for (const card of document.querySelectorAll(".card")) {
    const more = card.querySelector(".card-more"),
      desc = card.querySelector(".card-desc");
    if (!more || !desc) continue;
    const r = more.getBoundingClientRect(),
      s = getComputedStyle(more),
      b = getComputedStyle(more, "::before");
    const band = {
      l: r.left + parseFloat(s.borderLeftWidth) + parseFloat(b.left),
      t: r.top + parseFloat(s.borderTopWidth) + parseFloat(b.top),
      r: r.right - parseFloat(s.borderRightWidth) - parseFloat(b.right),
      b: r.bottom - parseFloat(s.borderBottomWidth) - parseFloat(b.bottom),
    };
    const range = document.createRange();
    range.selectNodeContents(desc);
    for (const line of range.getClientRects()) {
      const x = Math.min(band.r, line.right) - Math.max(band.l, line.left),
        y = Math.min(band.b, line.bottom) - Math.max(band.t, line.top);
      if (x > 0.5 && y > 0.5) {
        out.push(
          `${x.toFixed(1)}x${y.toFixed(1)}px of ${card.querySelector(".card-name").textContent.trim()}`,
        );
        break;
      }
    }
  }
  return out;
};
for (const width of PHONES) {
  await check(
    `the detail glyph's band leaves the description's text alone at ${width}px`,
    async () => {
      const bad = [];
      for (const url of [
        `${DEMO}en/`,
        `${DEMO}fr/`,
        ...["en", "de", "es", "pt", "sv", "fr", "it", "nl", "ar"].map(
          (l) => `${LOCALES}${l}/`,
        ),
      ]) {
        const p = await open(url, width);
        bad.push(
          ...(await p.evaluate(descUnderBand)).map((s) => `${s} on ${url}`),
        );
        await p.close();
      }
      if (bad.length) throw new Error(bad.slice(0, 6).join("; "));
    },
  );
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

// ---- the detail head and the hosted page on a phone ----

// Open sits under the name, at the start of the line the tile starts, and the
// pill and the state follow it on that one line, the way a store puts its Get
// button under an app's name. Lifted onto the back link's row, Open sat a
// whole row away from the name it acts on.
const detailHead = () => {
  const r = (s) => document.querySelector(s).getBoundingClientRect();
  const tile = r(".detail-top .tile"),
    h1 = r(".detail-top h1"),
    btn = r(".detail-top .btn"),
    pill = r(".detail-top .status-pill"),
    state = r(".detail-top .detail-state");
  const mid = (x) => x.top + x.height / 2;
  return {
    below: btn.top >= h1.bottom - 1,
    start: btn.left - tile.left,
    order: pill.left >= btn.right && state.left >= pill.right,
    spread:
      Math.max(mid(btn), mid(pill), mid(state)) -
      Math.min(mid(btn), mid(pill), mid(state)),
  };
};
// The state row narrowed until it wraps: whatever starts a line draws no
// divider before it, since a stroke there separates the item from nothing.
const wrappedRow = () => {
  const row = document.querySelector(".detail-top .detail-state-row");
  // The pill's slot is display: contents, so the pill itself is the flex item.
  const items = [...row.querySelectorAll(".status-pill, .detail-state")];
  row.style.maxWidth = `${items[0].getBoundingClientRect().width + 4}px`;
  const start = Math.min(...items.map((e) => e.getBoundingClientRect().left));
  const tops = new Set(
    items.map((e) => Math.round(e.getBoundingClientRect().top)),
  );
  return {
    lines: tops.size,
    divided: items
      .filter((e) => Math.abs(e.getBoundingClientRect().left - start) < 1)
      .filter((e) => {
        // A stroke on the start side only is a divider; the pill's outline
        // goes all the way round.
        const c = getComputedStyle(e);
        return (
          parseFloat(c.borderInlineStartWidth) >
          parseFloat(c.borderInlineEndWidth)
        );
      })
      .map((e) => e.className),
  };
};
for (const width of PHONES) {
  for (const url of [`${LOCALES}de/beta/`, `${DEMO}fr/whoami/`]) {
    await check(
      `Open leads the line under the name, then the pill and the state, at ${width}px on ${url}`,
      async () => {
        const p = await open(url, width);
        const m = await p.evaluate(detailHead);
        await p.close();
        const bad = [];
        if (!m.below) bad.push("Open is not below the name");
        if (Math.abs(m.start) > 1)
          bad.push(`Open starts ${m.start.toFixed(1)}px from the tile's edge`);
        if (!m.order) bad.push("the line does not read Open, pill, state");
        if (m.spread > 6)
          bad.push(`their centres are ${m.spread.toFixed(1)}px apart`);
        if (bad.length) throw new Error(bad.join("; "));
      },
    );
    await check(
      `a wrapped state line draws no divider at a line start at ${width}px on ${url}`,
      async () => {
        const p = await open(url, width);
        const m = await p.evaluate(wrappedRow);
        await p.close();
        if (m.lines < 2) throw new Error("the row did not wrap");
        if (m.divided.length)
          throw new Error(`a divider opens a line: ${m.divided.join(", ")}`);
      },
    );
  }
}

// A hosted page on a phone: the title stands alone, with no rule running off
// it into the margin, and the way back is a thumb's 44px high.
for (const width of PHONES) {
  await check(
    `a hosted page paints no rule after its title and a 44px back link at ${width}px`,
    async () => {
      const p = await open(`${SITE}en/hosting/`, width);
      const m = await p.evaluate(() => ({
        rule: document.querySelector(".page-head .way-rule").getClientRects()
          .length,
        back: document.querySelector(".back").getBoundingClientRect().height,
      }));
      await p.close();
      const bad = [];
      if (m.rule) bad.push("a rule is painted after the title");
      if (m.back < 44) bad.push(`the back link is ${m.back.toFixed(1)}px high`);
      if (bad.length) throw new Error(bad.join("; "));
    },
  );
}

// ---- screenshots ----

// The locales fixture's pad page carries four: wide.png (2000x700), half.png
// (800x400), tall.png (600x1200) and the demo's logo, whose size cairn does
// not read. The demo's whoami page carries its own 2000x700 one.
const SHOTS = [`${LOCALES}en/pad/`, `${DEMO}fr/whoami/`];
const shots = () =>
  [...document.querySelectorAll(".shot")].map((f) => {
    const img = f.querySelector("img"),
      strip = f.querySelector(".shot-scroll"),
      link = f.querySelector(".shot-full"),
      cap = f.querySelector("figcaption");
    const r = img.getBoundingClientRect(),
      box = (strip ?? img).getBoundingClientRect(),
      l = link?.getBoundingClientRect();
    return {
      name: img.getAttribute("src").split("/").pop(),
      w: r.width,
      h: r.height,
      ratio: +img.getAttribute("height") / +img.getAttribute("width") || 0,
      col: f.getBoundingClientRect().width,
      strip: !!strip,
      scrolls: strip ? strip.scrollWidth > strip.clientWidth + 1 : false,
      more: strip?.getAttribute("data-more") ?? null,
      left: strip?.scrollLeft ?? 0,
      // where the picture's left edge sits against the strip's: scrollLeft
      // alone reads 0 at the start of a right-to-left scroller too
      edge: r.left - box.left,
      mask: strip ? getComputedStyle(strip).maskImage : "none",
      link: l ? l.height : 0,
      under: l ? l.top >= box.bottom - 1 : false,
      capAbove: cap ? cap.getBoundingClientRect().bottom <= l.top + 1 : true,
    };
  });
const byName = (all, name) => all.find((s) => s.name === name);

// Shrunk to a phone column the demo's screenshot was 0.16 of its size and
// unreadable. It keeps 17rem of height and scrolls sideways instead; one that
// fits the column at that height fills it and does not scroll.
for (const width of PHONES) {
  for (const url of SHOTS) {
    await check(
      `a wide screenshot scrolls at a readable height at ${width}px on ${url}`,
      async () => {
        const p = await open(url, width);
        const all = await p.evaluate(shots);
        await p.close();
        const bad = [];
        const wide = all.find((s) => s.ratio && s.ratio < 0.4);
        if (!wide) throw new Error("no 2000x700 screenshot on the page");
        if (Math.abs(wide.h - 272) > 1)
          bad.push(`it is ${wide.h.toFixed(1)}px high, want 272`);
        if (wide.w / 2000 < 0.38)
          bad.push(`it shows at ${(wide.w / 2000).toFixed(3)} of its size`);
        if (!wide.scrolls) bad.push("its strip does not scroll");
        if (wide.more !== "right")
          bad.push(`its strip fades on ${wide.more}, want right`);
        if (url.startsWith(LOCALES)) {
          const half = byName(all, "half.png");
          if (!half?.scrolls) bad.push("half.png does not scroll");
          for (const n of ["tall.png", "logo.png"]) {
            const s = byName(all, n);
            if (!s) bad.push(`${n} is missing`);
            else if (Math.abs(s.w - s.col) > 1 || s.scrolls || s.more)
              bad.push(
                `${n} is ${s.w.toFixed(1)}px in a ${s.col.toFixed(1)}px column, scrolls ${s.scrolls}, fades ${s.more}`,
              );
          }
        }
        if (bad.length) throw new Error(bad.join("; "));
      },
    );
  }
}

for (const width of WIDE) {
  await check(
    `no screenshot scrolls and each fills the column at ${width}px`,
    async () => {
      const bad = [];
      for (const url of SHOTS) {
        const p = await open(url, width);
        for (const s of await p.evaluate(shots)) {
          if (s.scrolls || s.more || Math.abs(s.w - s.col) > 1)
            bad.push(
              `${s.name} on ${url}: ${s.w.toFixed(1)}px in ${s.col.toFixed(1)}px, scrolls ${s.scrolls}, fades ${s.more}`,
            );
        }
        await p.close();
      }
      if (bad.length) throw new Error(bad.join("; "));
    },
  );
}

// The link to the whole image is the one way to read it on a phone: under
// every screenshot, a thumb's 44px, under the caption where there is one.
for (const width of WIDTHS) {
  await check(
    `a 44px link to the whole image sits under every screenshot at ${width}px`,
    async () => {
      const bad = [];
      for (const url of SHOTS) {
        const p = await open(url, width);
        for (const s of await p.evaluate(shots)) {
          if (s.link < 44) bad.push(`${s.name}: link ${s.link.toFixed(1)}px`);
          if (!s.under) bad.push(`${s.name}: the link is not under the image`);
          if (!s.capAbove)
            bad.push(`${s.name}: the caption is not above the link`);
        }
        await p.close();
      }
      if (bad.length) throw new Error(bad.join("; "));
    },
  );
}

// A screenshot is a picture of a page, not text: right to left, the strip
// still opens on the image's left edge and fades on its right.
await check(
  "a right-to-left page opens the strip on the image's left edge",
  async () => {
    const p = await open(`${LOCALES}ar/pad/`, 390);
    const wide = byName(await p.evaluate(shots), "wide.png");
    await p.close();
    if (
      Math.abs(wide.edge) > 1 ||
      wide.left !== 0 ||
      wide.more !== "right" ||
      wide.mask === "none"
    )
      throw new Error(
        `the image starts ${wide.edge.toFixed(1)}px from the strip's left, scrollLeft ${wide.left}, fades on ${wide.more}, mask ${wide.mask}`,
      );
  },
);

// In the dark theme a white screenshot was the brightest thing on the screen:
// it steps down until the pointer is on it. Light never dims.
for (const theme of THEMES) {
  await check(`a screenshot's brightness in ${theme.name}`, async () => {
    const p = await open(`${DEMO}fr/whoami/`, 390, theme);
    const filter = () =>
      p.$eval(".shot img", (i) => getComputedStyle(i).filter);
    const rest = await filter();
    await p.hover(".shot img");
    const over = await filter();
    await p.close();
    const want = theme.name === "light" ? "none" : "brightness(0.78)";
    if (rest !== want || over !== "none")
      throw new Error(`at rest ${rest}, want ${want}; hovered ${over}`);
  });
}

// Paper does not scroll: printed at a phone's width, every screenshot is whole.
await check("a printed screenshot is whole at 390px", async () => {
  const p = await open(`${LOCALES}en/pad/`, 390);
  await p.emulateMedia({ media: "print" });
  const all = await p.evaluate(shots);
  await p.close();
  const bad = all
    .filter(
      (s) =>
        s.scrolls ||
        s.mask !== "none" ||
        s.w > s.col + 1 ||
        (s.ratio && Math.abs(s.h / s.w - s.ratio) > 0.02),
    )
    .map(
      (s) =>
        `${s.name} ${s.w.toFixed(1)}x${s.h.toFixed(1)} in ${s.col.toFixed(1)}px, scrolls ${s.scrolls}, mask ${s.mask}`,
    );
  if (bad.length) throw new Error(bad.join("; "));
});
// Paper is white whatever the screen's theme, and a link prints as words no
// one can follow: a printed screenshot is never dimmed and drops the link to
// the whole image.
for (const theme of THEMES) {
  await check(
    `a printed screenshot is undimmed and has no link to follow in ${theme.name}`,
    async () => {
      const p = await open(`${DEMO}fr/whoami/`, 390, theme);
      await p.emulateMedia({ media: "print" });
      const all = await p.$$eval(".shot", (fs) =>
        fs.map((f) => ({
          name: f.querySelector("img").getAttribute("src").split("/").pop(),
          filter: getComputedStyle(f.querySelector("img")).filter,
          link: !!f.querySelector(".shot-full")?.checkVisibility(),
        })),
      );
      await p.close();
      const bad = [];
      if (!all.length) bad.push("no screenshot on the page");
      for (const s of all) {
        if (s.filter !== "none") bad.push(`${s.name} prints at ${s.filter}`);
        if (s.link) bad.push(`${s.name} prints its full-screenshot link`);
      }
      if (bad.length) throw new Error(bad.join("; "));
    },
  );
}

// ---- every link and control is a 44px target ----

// Where a pointer or a thumb lands for a control: its border box, or the
// absolute ::before drawn around it when that covers more. The pseudo's
// offsets count from inside the control's border: a positioned box lays out
// its absolute children from its padding box. Runs in the page.
const hitRect = (el) => {
  const r = el.getBoundingClientRect();
  let box = { x0: r.left, y0: r.top, x1: r.right, y1: r.bottom };
  const s = getComputedStyle(el),
    b = getComputedStyle(el, "::before");
  if (
    s.position !== "static" &&
    b.content !== "none" &&
    b.display !== "none" &&
    b.position === "absolute"
  ) {
    const px = parseFloat;
    const p = {
      x0: r.left + px(s.borderLeftWidth) + px(b.left),
      y0: r.top + px(s.borderTopWidth) + px(b.top),
      x1: r.right - px(s.borderRightWidth) - px(b.right),
      y1: r.bottom - px(s.borderBottomWidth) - px(b.bottom),
    };
    const area = (q) => (q.x1 - q.x0) * (q.y1 - q.y0);
    if (Object.values(p).every(Number.isFinite) && area(p) > area(box)) box = p;
  }
  return { ...box, w: box.x1 - box.x0, h: box.y1 - box.y0 };
};

// Every painted link and control inside the scope, each with its hit area in
// page coordinates, whether it is exempt, whether the four midpoints of its
// edges (2px in, with the control scrolled to the middle of the screen) land
// on it, and which of its neighbours its hit area overlaps by more than .5px.
// Exempt are a link inside a sentence of the operator's prose, which WCAG
// 2.5.8 lets keep its line, and the card name, whose link is the whole card;
// they and a control nested in another sit out the overlap too. A hosted
// page's back link sits in a paragraph of its own and is no prose. The skip link
// is off screen until focused, so it is measured focused, at the end. Runs in
// the page; the scope is a selector, the whole document by default.
const pageTargets = (hitRect, scope) => {
  const SEL =
    'a, button, input, select, summary, [role=button], [tabindex]:not([tabindex="-1"])';
  const PROSE =
    ".about p, .detail > p, .detail > [lang] > p, .page p, .about li, .detail li, .page li";
  const root = document.querySelector(scope ?? "body");
  // A classless link is named by the nearest ancestor that has a class.
  const name = (el) => {
    const own = [...el.classList].map((c) => `.${c}`).join("");
    const at = own
      ? ""
      : `.${el.parentElement.closest("[class]")?.classList[0]} `;
    const words = (el.getAttribute("aria-label") ?? el.textContent ?? "")
      .trim()
      .replace(/\s+/g, " ")
      .slice(0, 28);
    return `${at}${el.tagName.toLowerCase()}${own} "${words}"`;
  };
  const all = [
    ...(root.matches(SEL) ? [root] : []),
    ...root.querySelectorAll(SEL),
  ].filter(
    (el) =>
      !el.matches(".skip") &&
      !el.closest(".visually-hidden, dialog:not([open]), [inert]") &&
      el.checkVisibility({ visibilityProperty: true }) &&
      el.getClientRects().length > 0,
  );
  scrollTo(0, 0);
  const list = all.map((el) => {
    const r = hitRect(el);
    return {
      el,
      name: name(el),
      exempt:
        el.matches(".card-name") ||
        (el.matches("a:not(.back)") && !!el.closest(PROSE)),
      fixed: getComputedStyle(el).position === "fixed",
      x0: r.x0 + scrollX,
      y0: r.y0 + scrollY,
      x1: r.x1 + scrollX,
      y1: r.y1 + scrollY,
      w: r.w,
      h: r.h,
    };
  });
  for (const t of list) {
    t.overlaps = [];
    if (t.exempt || t.fixed) continue;
    for (const u of list) {
      if (u === t || u.exempt || u.fixed) continue;
      if (t.el.contains(u.el) || u.el.contains(t.el)) continue;
      const by = Math.min(
        Math.min(t.x1, u.x1) - Math.max(t.x0, u.x0),
        Math.min(t.y1, u.y1) - Math.max(t.y0, u.y0),
      );
      if (by > 0.5) t.overlaps.push(`${u.name} by ${by.toFixed(1)}px`);
    }
  }
  const edges = (el) => {
    el.scrollIntoView({
      block: "center",
      inline: "center",
      behavior: "instant",
    });
    const r = hitRect(el),
      mx = (r.x0 + r.x1) / 2,
      my = (r.y0 + r.y1) / 2;
    return [
      ["top", mx, r.y0 + 2],
      ["bottom", mx, r.y1 - 2],
      ["left", r.x0 + 2, my],
      ["right", r.x1 - 2, my],
    ]
      .filter(([, x, y]) => {
        const hit = document.elementFromPoint(x, y);
        return !hit || !(hit === el || el.contains(hit));
      })
      .map(([side]) => side);
  };
  for (const t of list) t.missed = t.exempt ? [] : edges(t.el);
  let skip = null;
  const s = scope ? null : document.querySelector(".skip");
  if (s) {
    scrollTo(0, 0);
    s.focus();
    const r = hitRect(s);
    skip = { name: name(s), w: r.w, h: r.h, missed: edges(s) };
    s.blur();
  }
  return {
    skip,
    list: list.map(({ el, ...t }) => t),
  };
};

// The list a page answers with, measured as it stands.
const targets = (page, scope = null) =>
  page.evaluate(`(${pageTargets})(${hitRect}, ${JSON.stringify(scope)})`);

// What is wrong with one page's targets, worded for the failure.
const targetFaults = ({ skip, list }) => {
  const bad = [];
  for (const t of [...list, ...(skip ? [skip] : [])]) {
    if (t.exempt) continue;
    if (t.w < 43.5 || t.h < 43.5)
      bad.push(`${t.name} is ${t.w.toFixed(1)}x${t.h.toFixed(1)}`);
    if (t.missed.length)
      bad.push(`${t.name} does not answer at its ${t.missed.join(", ")} edge`);
    for (const o of t.overlaps ?? []) bad.push(`${t.name} overlaps ${o}`);
  }
  if (!list.some((t) => !t.exempt)) bad.push("no target found");
  return bad;
};

// Every page shape of every site the suite serves: home, a detail page, a
// hosted page and the page for an address that leads nowhere.
const TARGET_PAGES = [
  ...["", "whoami/", "hosting/", "nowhere/"].map((p) => `${DEMO}fr/${p}`),
  ...["", "nowhere/"].map((p) => `${LONG}fr/${p}`),
  ...["de", "ar"].flatMap((l) =>
    ["", "pad/", "hosting/", "nowhere/"].map((p) => `${LOCALES}${l}/${p}`),
  ),
  ...["", "pdf/", "photos/", "hosting/", "nowhere/"].map(
    (p) => `${SITE}en/${p}`,
  ),
  ...["", "nowhere/"].map((p) => `${MANY}en/${p}`),
];
// What only shows once opened: the leaving dialog, the phone's menu and the
// language menu of a site with more than three. Each is measured on its own,
// since the page behind a modal is inert and an open menu is drawn over it.
const OPENED = [
  {
    what: "the leaving dialog",
    url: `${LEAVE}en/`,
    scope: "#leave",
    open: (p) =>
      p.evaluate(() => document.querySelector("a[data-leave]").click()),
  },
  {
    what: "the open language menu",
    url: `${LOCALES}de/`,
    scope: ".langs-menu nav",
    open: (p) => p.click(".langs-menu summary"),
  },
  {
    what: "the open phone menu",
    url: `${MANY}en/`,
    scope: ".nav-links",
    phone: true,
    open: (p) => p.click(".nav-burger"),
  },
  // A stamped build links its version to the release, where this unstamped
  // one writes a span; the markup is the Go tests' to hold, so the span is
  // swapped for the link they pin, beside the credit it sits next to.
  {
    what: "the version link",
    url: `${DEMO}fr/`,
    scope: ".powered",
    open: (p) =>
      p.evaluate(() => {
        const span = document.querySelector("span.foot-version"),
          a = document.createElement("a");
        a.className = "foot-version";
        a.href = "https://github.com/MorganKryze/cairn/releases/tag/v1.0.0";
        a.textContent = "1.0.0";
        span.replaceWith(a);
      }),
  },
  // nav.js shows it a screen down the page, and at the foot of the demo's
  // home it is.
  {
    what: "the back-to-top button",
    url: `${DEMO}fr/`,
    scope: ".totop",
    phone: true,
    open: async (p) => {
      await p.evaluate(() => scrollTo(0, document.body.scrollHeight));
      await p.waitForSelector(".totop.show");
    },
  },
];
for (const theme of THEMES) {
  for (const width of WIDTHS) {
    await check(
      `every link and control is a 44px target at ${width}px in ${theme.name}`,
      async () => {
        const bad = [];
        // The toggle's dark is the demo's alone: the other dark has every
        // page already, and the geometry under it is the same rules.
        const urls = theme.stored ? TARGET_PAGES.slice(0, 4) : TARGET_PAGES;
        for (const url of urls) {
          const p = await open(url, width, theme);
          for (const b of targetFaults(await targets(p)))
            bad.push(`${url}: ${b}`);
          await p.close();
        }
        if (!theme.stored) {
          for (const o of OPENED) {
            if (o.phone && width > 704) continue;
            const p = await open(o.url, width, theme);
            await o.open(p);
            await p.waitForSelector(o.scope, { state: "visible" });
            for (const b of targetFaults(await targets(p, o.scope)))
              bad.push(`${o.what}: ${b}`);
            await p.close();
          }
        }
        if (bad.length) throw new Error(bad.join("\n        "));
      },
    );
  }
}

// The rail is fixed, so the page scrolling never brings its foot into view: at
// 44px a link, nine categories under a 480px window put the last two below the
// screen, out of a pointer's reach and focused where no one can see the ring.
// The rail scrolls on its own instead, and focus scrolls it.
await check(
  "every rail link can be reached and its ring seen on a short screen",
  async () => {
    const p = await open(`${MANY}en/`, 1440, THEMES[0], {
      viewport: { width: 1440, height: 480 },
    });
    const r = await p.evaluate(() => {
      const toc = document.querySelector(".toc");
      const box = toc.getBoundingClientRect();
      const links = [...toc.querySelectorAll("a")];
      const bad = [];
      if (box.bottom > innerHeight + 0.5)
        bad.push(`the rail ends at ${box.bottom.toFixed(1)}px`);
      for (const a of links) {
        a.focus();
        // A rail that scrolls clips what it holds at its own box; one that
        // does not clips nothing short of the screen.
        const v =
            getComputedStyle(toc).overflowY === "visible"
              ? { top: 0, bottom: innerHeight, left: 0, right: innerWidth }
              : toc.getBoundingClientRect(),
          l = a.getBoundingClientRect();
        // The ring is 2px wide and 2px out: 4px round the link must show.
        const ring = {
          top: l.top - 4,
          bottom: l.bottom + 4,
          left: l.left - 4,
          right: l.right + 4,
        };
        const hit = document.elementFromPoint(
          (l.left + l.right) / 2,
          (l.top + l.bottom) / 2,
        );
        const word = a.textContent.trim();
        if (!hit || !a.contains(hit))
          bad.push(`"${word}" focused does not answer at its centre`);
        if (
          ring.top < Math.max(v.top, 0) - 0.5 ||
          ring.bottom > Math.min(v.bottom, innerHeight) + 0.5 ||
          ring.left < v.left - 0.5 ||
          ring.right > v.right + 0.5
        )
          bad.push(
            `"${word}" focused rings ${ring.top.toFixed(1)} to ${ring.bottom.toFixed(1)}px down, ${ring.left.toFixed(1)} to ${ring.right.toFixed(1)}px across, in a rail shown ${Math.max(v.top, 0).toFixed(1)} to ${Math.min(v.bottom, innerHeight).toFixed(1)} by ${v.left.toFixed(1)} to ${v.right.toFixed(1)}`,
          );
      }
      return { n: links.length, bad };
    });
    await p.close();
    if (r.n < 9) throw new Error(`${r.n} rail links, so this proves nothing`);
    if (r.bad.length) throw new Error(r.bad.join("; "));
  },
);

// ---- right to left ----

// Arabic turns the page around through logical sides alone, so each of these
// sits at the other end of its row from where English has it: a left or right
// written where an inline side was meant keeps it where it was.
for (const width of WIDE) {
  await check(
    `right to left, the header rule opens with its diamond at the column's right edge at ${width}px`,
    async () => {
      const p = await open(`${LOCALES}ar/`, width);
      const m = await p.evaluate(() => {
        const w = document.querySelector("main .wrap"),
          wr = w.getBoundingClientRect();
        const menu = document.querySelector(".menu"),
          mr = menu.getBoundingClientRect();
        const d = getComputedStyle(menu, "::after"),
          r = getComputedStyle(menu, "::before");
        return {
          dir: getComputedStyle(document.documentElement).direction,
          colEnd: wr.right - parseFloat(getComputedStyle(w).paddingRight),
          diamond: {
            left: mr.left + parseFloat(d.left),
            right: mr.right - parseFloat(d.right),
          },
          rule: {
            left: mr.left + parseFloat(r.left),
            right: mr.right - parseFloat(r.right),
          },
        };
      });
      await p.close();
      if (m.dir !== "rtl") throw new Error(`the page reads ${m.dir}`);
      if (Math.abs(m.diamond.right - m.colEnd) > 1)
        throw new Error(
          `the diamond ends at ${m.diamond.right.toFixed(1)}px, the column at ${m.colEnd.toFixed(1)}px`,
        );
      if (m.rule.right > m.diamond.left + 0.5)
        throw new Error(
          `the rule runs to ${m.rule.right.toFixed(1)}px, under the diamond at ${m.diamond.left.toFixed(1)}px`,
        );
    },
  );
}

for (const width of WIDTHS) {
  await check(
    `right to left, a card foot has the flag at the card's right and the pill at its left at ${width}px`,
    async () => {
      const p = await open(`${LOCALES}ar/`, width);
      const m = await p.$$eval(".card", (cards) =>
        cards
          .filter((c) => c.querySelector(".card-foot .flag"))
          .map((c) => {
            const r = c.getBoundingClientRect(),
              s = getComputedStyle(c);
            const end =
                r.right -
                parseFloat(s.borderRightWidth) -
                parseFloat(s.paddingRight),
              start =
                r.left +
                parseFloat(s.borderLeftWidth) +
                parseFloat(s.paddingLeft);
            const flag = c
                .querySelector(".card-foot .flag")
                .getBoundingClientRect(),
              pill = c
                .querySelector(".card-foot .status-pill")
                ?.getBoundingClientRect();
            const name = c.querySelector(".card-name").textContent.trim();
            const bad = [];
            if (Math.abs(flag.right - end) > 0.5)
              bad.push(
                `the flag ends ${(end - flag.right).toFixed(1)}px short of the right`,
              );
            if (pill && pill.right > flag.left + 0.5)
              bad.push("the pill is not left of the flag");
            if (pill && Math.abs(pill.left - start) > 0.5)
              bad.push(
                `the pill starts ${(pill.left - start).toFixed(1)}px off the card's left`,
              );
            return {
              pill: !!pill,
              fault: bad.length ? `${name}: ${bad.join(", ")}` : null,
            };
          }),
      );
      await p.close();
      if (!m.some((c) => c.pill))
        throw new Error("no card carries both a flag and a pill");
      const bad = m.map((c) => c.fault).filter(Boolean);
      if (bad.length) throw new Error(bad.join("; "));
    },
  );
}

for (const width of PHONES) {
  await check(
    `right to left, the detail line reads Open, pill, state from the right at ${width}px`,
    async () => {
      const p = await open(`${LOCALES}ar/beta/`, width);
      const m = await p.evaluate(() => {
        const r = (s) => document.querySelector(s).getBoundingClientRect();
        const tile = r(".detail-top .tile"),
          btn = r(".detail-top .btn"),
          pill = r(".detail-top .status-pill"),
          state = r(".detail-top .detail-state");
        return {
          start: tile.right - btn.right,
          order: pill.right <= btn.left + 0.5 && state.right <= pill.left + 0.5,
        };
      });
      await p.close();
      const bad = [];
      if (!m.order)
        bad.push("the line does not read Open, pill, state from the right");
      if (Math.abs(m.start) > 1)
        bad.push(
          `Open ends ${m.start.toFixed(1)}px from the tile's right edge`,
        );
      if (bad.length) throw new Error(bad.join("; "));
    },
  );
}

// ---- without JavaScript ----

// A visitor with scripting off gets every page whole at a phone's 360: nothing
// pushes it sideways, the welcome note's row opens and folds on its own, and
// every link goes somewhere, since nothing will fill an empty href in later.
// The leaving dialog's own link is the one leave.js fills, and without it the
// dialog never opens, so a link in a closed dialog is out of reach.
const BARE = [
  ...["", "whoami/", "hosting/", "nowhere/"].map((p) => `${DEMO}fr/${p}`),
  ...["", "nowhere/"].map((p) => `${LONG}fr/${p}`),
  ...["", "pad/", "hosting/", "nowhere/"].map((p) => `${LOCALES}de/${p}`),
];
await check(
  "without JavaScript every page at 360px fits, folds and links",
  async () => {
    const bad = [];
    let folds = 0;
    for (const url of BARE) {
      const p = await open(url, 360, THEMES[0], { javaScriptEnabled: false });
      const m = await p.evaluate(() => ({
        over:
          document.documentElement.scrollWidth -
          document.documentElement.clientWidth,
        empty: [...document.querySelectorAll("a")]
          .filter(
            (a) =>
              !a.closest("dialog:not([open])") &&
              !a.getAttribute("href")?.trim(),
          )
          .map(
            (a) =>
              `${a.className || a.parentElement.className} "${a.textContent.trim().slice(0, 20)}"`,
          ),
        row: !!document.querySelector(".about-sum")?.checkVisibility(),
      }));
      if (m.over > 0) bad.push(`${url} overflows by ${m.over}px`);
      for (const a of m.empty) bad.push(`${url}: ${a} has no href`);
      if (m.row) {
        folds++;
        const state = () =>
          p.evaluate(() => {
            const d = document.querySelector(".about-sum").parentElement;
            const text = d.querySelector(":scope > :not(summary)");
            return [d.open, !!text?.checkVisibility()];
          });
        const before = await state();
        await p.click(".about-sum");
        const opened = await state();
        await p.click(".about-sum");
        const closed = await state();
        const seen = JSON.stringify([before, opened, closed]);
        if (
          seen !==
          JSON.stringify([
            [false, false],
            [true, true],
            [false, false],
          ])
        )
          bad.push(`${url}: the note's row went (open, shown) ${seen}`);
      }
      await p.close();
    }
    if (!folds) bad.push("no page showed the note's row");
    if (bad.length) throw new Error(bad.join("; "));
  },
);

for (const ctx of contexts.values()) await ctx.close();
await browser.close();
if (failures) {
  console.log(`\n${failures} failed`);
  process.exit(1);
}
console.log("\nall layout checks passed");
