// Drives five accessibility behaviours in a real browser against a running
// cairn, because not one of them survives a look at the markup: the theme
// toggle's state is only known once theme.js has run, the mobile category
// swap turns on whether JavaScript exists at all, a contrast ratio needs the
// colours the browser actually resolved, and a scroll is a scroll.
//
// Run it with `just test-browser`, which builds cairn and serves the example
// config, scripts/fixtures/many-categories, scripts/fixtures/status and
// scripts/fixtures/themed on scratch ports first.
//
// Usage: node scripts/a11y.mjs [example-url] [many-url] [status-url] [themed-url]
import { chromium } from "playwright";

const SITE =
  process.argv[2] ?? process.env.CAIRN_URL ?? "http://127.0.0.1:8090/en/";
const MANY =
  process.argv[3] ?? process.env.CAIRN_MANY_URL ?? "http://127.0.0.1:8091/en/";
const STATUS =
  process.argv[4] ??
  process.env.CAIRN_STATUS_URL ??
  "http://127.0.0.1:8092/en/";
const THEMED =
  process.argv[5] ?? process.env.CAIRN_THEMED_URL ?? "http://127.0.0.1:8093/en/";
const PHONE = { width: 390, height: 780 };

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
// Painted, not merely marked. Everything below turns on a display rule, and a
// display rule is invisible to any test that asks the DOM what it contains:
// the element is there either way. getClientRects asks what the layout did.
const painted = (page, sel) =>
  page.$$eval(sel, (els) =>
    els
      .filter((e) => e.getClientRects().length > 0)
      .map((e) => e.textContent.trim()),
  );

const browser = await chromium.launch();

// ---- A1: the theme toggle has to say whether it is pressed (WCAG 4.1.2) ----

const desk = await browser.newContext();
const page = await desk.newPage();
await page.goto(SITE);
const toggle = page.locator("#theme-toggle");

await check("the theme toggle states whether it is pressed", async () => {
  const [pressed, dark] = await page.evaluate(() => [
    document.getElementById("theme-toggle").getAttribute("aria-pressed"),
    document.documentElement.dataset.theme
      ? document.documentElement.dataset.theme === "dark"
      : matchMedia("(prefers-color-scheme: dark)").matches,
  ]);
  if (pressed === null)
    throw new Error(
      "no aria-pressed at all: a screen reader hears a button and no state",
    );
  eq(pressed, String(dark), "aria-pressed against the theme actually in force");
});

await check("and it flips with the theme, in both directions", async () => {
  const seen = [];
  for (const _ of [0, 1]) {
    await toggle.click();
    const theme = await page.evaluate(
      () => document.documentElement.dataset.theme,
    );
    seen.push(theme);
    eq(
      await toggle.getAttribute("aria-pressed"),
      String(theme === "dark"),
      `with data-theme=${theme}, aria-pressed`,
    );
  }
  if (seen[0] === seen[1])
    throw new Error(
      `two clicks both landed on ${seen[0]}, so only one direction was tested`,
    );
});

// ---- A3: 3:1 for the boundary of a control someone can operate (1.4.11) ----

