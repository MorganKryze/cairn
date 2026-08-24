// Drives the accessibility behaviours no look at the markup can settle: the
// theme toggle's state is only known once theme.js has run, the mobile
// category swap turns on whether JavaScript exists at all, a contrast ratio
// needs the colours the browser resolved, and a scroll is a scroll.
//
// Run it with `just test-browser`, which builds cairn and serves the example
// config, scripts/fixtures/many-categories, scripts/fixtures/status,
// scripts/fixtures/themed and scripts/fixtures/leave on scratch ports first.
//
// Usage: node scripts/a11y.mjs [example-url] [many-url] [status-url] [themed-url] [leave-url]
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
  process.argv[5] ??
  process.env.CAIRN_THEMED_URL ??
  "http://127.0.0.1:8093/en/";
const LEAVE =
  process.argv[6] ?? process.env.CAIRN_LEAVE_URL ?? "http://127.0.0.1:8094/en/";
const STATES =
  process.argv[7] ??
  process.env.CAIRN_STATES_URL ??
  "http://127.0.0.1:8095/en/";
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
// Painted, not merely marked. A display rule is invisible to any test that
// asks the DOM what it contains, since the element is there either way.
// getClientRects asks what the layout did.
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

// ---- A2: the welcome note's dismiss button folds like the host flag ----

// The fold: a glyph at rest, the word unfurled beside it on hover. It runs on
// `page` rather than on `home` below, which asks for reduced motion and so
// measures every transition already finished.
//
// A display swap satisfies neither half: display does not interpolate, and it
// takes the glyph away at the instant the pointer lands on it. Both states are
// held instead, faded and clipped at rest, both parts painted once open.
await check("the welcome note's dismiss button unfurls its label", async () => {
  const x = page.locator("#about-x");
  await x.waitFor({ state: "visible" });
  const read = () =>
    x.evaluate((el) => {
      const box = el.getBoundingClientRect();
      const g = el.querySelector("svg").getBoundingClientRect();
      return {
        w: box.width,
        h: box.height,
        // the glyph's own rect ignores the clip, so a negative side is a leg
        // cut off rather than a cross merely sitting off centre
        sides: [g.left - box.left, box.right - g.right],
        glyphPainted: el.querySelector("svg").getClientRects().length > 0,
        // a display:none span still reports opacity 1
        labelOpacity: parseFloat(
          getComputedStyle(el.querySelector("span")).opacity,
        ),
      };
    });

  const rest = await read();
  if (rest.labelOpacity !== 0)
    throw new Error(
      `the label sits at opacity ${rest.labelOpacity} before anyone hovers, so there is nothing to fade in`,
    );
  if (!rest.glyphPainted)
    throw new Error("no glyph at rest: the button reads as empty");
  // Folded it is a disc, not a pill: the radius is already past half of either
  // side, so any difference between width and height reads as a circle
  // squashed flat top and bottom.
  if (Math.abs(rest.w - rest.h) > 1)
    throw new Error(
      `${Math.round(rest.w)} by ${Math.round(rest.h)} folded, so the pill is not round`,
    );
  // a clip tight enough to square the box must not take a leg off the cross
  const [near, far] = rest.sides;
  if (Math.min(near, far) < 0 || Math.abs(near - far) > 1)
    throw new Error(
      `the glyph leaves ${near.toFixed(1)}px on one side and ${far.toFixed(1)}px on the other`,
    );
  if (rest.w < 24)
    throw new Error(
      `${Math.round(rest.w)}px wide folded, and WCAG 2.2 asks 24 by 24 of a target`,
    );

  await x.hover();
  await page.waitForTimeout(450);
  const open = await read();
  if (open.labelOpacity !== 1)
    throw new Error(
      `hovered, the label is still at opacity ${open.labelOpacity}`,
    );
  if (!open.glyphPainted)
    throw new Error(
      "the glyph left as the pointer arrived: the button empties itself to show the word",
    );
  if (open.w <= rest.w + 8)
    throw new Error(
      `${Math.round(rest.w)}px folded and ${Math.round(open.w)}px hovered: the label is not unfurling`,
    );
  await page.mouse.move(2, 2);
});

// ---- A3: 3:1 for the boundary of a control someone can operate (1.4.11) ----