// The border and both of its neighbours, straight out of the browser: the
// inside is the control's own fill, the outside is the nearest ancestor that
// actually paints one, since the header paints nothing on a wide viewport and
// the colour behind the box is really the body's.
const boundary = (p, sel) =>
  p.$eval(sel, async (el) => {
    const chan = (s) => {
      // Plain colours serialize as rgb()/rgba(). A color-mix() comes back as
      // oklab(), whose three numbers are not sRGB channels at all, and reading
      // them as channels reports near-black for every mix without complaining.
      if (!s.startsWith("rgb"))
        throw new Error(`cannot read ${s} as sRGB channels`);
      return s
        .match(/[\d.]+/g)
        .slice(0, 3)
        .map(Number);
    };
    const lum = (s) =>
      chan(s)
        .map((v) =>
          v / 255 <= 0.03928
            ? v / 255 / 12.92
            : ((v / 255 + 0.055) / 1.055) ** 2.4,
        )
        .reduce((acc, c, i) => acc + [0.2126, 0.7152, 0.0722][i] * c, 0);
    const wcag = (a, b) => {
      const [hi, lo] = [lum(a), lum(b)].sort((x, y) => y - x);
      return (hi + 0.05) / (lo + 0.05);
    };
    const opaque = (s) => !/,\s*0\)$/.test(s) && s !== "transparent";
    let outside = getComputedStyle(document.documentElement).backgroundColor;
    for (let e = el.parentElement; e; e = e.parentElement) {
      const bg = getComputedStyle(e).backgroundColor;
      if (opaque(bg)) {
        outside = bg;
        break;
      }
    }
    // Read once, wait a frame, read again. A colour still on its way from one
    // palette value to the other differs between the two, and that is the only
    // way this measurement has ever been wrong: it reported a point between
    // --border and --ui-border and called it the answer. Reduced motion should
    // make that impossible; this is here for the day it stops working, and it
    // fails on any machine rather than only on a slow one.
    const before = getComputedStyle(el).borderTopColor;
    await new Promise((r) =>
      requestAnimationFrame(() => requestAnimationFrame(r)),
    );
    const s = getComputedStyle(el);
    if (s.borderTopColor !== before) {
      throw new Error(
        `the border is still moving: ${before} then ${s.borderTopColor}, so neither is the resting colour`,
      );
    }
    return {
      border: s.borderTopColor,
      inside: s.backgroundColor,
      vsInside: wcag(s.borderTopColor, s.backgroundColor),
      vsOutside: wcag(s.borderTopColor, outside),
    };
  });

const atLeast3 = (m, what) => {
  for (const [where, r] of [
    ["the page behind it", m.vsOutside],
    ["its own fill", m.vsInside],
  ]) {
    if (r < 3) {
      throw new Error(
        `${what} border ${m.border} against ${where}: ${r.toFixed(2)}:1, WCAG 1.4.11 asks 3:1`,
      );
    }
  }
};

// Every contrast measurement runs with reduced motion asked for, because the
// box transitions its border-color over .15s and search.js starts that fade by
// enabling the input on load. Sampling mid-fade reads a colour that is on
// neither palette: a CI runner reported rgb(149, 152, 146) at 2.55:1, which is
// neither --border nor --ui-border but a point between them. A page of its own
// is not enough, since the fade starts on every one of them, and a fast machine
// hides the race entirely. The stylesheet turns every transition off under
// prefers-reduced-motion, so asking for it measures the resting value by
// construction rather than by winning a race, and that is also the colour a
// visitor with that preference sees for the whole life of the page.
const still = await browser.newContext({ reducedMotion: "reduce" });
const home = await still.newPage();
await home.goto(SITE);

await check(
  "the live search box clears 3:1 against both of its neighbours",
  async () => {
    atLeast3(await boundary(home, "#search .search-box"), "the search box");
  },
);

// The rule is scoped rather than global on purpose: 1.4.11 exempts an inactive
// component, and the bar renders disabled and decorative on every page but the
// home page, purely for layout parity. So there is no ratio to assert for the
// sleeping one, only that it was left alone.
const detail = await still.newPage();
await detail.goto(new URL("pdf/", SITE).href);
await check(
  "a disabled search box is left on the quiet border, being exempt",
  async () => {
    const live = (await boundary(home, "#search .search-box")).border;
    const dead = (await boundary(detail, ".search .search-box")).border;
    if (dead === live) {
      throw new Error(
        `the disabled bar wears the live border ${live}: the strong colour is not scoped to an enabled input`,
      );
    }
  },
);

// The colour math in node, for the checks below. boundary keeps its own copy
// inside the page because it walks the ancestors there; here the two colours
// come out and the ratio is computed on this side, which is the half that is
// easier to read when a value moves.
const luminance = (s) => {
  if (!s.startsWith("rgb"))
    throw new Error(`cannot read ${s} as sRGB channels`);
  return s
    .match(/[\d.]+/g)
    .slice(0, 3)
    .map(Number)
    .map((v) =>
      v / 255 <= 0.03928 ? v / 255 / 12.92 : ((v / 255 + 0.055) / 1.055) ** 2.4,
    )
    .reduce((acc, c, i) => acc + [0.2126, 0.7152, 0.0722][i] * c, 0);
};
const ratio = (a, b) => {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
};

// ---- A5: the card's detail glyph, which is both a target and a graphic ----

// It replaced words at the end of the description, so it has two duties the
// words did not: it has to be big enough to hit (2.5.8 asks 24 by 24) and its
// glyph has to be visible, since the glyph is now the whole message (1.4.11
// asks 3:1 of a graphic that carries meaning).
const glyph = (p, theme) =>
  p.$eval(
    ".card-more",
    (el, theme) => {
      document.documentElement.dataset.theme = theme;
      const r = el.getBoundingClientRect();
      const s = getComputedStyle(el);
      return {
        w: r.width,
        h: r.height,
        colour: s.color,
        behind: s.backgroundColor,
      };
    },
    theme,
  );

for (const theme of ["light", "dark"]) {
  await check(
    `the detail glyph is a real target and stays visible in ${theme}`,
    async () => {
      const m = await glyph(home, theme);
      if (m.w < 24 || m.h < 24) {
        throw new Error(
          `${Math.round(m.w)} by ${Math.round(m.h)}, WCAG 2.2 asks 24 by 24 of a target`,
        );
      }
      const r = ratio(m.colour, m.behind);
      if (r < 3) {
        throw new Error(
          `${m.colour} on ${m.behind} is ${r.toFixed(2)}:1, WCAG 1.4.11 asks 3:1`,
        );
      }
    },
  );
}

// The way back out of a detail page: the same two duties as the card glyph,
// since it was the other control under 24 by 24.
await check("the back link is a real target on a detail page", async () => {
  const m = await detail.$eval(".back", (el) => {
    const r = el.getBoundingClientRect();
    const s = getComputedStyle(el);
    return {
      w: r.width,
      h: r.height,
      colour: s.color,
      behind: s.backgroundColor,
    };
  });
  if (m.w < 24 || m.h < 24) {
    throw new Error(
      `${Math.round(m.w)} by ${Math.round(m.h)}, WCAG 2.2 asks 24 by 24 of a target`,
    );
  }
  const r = ratio(m.colour, m.behind);
  if (r < 4.5) {
    throw new Error(
      `its words are ${r.toFixed(2)}:1 against the chip, and text asks 4.5:1`,
    );
  }
});

// A hosting flag that leads somewhere is a control, and the demo sets one, so
// the resting size is worth holding: folded, before anyone hovers, is what a
// thumb arrives on.
await check(
  "a linked hosting flag is a real target while still folded",
  async () => {
    const m = await home.$eval(".card-foot a.flag", (el) => {
      const r = el.getBoundingClientRect();
      return { w: r.width, h: r.height };
    });
    if (m.w < 24 || m.h < 24) {
      throw new Error(
        `${Math.round(m.w)} by ${Math.round(m.h)} at rest, WCAG 2.2 asks 24 by 24 of a target`,
      );
    }
  },
);

// A card is one big link, so the pointer is a hand everywhere on it and says
// nothing extra when it reaches a control. The fill is what tells a visitor
// they are on a button, so it is worth holding: 3:1 against the card for the
// boundary, 4.5:1 for the words the pill and the flag carry on top of it.
//
// color-mix() serializes as oklab(), which the sRGB reader above refuses, so
// the colours go through a canvas: it hands back what would be painted.
const hoverColours = (p, sel, theme) =>
  p.$eval(
    sel,
    (el, theme) => {
      document.documentElement.dataset.theme = theme;
      const toRGB = (c) => {
        const cv = document.createElement("canvas");
        cv.width = cv.height = 1;
        const x = cv.getContext("2d");
        x.fillStyle = c;
        x.fillRect(0, 0, 1, 1);
        const d = x.getImageData(0, 0, 1, 1).data;
        return `rgb(${d[0]}, ${d[1]}, ${d[2]})`;
      };
      const s = getComputedStyle(el);
      return {
        fill: toRGB(s.backgroundColor),
        ink: toRGB(s.color),
        card: toRGB(getComputedStyle(el.closest(".card")).backgroundColor),
      };
    },
    theme,
  );