// The border and both of its neighbours, straight out of the browser. The
// outside is the nearest ancestor that actually paints one: the header paints
// nothing on a wide viewport, so the colour behind the box is the body's.
const boundary = (p, sel) =>
  p.$eval(sel, async (el) => {
    const chan = (s) => {
      // Plain colours serialize as rgb()/rgba(). A color-mix() comes back as
      // oklab(), whose three numbers are not sRGB channels, and reading them
      // as channels reports near-black for every mix without complaining.
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
    // way this measurement has ever been wrong: a point between --border and
    // --ui-border, reported as the answer. Reduced motion should make it
    // impossible; this catches the day that stops holding, on any machine
    // rather than only on a slow one.
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

// Every contrast measurement runs with reduced motion asked for. The box
// transitions its border-color over .15s and search.js starts that fade by
// enabling the input on load, so sampling mid-fade reads a colour on neither
// palette: a CI runner reported rgb(149, 152, 146) at 2.55:1, a point between
// --border and --ui-border. A page of its own does not help, since the fade
// starts on every one, and a fast machine hides the race. Under
// prefers-reduced-motion the stylesheet turns every transition off, so this
// measures the resting value by construction, which is also the colour a
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

// 1.4.11 exempts an inactive component, and the bar renders disabled and
// decorative on every page but the home page, for layout parity. There is no
// ratio to assert for the sleeping one, only that it was left alone.
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
// inside the page because it walks the ancestors there.
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

// It replaced words at the end of the description, so it carries two duties
// those words did not: 24 by 24 to hit (2.5.8), and 3:1 for the glyph, which
// is now the whole message (1.4.11, a graphic that carries meaning).
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

// The button that opens the service, keyboard-first. Added after a stylesheet
// rewrite silently dropped .btn:focus-visible, which no check here noticed
// because every other one is about size or colour at rest.
//
// It asserts the designed ring rather than merely that focus is visible: with
// the rule gone the button still gets a ring, Chromium's own, outline-style
// "auto" at 1px in its blue, and a check that only asked for an outline passed
// with the rule deleted. The rule paints solid 2px in --fg.
await check(
  "the detail page's open button wears cairn's focus ring",
  async () => {
    const m = await detail.$eval("a.btn", (el) => {
      el.focus();
      const s = getComputedStyle(el);
      return { style: s.outlineStyle, width: parseFloat(s.outlineWidth) };
    });
    if (m.style !== "solid" || m.width < 2) {
      throw new Error(
        `the open button fell back to the browser's ring: ${m.style} ${m.width}px, want solid 2px`,
      );
    }
  },
);

// The way back out of a detail page: the same two duties as the card glyph.
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

// A hosting flag that leads somewhere is a control, and folded, before anyone
// hovers, is what a thumb arrives on.
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
// they are on a button: 3:1 against the card for the boundary, 4.5:1 for the
// words the pill and the flag carry on top of it.
//
// color-mix() serializes as oklab(), which the sRGB reader above refuses, so
// these colours go through a canvas, which hands back what would be painted.
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
      // The example config has no monitor, so it has no pill. The controls
      // checked are the ones this page carries, and none at all is a failure
      // rather than a quiet pass.
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
// subpixel phase. A ring painted on the box edges and a glyph centred inside
// it then round to different sides, and the glyph reads off centre,
// differently on each card. An integer gap on both sides is what stops it.
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

// The fixture's monitor is a port nothing listens on, so every pill renders
// unknown and neither of these two states can be reached by waiting: both come
// from monitors that report more than a bool. The stylesheet keys on the
// class, so the class is what is set, and this measures colours only.
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
// is nine pixels across. The label carries the meaning without colour (1.4.1);
// the shape is the redundancy for the glance.
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

// A phone raises the dismiss button to 44px for the thumb, and that rule is
// older than the fold. The fold caps the width from the desktop block, so
// raising only the height leaves an oval with the cross pressed against the
// clip: 30 by 44 with 3px of air on one side, on a shipped release. Held at
// 320 as well, where a rem is no smaller but the note is tightest.
for (const width of [390, 320]) {
  await check(`the dismiss button is a disc at ${width}px`, async () => {
    const p = await browser.newPage({ viewport: { width, height: 780 } });
    await p.goto(SITE);
    const x = p.locator("#about-x");
    await x.waitFor({ state: "visible" });
    const g = await x.evaluate((el) => {
      const r = el.getBoundingClientRect();
      const svg = el.querySelector("svg").getBoundingClientRect();
      return {
        w: r.width,
        h: r.height,
        near: svg.left - r.left,
        far: r.right - svg.right,
      };
    });
    await p.close();
    if (Math.abs(g.w - g.h) > 1) {
      throw new Error(
        `${Math.round(g.w)} by ${Math.round(g.h)}, so the pill is not round`,
      );
    }
    if (g.h < 44) {
      throw new Error(
        `${Math.round(g.h)}px tall, and the rule that raises it exists to give a thumb 44`,
      );
    }
    if (Math.min(g.near, g.far) < 0 || Math.abs(g.near - g.far) > 1) {
      throw new Error(
        `the cross leaves ${g.near.toFixed(1)}px on one side and ${g.far.toFixed(1)}px on the other`,
      );
    }
  });
}

// The jump-to select is painted only at phone width.
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

// The swap to a select is gated on JavaScript because nav.js is the select's
// entire behaviour. Ungated, a phone with scripting off got no category
// navigation at all, on the viewport where a nine-category list is least
// scrollable.
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
  // hidden element is a no-op that leaves focus on the burger: the other case
  // entirely, passing for the wrong reason.
  await m
    .locator(".nav-links .menu-link")
    .first()
    .waitFor({ state: "visible" });
};
// behavior:'instant' because the stylesheet sets scroll-behavior:smooth, so a
// plain scrollBy animates and scrollY has barely moved a frame later. A page
// that did not move fires no scroll at all and every check below would pass
// without testing anything, so the move itself is asserted.
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

// Artwork that cannot serve both themes. The feature is a CSS rule keyed on
// data-theme, so the markup says nothing about it: what counts is which url()
// the browser resolved on each element, and that the theme button moves it. A
// <picture> would pass a markup test and fail this one, since it answers to
// the operating system and not to the button.
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

  // A negative control: it passes whether or not the feature exists, and
  // catches a rule written loosely enough to repaint every icon.
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

  // The detail page draws the tile from its own template, so it is a second
  // place the class has to reach.
  await check("a detail page swaps its tile too", async () => {
    await t.goto(new URL("themed-one/", THEMED).href, {
      waitUntil: "networkidle",
    });
    const file = () =>
      t.evaluate(() =>
        getComputedStyle(document.querySelector(".tile img"))
          .content.split("/")
          .pop()
          .replace(/["')]/g, ""),
      );
    eq(await file(), "normal", "the detail tile in light");
    await t.locator("#theme-toggle").click();
    eq(await file(), "mark-pale.svg", "the detail tile in dark");
    // The choice lives in localStorage and outlives the navigation, so
    // leaving it on dark would decide what the next check starts from.
    await t.locator("#theme-toggle").click();
    await t.goto(THEMED, { waitUntil: "networkidle" });
  });

  // The favicon is the one themed surface CSS cannot reach: it ships as a
  // second <link> whose media query the button rewrites by hand.
  const media = () =>
    t.evaluate(() => document.querySelector('link[rel="icon"][media]').media);
  await check("the themed favicon follows the button too", async () => {
    // From a stated starting point rather than whatever the last check left.
    await t.evaluate(() => localStorage.removeItem("theme"));
    await t.reload({ waitUntil: "networkidle" });
    eq(
      await media(),
      "(prefers-color-scheme: dark)",
      "the media before any choice",
    );
    await t.locator("#theme-toggle").click();
    eq(await media(), "all", "the dark favicon's media in dark");
    await t.locator("#theme-toggle").click();
    eq(await media(), "not all", "the dark favicon's media in light");
  });
  await t.close();
}

// ---- the leaving dialog ----
//
// Go tests hold which links are guarded. Only a browser can say whether the
// dialog a visitor meets behaves like one: modal, escapable, and returning
// them where they were. All of that comes from <dialog>.showModal(), so what
// is checked here is that the script calls it rather than rolling its own div.
{
  const l = await browser.newPage();
  await l.goto(LEAVE, { waitUntil: "networkidle" });
  console.log("\nleaving dialog");

  const guarded = l.locator('a[data-leave][href^="https://mail"]').first();
  const dialog = l.locator("#leave");

  await check("an unguarded link carries no data-leave", async () => {
    eq(
      await l.locator('a[data-leave][href^="https://pad"]').count(),
      0,
      "the self-hosted card",
    );
    eq(
      await l.locator('a[data-leave][href^="https://quiet"]').count(),
      0,
      "the unflagged card",
    );
  });

  await check(
    "clicking a guarded link opens the dialog instead of leaving",
    async () => {
      const before = l.url();
      await guarded.click();
      eq(await dialog.evaluate((d) => d.open), true, "dialog.open");
      eq(l.url(), before, "the page navigated anyway");
    },
  );

  await check(
    "it is modal, so the page behind it cannot be reached",
    async () => {
      // A real showModal() puts everything else in the inert subtree, which is
      // what makes the focus trap and the backdrop work. A div pretending to
      // be a dialog passes every markup check and fails this one.
      eq(
        await l.evaluate(
          () =>
            document.querySelector(".card-name").matches(":not(dialog *)") &&
            document.querySelector("#leave").matches(":modal"),
        ),
        true,
        "#leave is :modal",
      );
    },
  );

  await check(
    "it names the origin apart from the rest of the address",
    async () => {
      eq(
        await l.locator(".leave-host").textContent(),
        "https://mail.example.org",
        "the origin",
      );
      eq(
        await l.locator(".leave-rest").textContent(),
        "/inbox?tab=1",
        "the path and query",
      );
    },
  );

  await check("continue is a real link to the destination", async () => {
    const go = l.locator(".leave-go");
    eq(
      await go.getAttribute("href"),
      "https://mail.example.org/inbox?tab=1",
      "the continue href",
    );
    eq(
      await go.getAttribute("target"),
      "_blank",
      "new_tab is on in this fixture",
    );
    eq(await go.getAttribute("rel"), "noopener noreferrer", "the rel");
  });

  // Chromium lets Tab off the end of a modal dialog, into the browser's own
  // toolbar, so one press in four moves nothing on screen. A bare <dialog>
  // does the same. WAI-ARIA's dialog pattern asks for the wrap.
  await check(
    "Tab wraps inside the dialog rather than leaving it",
    async () => {
      eq(
        await dialog.evaluate((d) => d.open),
        true,
        "dialog.open before tabbing",
      );
      const focused = () =>
        l.evaluate(() => {
          const a = document.activeElement;
          if (!a) return "(none)";
          return a.closest("#leave") ? a.className : `outside: ${a.tagName}`;
        });

      await l.locator(".leave-go").focus();
      await l.keyboard.press("Tab");
      eq(
        await focused(),
        "leave-copy",
        "where Tab off the last control landed",
      );

      await l.locator(".leave-copy").focus();
      await l.keyboard.press("Shift+Tab");
      eq(
        await focused(),
        "btn leave-go",
        "where Shift+Tab off the first control landed",
      );
    },
  );

  await check("Escape closes it and the focus comes back", async () => {
    // Open is asserted first: a dialog that never opened is already closed and
    // the focus never left the link that was clicked, so with no script at all
    // this check passed on its own.
    eq(await dialog.evaluate((d) => d.open), true, "dialog.open before Escape");
    await l.keyboard.press("Escape");
    eq(await dialog.evaluate((d) => d.open), false, "dialog.open after Escape");
    // <dialog> returns focus to whatever opened it, which is the guarded link.
    eq(
      await l.evaluate(() => document.activeElement?.getAttribute("href")),
      "https://mail.example.org/inbox?tab=1",
      "where the focus landed",
    );
  });

  await check("the stay button closes it too", async () => {
    await guarded.click();
    await l.locator(".leave-stay").click();
    eq(await dialog.evaluate((d) => d.open), false, "dialog.open after stay");
  });

  await check("the detail page carries the same guard", async () => {
    await l.goto(new URL("mail/", LEAVE).href, { waitUntil: "networkidle" });
    await l.locator("a.btn[data-leave]").click();
    eq(
      await l.locator("#leave").evaluate((d) => d.open),
      true,
      "the detail dialog",
    );
  });

  // ---- the same dialog on a phone ----
  //
  // The actions are a wrapping row with the primary pushed to the far end by
  // an auto margin, which reads well on a wide screen and falls apart the
  // moment it wraps. At 390px Continue landed alone on a second line, shoved
  // right, with dead space beside it; at 320px all three stacked at three
  // different widths, the last one still offset.
  {
    const ph = await browser.newPage({ viewport: { width: 390, height: 780 } });
    await ph.goto(LEAVE, { waitUntil: "networkidle" });
    await ph.locator("a[data-leave]").first().click();

    const boxes = () =>
      ph.evaluate(() => {
        const d = document.getElementById("leave");
        const one = (s) => {
          const b = d.querySelector(s).getBoundingClientRect();
          return {
            x: +b.x.toFixed(1),
            y: +b.y.toFixed(1),
            w: +b.width.toFixed(1),
            h: +b.height.toFixed(1),
          };
        };
        const acts = d.querySelector(".leave-acts").getBoundingClientRect();
        return {
          row: +acts.width.toFixed(1),
          copy: one(".leave-copy"),
          stay: one(".leave-stay"),
          go: one(".leave-go"),
        };
      });

    await check(
      "on a phone the actions stack, each filling the width",
      async () => {
        const b = await boxes();
        for (const k of ["copy", "stay", "go"]) {
          if (Math.abs(b[k].w - b.row) > 1) {
            throw new Error(
              `${k} is ${b[k].w}px inside a ${b.row}px row, so it does not fill it`,
            );
          }
        }
        // Three rows, not one and a half: every top differs from the others.
        const tops = new Set([b.copy.y, b.stay.y, b.go.y]);
        if (tops.size !== 3)
          throw new Error(
            `the three actions share ${3 - tops.size + 1} rows, want one each`,
          );
      },
    );

    await check(
      "and they are painted in the order they are read in",
      async () => {
        // A column-reverse would put the primary on top while a keyboard still
        // tabs copy, stay, continue: 2.4.3 broken for the sake of a layout.
        const b = await boxes();
        if (!(b.copy.y < b.stay.y && b.stay.y < b.go.y)) {
          throw new Error(
            `painted order ${JSON.stringify([b.copy.y, b.stay.y, b.go.y])} does not follow the markup`,
          );
        }
        if (!(b.copy.x === b.stay.x && b.stay.x === b.go.x)) {
          throw new Error(
            "the three do not share a left edge, so one is still offset",
          );
        }
      },
    );

    await check("each action is a 44px target on a phone", async () => {
      const b = await boxes();
      for (const k of ["copy", "stay", "go"]) {
        if (b[k].h < 44) throw new Error(`${k} is ${b[k].h}px tall, want 44`);
      }
    });

    await ph.close();
  }

  // Wide screens keep the row: the utility on the left, the primary at the far
  // end, which is what the phone layout departs from.
  await check("on a wide screen the actions stay on one row", async () => {
    await l.setViewportSize({ width: 1100, height: 900 });
    await l.goto(LEAVE, { waitUntil: "networkidle" });
    await l.locator("a[data-leave]").first().click();
    const b = await l.evaluate(() => {
      const d = document.getElementById("leave");
      const one = (s) => d.querySelector(s).getBoundingClientRect();
      const go = one(".leave-go");
      return {
        // The row's own height, against the tallest thing in it. Comparing
        // the tops instead looks right and is not: align-items centres three
        // controls of different heights, so their tops differ by a few pixels
        // while sharing a row, and that comparison failed on a layout that
        // was correct.
        rowH: d.querySelector(".leave-acts").getBoundingClientRect().height,
        tallest: Math.max(
          one(".leave-copy").height,
          one(".leave-stay").height,
          go.height,
        ),
        goRight: go.right,
        edge: d.getBoundingClientRect().right,
      };
    });
    if (b.rowH > b.tallest + 2) {
      throw new Error(
        `the actions occupy ${b.rowH.toFixed(0)}px for a ${b.tallest.toFixed(0)}px control, so they wrapped`,
      );
    }
    if (b.edge - b.goRight > 40) {
      throw new Error(
        `the primary sits ${(b.edge - b.goRight).toFixed(0)}px from the edge, so it is no longer at the end`,
      );
    }
  });

  // The two buttons are controls whose boundary is the only thing saying so,
  // the case 1.4.11 asks 3:1 for. This opens the dialog directly rather than
  // through leave.js: the paint is what is under test. The colours are read
  // from the browser and not from the stylesheet, since --ui-border is a
  // light-dark() and a computed style hands back the unresolved function
  // until something paints it.
  await check("the dialog's own buttons clear 3:1 in both themes", async () => {
    await l.goto(LEAVE, { waitUntil: "networkidle" });
    for (const theme of ["light", "dark"]) {
      const m = await l.evaluate((theme) => {
        document.documentElement.dataset.theme = theme;
        document.getElementById("leave").showModal();
        const out = {};
        for (const sel of [".leave-copy", ".leave-stay"]) {
          const s = getComputedStyle(document.querySelector(sel));
          out[sel] = [
            s.borderTopColor,
            getComputedStyle(document.getElementById("leave")).backgroundColor,
          ];
        }
        document.getElementById("leave").close();
        return out;
      }, theme);
      for (const [sel, [edge, behind]] of Object.entries(m)) {
        const r = ratio(edge, behind);
        if (r < 3)
          throw new Error(
            `${sel} in ${theme} is ${r.toFixed(2)}:1 on the dialog, want 3`,
          );
      }
    }
  });
  await l.close();
}

// ---- the card states ----
//
// Four things no markup assertion can see. The contrast helper here is not the
// one above: it composites the translucent veil itself, respects the paint
// order while doing it, and reads colours the browser hands back as oklab() or
// as color(srgb ...) with a channel just outside the gamut in scientific
// notation. Each of those three produced a plausible wrong number, and none of
// them produced an error.
{
  const st = await browser.newPage();
  await st.goto(STATES, { waitUntil: "networkidle" });
  console.log("\ncard states");

  await check("a disabled card is not reachable by Tab", async () => {
    const tag = await st.evaluate(() => {
      const card = document.querySelector(".card-off");
      return card ? card.querySelector(".card-name").tagName : null;
    });
    if (tag === null) throw new Error("no muted card on the page at all");
    if (tag !== "SPAN") {
      throw new Error(
        `the name of a retired service is a ${tag}, so Tab still reaches it`,
      );
    }
    // and walk it: a tag name is markup, and what has to hold is that the
    // name never takes focus
    await st.evaluate(() => document.body.focus());
    for (let i = 0; i < 40; i++) {
      await st.keyboard.press("Tab");
      const inside = await st.evaluate(
        () =>
          !!document.activeElement
            ?.closest(".card-off")
            ?.querySelector(".card-name:focus"),
      );
      if (inside) {
        throw new Error(
          `Tab landed inside the muted card's name after ${i + 1} presses`,
        );
      }
    }
  });
  // Not by comparing two cards in a row: the grid stretches every card in a
  // row to the tallest, so those two numbers are equal whatever the badge
  // does, and measured that way this check passed with the badge back inside
  // the box, the one failure it exists to catch. It asks instead whether the
  // badge is in flow: if it is, the card body starts below it, and the grid
  // charges that offset to every neighbour.
  await check("the badge takes no row inside the card", async () => {
    const drop = await st.evaluate(() => {
      const card = document.querySelector(".card-badge").closest(".card");
      const main = card.querySelector(".card-main");
      return (
        main.getBoundingClientRect().top - card.getBoundingClientRect().top
      );
    });
    if (drop > 3) {
      throw new Error(
        `the card body starts ${Math.round(drop)}px below the card's top: the badge is in flow, and the grid charges that height to every card beside it`,
      );
    }
  });

  await check(
    "the corner glyph stays in its corner on a muted card",
    async () => {
      const m = await st.evaluate(() => {
        const card = document.querySelector(".card-off");
        const g = card.querySelector(".card-more");
        if (!g) return null;
        const c = card.getBoundingClientRect();
        const r = g.getBoundingClientRect();
        return {
          inside: r.left >= c.left && r.right <= c.right,
          gap: c.right - r.right,
        };
      });
      if (m === null)
        throw new Error("the muted card has no detail glyph to measure");
      if (!m.inside) {
        throw new Error(
          "the glyph is outside the card: it was raised with position:relative, which overrides its own absolute",
        );
      }
      if (m.gap > 40) {
        throw new Error(
          `the glyph is ${Math.round(m.gap)}px from the end: it is no longer in the corner`,
        );
      }
    },
  );

  await check(
    "a muted card and every badge still clear their thresholds",
    async () => {
      const bad = await st.evaluate(() => {
        // A number here can be negative and can be in scientific notation: a
        // mix just outside the gamut serialises as -3.48948e-7, and [\d.]+
        // splits that into "3.48948" and "7", read as a blue channel of 891.
        // It reported a colour with 8:1 as having 1.39:1.
        const NUM = /-?\d*\.?\d+(?:e[-+]?\d+)?/gi;
        const probe = document.createElement("span");
        probe.style.display = "none";
        document.body.append(probe);
        // oklab() is not sRGB, and half this palette arrives that way because
        // --accent-ink is itself such a mix. The browser does the conversion.
        const toSRGB = (v) => {
          if (v.startsWith("rgb") || v.startsWith("color(srgb")) return v;
          probe.style.color = `color-mix(in srgb, ${v} 100%, transparent)`;
          return getComputedStyle(probe).color;
        };
        const clamp = (v) => Math.min(255, Math.max(0, v));
        const chan = (raw) => {
          const v = toSRGB(raw);
          const n = v.match(NUM).map(Number);
          if (v.startsWith("rgb")) return n.slice(0, 3).map(clamp);
          if (v.startsWith("color(srgb"))
            return n.slice(0, 3).map((x) => clamp(x * 255));
          throw new Error(`cannot read ${v} as sRGB channels`);
        };
        const alphaOf = (v) =>
          v.includes("/") ? Number(v.split("/")[1].match(NUM)[0]) : 1;
        const over = (fg, fa, bg) =>
          fg.map((c, i) => c * fa + bg[i] * (1 - fa));
        const lum = ([r, g, b]) => {
          const f = (x) =>
            (x /= 255) <= 0.04045 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4;
          return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b);
        };
        const ratio = (a, b) => {
          const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p);
          return (x + 0.05) / (y + 0.05);
        };

        const page = chan(getComputedStyle(document.body).backgroundColor);
        const fails = [];
        const fillOf = (el) =>
          over(
            chan(getComputedStyle(el).backgroundColor),
            alphaOf(getComputedStyle(el).backgroundColor),
            page,
          );

        const badges = document.querySelectorAll(".card-badge");
        if (!badges.length) fails.push("no badge on the page at all");
        for (const badge of badges) {
          const bg = fillOf(badge.closest(".card"));
          const r = ratio(chan(getComputedStyle(badge).color), bg);
          if (r < 4.5)
            fails.push(`${badge.textContent.trim()} at ${r.toFixed(2)}:1`);
        }

        const card = document.querySelector(".card-off");
        if (!card) fails.push("no muted card on the page at all");
        if (card) {
          const veil = getComputedStyle(card, "::after").backgroundColor;
          const v = chan(veil);
          const va = alphaOf(veil);
          const behind = over(v, va, fillOf(card));
          // paint order: what sits above the film is not read through it
          const above = (el) => (Number(getComputedStyle(el).zIndex) || 0) >= 2;
          const inkOf = (el) => {
            const c = chan(getComputedStyle(el).color);
            return above(el) || above(el.parentElement) ? c : over(v, va, c);
          };
          for (const [sel, floor] of [
            [".card-name", 4.5],
            [".card-desc", 4.5],
            [".flag-label", 4.5],
            [".card-more", 3],
          ]) {
            const el = card.querySelector(sel);
            if (!el) continue;
            const r = ratio(inkOf(el), behind);
            if (r < floor)
              fails.push(`${sel} at ${r.toFixed(2)}:1, floor ${floor}`);
          }
        }
        return fails;
      });
      if (bad.length) throw new Error(bad.join("; "));
    },
  );

  // The rule between the pill and the state must not be drawn with nothing to
  // its left, which is every disabling state, since none of them is monitored:
  // a stroke hanging off the start of a line reads as a rendering fault.
  await check(
    "the rule appears only when a pill precedes the state",
    async () => {
      const border = async (page, path) => {
        const p2 = await browser.newPage();
        await p2.goto(new URL(path, page).href, { waitUntil: "networkidle" });
        const m = await p2.evaluate(() => {
          const el = document.querySelector(".detail-state");
          if (!el) return null;
          const s = getComputedStyle(el);
          return {
            w: parseFloat(s.borderInlineStartWidth) || 0,
            style: s.borderInlineStartStyle,
            pill: !!document.querySelector(".status-slot")?.textContent.trim(),
          };
        });
        await p2.close();
        return m;
      };

      const withPill = await border(STATES, "beta/");
      if (!withPill)
        throw new Error("the beta detail page names no state at all");
      if (!withPill.pill)
        throw new Error(
          "the beta detail page has no pill, so this proves nothing",
        );
      if (withPill.w < 1 || withPill.style === "none") {
        throw new Error("no rule beside a state that follows a pill");
      }

      const alone = await border(STATES, "gone/");
      if (!alone)
        throw new Error("the retired detail page names no state at all");
      if (alone.pill)
        throw new Error(
          "the retired detail page has a pill, so this proves nothing",
        );
      if (alone.w >= 1 && alone.style !== "none") {
        throw new Error(
          "a rule is drawn with nothing to its left on a page that has no pill",
        );
      }
    },
  );

  await st.close();
}

// ---- A12: the trail, the headings that answer it, and where a jump lands ----

{
  const wp = await browser.newContext();

  // Halfway between the middle of the lowercase and the middle of the
  // capitals. Centred on the line box a mark reads high, since the box carries
  // descender space no lowercase letter reaches; dropped onto the lowercase it
  // reads low, since the capital and the ascenders have nothing under them to
  // answer. Both references come from the layout and not from the font's own
  // tables: a zero-height inline-block sits on the baseline and 1ex is the
  // x-height. Computing the baseline from the font's ascent instead put it
  // 0.45px out, which is most of what is being corrected here.
  const offMark = (page, markSel, nameSel) =>
    page.evaluate(
      ([m, n]) => {
        const mark = document.querySelector(m),
          name = document.querySelector(n);
        if (!mark || !name || !mark.getClientRects().length) return null;
        const ex = document.createElement("span");
        ex.style.cssText = "display:inline-block;width:0;height:1ex";
        const base = document.createElement("span");
        base.style.cssText = "display:inline-block;width:0;height:0";
        name.appendChild(ex);
        name.appendChild(base);
        const xh = ex.getBoundingClientRect().height;
        const baseline = base.getBoundingClientRect().bottom;
        ex.remove();
        base.remove();
        const cs = getComputedStyle(name);
        const cv = document.createElement("canvas").getContext("2d");
        cv.font = `${cs.fontWeight} ${cs.fontSize} ${cs.fontFamily}`;
        const cap = cv.measureText("H").actualBoundingBoxAscent;
        const mr = mark.getBoundingClientRect();
        const lower = baseline - xh / 2;
        const caps = baseline - cap / 2;
        return mr.top + mr.height / 2 - (lower + caps) / 2;
      },
      [markSel, nameSel],
    );

  await check(
    "a waypoint diamond sits between the lowercase and the capitals",
    async () => {
      const wide = await wp.newPage();
      await wide.setViewportSize({ width: 1440, height: 900 });
      await wide.goto(MANY);
      const rail = await offMark(wide, ".toc-mark", ".toc-name");
      const heading = await offMark(wide, ".way-mark", ".way-name");
      await wide.close();
      if (rail === null) throw new Error("the rail is not painted at 1440px");
      if (heading === null) throw new Error("no diamond beside a heading");
      for (const [what, off] of [
        ["the rail's", rail],
        ["a heading's", heading],
      ]) {
        if (Math.abs(off) > 0.5) {
          throw new Error(
            `${what} diamond is ${off.toFixed(2)}px off that midpoint`,
          );
        }
      }
    },
  );

  // The glyph hangs in the margin rather than in the row, so the two things
  // worth pinning are that it appears and that nothing else moves when it
  // does. It is also off below the breakpoint, where the margin cannot hold it.
  await check(
    "the link glyph appears in the margin, moving nothing",
    async () => {
      const page = await wp.newPage();
      await page.setViewportSize({ width: 1280, height: 900 });
      await page.goto(MANY);
      const before = await page.evaluate(() => {
        const way = [...document.querySelectorAll(".way")][1];
        const g = way.querySelector(".way-link");
        const n = way.querySelector(".way-name");
        return {
          opacity: +getComputedStyle(g).opacity,
          name: n.getBoundingClientRect().left,
          rowLeft: way.getBoundingClientRect().left,
        };
      });
      if (before.opacity !== 0) {
        throw new Error(
          `the glyph shows at rest, at opacity ${before.opacity}`,
        );
      }
      await (await page.$$(".way-name a"))[1].hover();
      await page.waitForTimeout(300);
      const after = await page.evaluate(() => {
        const way = [...document.querySelectorAll(".way")][1];
        const g = way.querySelector(".way-link");
        const gr = g.getBoundingClientRect();
        return {
          opacity: +getComputedStyle(g).opacity,
          name: way.querySelector(".way-name").getBoundingClientRect().left,
          glyphRight: gr.right,
          glyphLeft: gr.left,
          rowLeft: way.getBoundingClientRect().left,
        };
      });
      await page.close();
      // Visible, not opaque: it is deliberately held back, so this is a floor
      // for "you can see it" and not the value the rule happens to carry.
      if (after.opacity < 0.5) {
        throw new Error(`hovering left the glyph at opacity ${after.opacity}`);
      }
      if (Math.abs(after.name - before.name) > 0.5) {
        throw new Error(
          `the heading moved ${(after.name - before.name).toFixed(1)}px when the glyph appeared`,
        );
      }
      if (after.glyphRight > before.rowLeft + 0.5) {
        throw new Error(
          "the glyph overlaps the row rather than sitting beside it",
        );
      }
      if (after.glyphLeft < 0) {
        throw new Error(
          `the glyph is ${after.glyphLeft.toFixed(1)}px off the left edge`,
        );
      }
    },
  );

  await check("and stays away where the margin cannot hold it", async () => {
    const page = await wp.newPage();
    await page.setViewportSize({ width: 1024, height: 900 });
    await page.goto(MANY);
    await (await page.$$(".way-name a"))[1].hover();
    await page.waitForTimeout(300);
    // The glyph of the heading being hovered, not the first one in the
    // document: reading that one leaves the check green whatever the rule
    // does, since nothing is ever hovering it.
    const o = await page.evaluate(() => {
      const g = [...document.querySelectorAll(".way")][1].querySelector(
        ".way-link",
      );
      if (!g) throw new Error("no glyph beside the heading at all");
      return +getComputedStyle(g).opacity;
    });
    await page.close();
    if (o !== 0) {
      throw new Error(`at 1024px the glyph shows anyway, at opacity ${o}`);
    }
  });

  await check("the rail's spine runs through its diamonds", async () => {
    const wide = await wp.newPage();
    await wide.setViewportSize({ width: 1440, height: 900 });
    await wide.goto(MANY);
    const offs = await wide.evaluate(() => {
      const ul = document.querySelector(".toc ul");
      if (!ul || !ul.getClientRects().length) return null;
      const cs = getComputedStyle(ul, "::before");
      const start = parseFloat(cs.insetInlineStart || cs.left);
      const centre =
        ul.getBoundingClientRect().left + start + parseFloat(cs.width) / 2;
      return [...document.querySelectorAll(".toc-mark")].map((m) => {
        const r = m.getBoundingClientRect();
        return r.left + r.width / 2 - centre;
      });
    });
    await wide.close();
    if (!offs) throw new Error("the rail is not painted at 1440px");
    if (!offs.length) throw new Error("the rail carries no diamonds");
    const worst = offs.reduce((a, b) => (Math.abs(b) > Math.abs(a) ? b : a));
    if (Math.abs(worst) > 0.25) {
      throw new Error(
        `a diamond sits ${worst.toFixed(2)}px ${worst < 0 ? "left" : "right"} of the spine`,
      );
    }
  });

  await check("a category heading is a link to its own anchor", async () => {
    const page = await wp.newPage();
    await page.goto(MANY);
    const pairs = await page.$$eval(".way-name", (hs) =>
      hs.map((h) => ({
        id: h.id,
        href: h.querySelector("a")?.getAttribute("href"),
      })),
    );
    await page.close();
    if (!pairs.length) throw new Error("no category heading on the page");
    for (const { id, href } of pairs) {
      if (href !== `#${id}`) {
        throw new Error(`heading ${id} links to ${href} rather than to itself`);
      }
    }
  });

  // The gap above a heading is 2.6rem, so a margin past it pulls the previous
  // section's last card back into view. Both edges are asserted: a margin of
  // zero would satisfy the second on its own.
  for (const [label, size] of [
    ["a wide screen", { width: 1440, height: 900 }],
    ["a phone", PHONE],
  ]) {
    await check(
      `clicking a heading lands it clear of the top on ${label}`,
      async () => {
        const page = await wp.newPage();
        await page.setViewportSize(size);
        await page.emulateMedia({ reducedMotion: "reduce" });
        await page.goto(MANY);
        const m = await page.evaluate(async () => {
          const cats = [...document.querySelectorAll(".cat")];
          const target = cats[2];
          scrollTo(0, 0);
          await new Promise((r) => setTimeout(r, 250));
          target.querySelector(".way-name a").click();
          await new Promise((r) => setTimeout(r, 700));
          const prev = [...cats[1].querySelectorAll(".card")].pop();
          return {
            hash: location.hash,
            top: target.querySelector(".way-name").getBoundingClientRect().top,
            prevBottom: prev.getBoundingClientRect().bottom,
          };
        });
        await page.close();
        if (m.hash !== "#cat-charlie") {
          throw new Error(`the click set ${m.hash || "no hash at all"}`);
        }
        if (m.top < 24) {
          throw new Error(
            `the heading lands ${m.top.toFixed(1)}px from the top, too tight`,
          );
        }
        if (m.prevBottom > 0) {
          throw new Error(
            `${m.prevBottom.toFixed(1)}px of the previous section's last card is in view`,
          );
        }
      },
    );
  }

  // Reading a language is not an action. The link led back to the page it was
  // on, and on a site with one language it was the only thing there to click.
  for (const [label, url] of [
    ["among several", SITE],
    ["when it is the only one", MANY],
  ]) {
    await check(
      `the language you are reading is not a control, ${label}`,
      async () => {
        const page = await wp.newPage();
        await page.goto(url);
        const m = await page.evaluate(() => {
          const el = document.querySelector(".langs [aria-current]");
          if (!el) return null;
          el.focus?.();
          return {
            tag: el.tagName.toLowerCase(),
            href: el.getAttribute("href"),
            tookFocus: document.activeElement === el,
            others: document.querySelectorAll(".langs a[href]").length,
          };
        });
        await page.close();
        if (!m) throw new Error("nothing in the switcher is marked as current");
        if (m.tag === "a" || m.href !== null) {
          throw new Error(
            `the current language is still a <${m.tag} href=${m.href}>`,
          );
        }
        if (m.tookFocus)
          throw new Error("the current language still takes focus");
      },
    );
  }

  await check("every other language is still one click away", async () => {
    const page = await wp.newPage();
    await page.goto(SITE);
    const hrefs = await page.$$eval(".langs a[href]", (as) =>
      as.map((a) => a.getAttribute("href")),
    );
    await page.close();
    // example/ carries fr and en, so exactly one link is left beside the label
    if (hrefs.length !== 1) {
      throw new Error(
        `expected one switchable language, found ${hrefs.length}`,
      );
    }
    if (!hrefs[0].includes("?choose")) {
      throw new Error(`${hrefs[0]} does not carry ?choose, so it cannot pin`);
    }
  });

  await wp.close();
}

await browser.close();
console.log(failures ? `\n${failures} failed` : "\nall passed");
process.exit(failures ? 1 : 0);