for (const theme of ["light", "dark"]) {
  await check(
    `hovering a card control fills it, visibly, in ${theme}`,
    async () => {
      // The example config has no monitor, so it has no pill: the controls
      // checked are the ones this page actually carries, and none at all is a
      // failure rather than a quiet pass.
      const present = [];
      for (const sel of [
        ".card-more",
        ".card-foot a.flag",
        ".card-foot a.status-pill",
      ]) {
        if (await home.$(sel)) present.push(sel);
      }
      if (present.length === 0) {
        throw new Error("no card control on this page to hover at all");
      }
      for (const sel of present) {
        await home.hover(sel);
        await home.waitForTimeout(250);
        const m = await hoverColours(home, sel, theme);
        const fill = ratio(m.fill, m.card);
        if (fill < 3) {
          throw new Error(
            `${sel} fills to ${m.fill}, ${fill.toFixed(2)}:1 against the card, which reads as nothing`,
          );
        }
        const ink = ratio(m.ink, m.fill);
        if (ink < 4.5) {
          throw new Error(
            `${sel} puts ${m.ink} on ${m.fill}, ${ink.toFixed(2)}:1, and text asks 4.5:1`,
          );
        }
      }
      await home.mouse.move(2, 2);
    },
  );
}

// A card grid lands on fractional track widths, so every card sits at its own
// subpixel phase. A ring painted on the box edges and a glyph centred inside it
// then round to different sides, and the glyph reads off centre, differently on
// each card. Whole numbers on both sides is what stops it, so that is what is
// held here: the gap has to be an integer, or the drawing goes back to
// disagreeing with its own outline.
await check("the detail glyph and its ring round the same way", async () => {
  const gap = await home.$eval(".card-more", (el) => {
    const s = getComputedStyle(el);
    const box =
      parseFloat(s.width) -
      parseFloat(s.borderLeftWidth) -
      parseFloat(s.borderRightWidth);
    const glyph = el.querySelector("svg").getBoundingClientRect().width;
    return { box, glyph, gap: (box - glyph) / 2 };
  });
  if (!Number.isInteger(gap.gap)) {
    throw new Error(
      `a ${gap.glyph}px glyph in a ${gap.box}px box leaves ${gap.gap}px on each side, which rounds one way on one card and the other way on the next`,
    );
  }
});

// ---- A6: 3:1 for a dot that carries meaning, on both themes (1.4.11) ----

// The fixture's monitor is a port nothing listens on, so every pill on that
// page renders unknown and neither of these two states can be reached by
// waiting. Both come from monitors that report more than a bool, which a
// fixture cannot stand up. What the stylesheet keys on is the class, so the
// class is what is set: this measures the colours, which is all it claims to.
const dot = (p, cls, theme) =>
  p.$eval(
    ".status-pill",
    (el, [cls, theme]) => {
      document.documentElement.dataset.theme = theme;
      el.className = `status-pill ${cls}`;
      const d = el.querySelector(".dot");
      const s = getComputedStyle(d);
      return {
        colour: s.backgroundColor,
        behind: getComputedStyle(el).backgroundColor,
        radius: s.borderTopLeftRadius,
      };
    },
    [cls, theme],
  );

const pills = await still.newPage();
await pills.goto(STATUS);

for (const cls of ["status-degraded", "status-maintenance"]) {
  await check(
    `the ${cls.slice(7)} dot clears 3:1 against its pill on both themes`,
    async () => {
      for (const theme of ["light", "dark"]) {
        const m = await dot(pills, cls, theme);
        const r = ratio(m.colour, m.behind);
        if (r < 3) {
          throw new Error(
            `${theme}: ${m.colour} on ${m.behind} is ${r.toFixed(2)}:1, WCAG 1.4.11 asks 3:1`,
          );
        }
      }
    },
  );
}

// Blue against green is the pairing most often indistinguishable, and this dot
// is nine pixels across. The label carries the meaning without colour, which is
// what 1.4.1 asks for; the shape is the redundancy for the glance.
await check(
  "the maintenance dot is square, so colour is not the only cue",
  async () => {
    const round = await dot(pills, "status-up", "light");
    const square = await dot(pills, "status-maintenance", "light");
    if (square.radius === round.radius) {
      throw new Error(
        `the maintenance dot wears the round radius ${round.radius}: only its colour tells it apart`,
      );
    }
  },
);

// ---- A2 and A4, both of which only exist at phone width ----

const phone = await browser.newContext({ viewport: PHONE });
const m = await phone.newPage();
await m.goto(MANY);

// Measured still as well, and at phone width, which is the only place this
// control is painted at all.
const stillPhone = await browser.newContext({
  viewport: PHONE,
  reducedMotion: "reduce",
});
const sm = await stillPhone.newPage();
await sm.goto(MANY);
await check("the mobile jump-to select clears 3:1 too", async () => {
  atLeast3(await boundary(sm, ".toc-select"), "the jump-to select");
});

await check(
  "a phone with JavaScript shows the jump-to select, not the chips",
  async () => {
    eq((await painted(m, ".toc-select")).length, 1, "painted jump selects");
    eq((await painted(m, ".toc.many a")).length, 0, "painted trail chips");
  },
);

// The regression: the swap to a select is gated on JavaScript because nav.js
// is the select's entire behaviour. Ungated, a phone with scripting off got no
// category navigation at all, on the viewport where a nine-category list is
// least scrollable.
const dumb = await browser.newContext({
  viewport: PHONE,
  javaScriptEnabled: false,
});
const nojs = await dumb.newPage();
await nojs.goto(MANY);

await check(
  "a phone without JavaScript still paints category navigation",
  async () => {
    const chips = await painted(nojs, ".toc.many a");
    if (chips.length === 0) {
      const inert = (await painted(nojs, ".toc-select")).length;
      throw new Error(
        `no category link is painted, and ${inert} jump select(s) are, which nothing can drive`,
      );
    }
  },
);

// A4: a scroll dismisses the open menu, unless the keyboard is inside it.
const nav = m.locator("details.nav");
const openMenu = async () => {
  await m.evaluate(() => {
    scrollTo({ top: 0, behavior: "instant" });
    document.querySelector("details.nav").open = false;
  });
  await m.locator(".nav-burger").click();
  eq(await nav.evaluate((el) => el.open), true, "the menu opened");
  // The dropdown fades in over .16s from visibility:hidden, and focus() on a
  // hidden element is a no-op that leaves focus on the burger, which is the
  // other case entirely and would pass for the wrong reason.
  await m
    .locator(".nav-links .menu-link")
    .first()
    .waitFor({ state: "visible" });
};
// behavior:'instant' because the stylesheet sets scroll-behavior:smooth, so a
// plain scrollBy animates and scrollY has barely moved a frame later. And a
// page that did not move fires no scroll at all, which would let every check
// below pass without testing anything: hence the assertion that it did.
const scrollDown = async () => {
  const moved = await m.evaluate(async () => {
    const before = scrollY;
    scrollTo({ top: before + 400, behavior: "instant" });
    // nav.js batches its work into a requestAnimationFrame, so the assertion
    // has to wait for the frame the scroll event scheduled.
    await new Promise((r) =>
      requestAnimationFrame(() => requestAnimationFrame(r)),
    );
    return scrollY !== before;
  });
  if (!moved)
    throw new Error("the page never scrolled, so nothing was under test");
};

await check(
  "scrolling with focus inside the menu keeps it open, and keeps the focus",
  async () => {
    await openMenu();
    const link = m.locator(".nav-links .menu-link").first();
    const label = (await link.textContent()).trim();
    await link.focus();
    await scrollDown();
    eq(await nav.evaluate((el) => el.open), true, "the menu after a scroll");
    const holder = await m.evaluate(() => {
      const a = document.activeElement;
      return a && a.closest(".nav-links")
        ? a.textContent.trim()
        : `<${((a && a.tagName) || "?").toLowerCase()}>`;
    });
    eq(holder, label, "what holds focus after the scroll");
  },
);

await check(
  "and a scroll with focus on the burger itself still dismisses it",
  async () => {
    await openMenu();
    await m.locator(".nav-burger").focus();
    await scrollDown();
    eq(await nav.evaluate((el) => el.open), false, "the menu after a scroll");
  },
);

// Artwork that cannot serve both themes. The whole feature is a CSS rule
// keyed on data-theme, so the markup says nothing about it: what matters is
// which url() the browser has resolved on each element, and that pressing the
// theme button moves it. A <picture> would pass a markup test and fail this
// one, because it answers to the operating system and not to the button.
{
  const t = await browser.newPage();
  await t.goto(THEMED, { waitUntil: "networkidle" });
  const painted = () =>
    t.evaluate(() => {
      const file = (el) =>
        getComputedStyle(el).content.split("/").pop().replace(/["')]/g, "");
      const tiles = [...document.querySelectorAll(".tile img")];
      return {
        logo: file(document.querySelector(".brand img")),
        first: file(tiles[0]),
        second: file(tiles[1]),
        plain: file(tiles[2]),
        classes: tiles.slice(0, 2).map((i) => i.className),
      };
    });

  await check("themed artwork starts on the light image", async () => {
    const p = await painted();
    eq(p.logo, "normal", "the logo before any switch");
    eq(p.first, "normal", "a themed icon before any switch");
  });

  await check("pressing the theme button swaps the artwork", async () => {
    await t.locator("#theme-toggle").click();
    const p = await painted();
    eq(p.logo, "logo-pale.svg", "the logo in dark");
    eq(p.first, "mark-pale.svg", "a themed icon in dark");
  });

  // A negative control: it passes whether or not the feature exists, and is
  // here to catch a rule written loosely enough to repaint every icon.
  await check("an icon that themes nothing is left alone", async () => {
    eq((await painted()).plain, "normal", "the untouched icon in dark");
  });

  await check("cards sharing a pair share one rule", async () => {
    const p = await painted();
    // Non-empty first: two cards with no class at all would otherwise agree
    // with each other and let a missing feature pass for a working one.
    eq(p.classes[0] !== "", true, "the first card carries a class");
    eq(p.classes[1], p.classes[0], "the second card's class");
    eq(p.second, "mark-pale.svg", "the second card's icon in dark");
  });

  // The other direction, and the other negative control: a rule that applied
  // in both themes would fail here and nowhere else.
  await check("pressing it again puts the light artwork back", async () => {
    await t.locator("#theme-toggle").click();
    const p = await painted();
    eq(p.logo, "normal", "the logo after switching back");
    eq(p.first, "normal", "a themed icon after switching back");
  });

  // The favicon is the one themed surface CSS cannot reach: it ships as a
  // second <link> whose media query the button rewrites by hand.
  const media = () =>
    t.evaluate(() => document.querySelector('link[rel="icon"][media]').media);
  await check("the themed favicon follows the button too", async () => {
    await t.locator("#theme-toggle").click();
    eq(await media(), "all", "the dark favicon's media in dark");
    await t.locator("#theme-toggle").click();
    eq(await media(), "not all", "the dark favicon's media in light");
  });
  await t.close();
}

await browser.close();
console.log(failures ? `\n${failures} failed` : "\nall passed");
process.exit(failures ? 1 : 0);
