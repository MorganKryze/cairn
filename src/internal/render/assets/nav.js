// Scroll chrome for the header, the back-to-top button and the jump-to-category
// select, and the phone header's long-name row. All progressive: without JS
// the header stays put, the button stays hidden, the select is inert and a
// long name wraps beside the tools.
(() => {
  const NEAR_TOP = 80;   // still counts as the top of the page
  const JITTER = 4;      // below this a scroll is not a direction

  const header = document.querySelector('header');
  const totop = document.querySelector('.totop');
  const nav = document.querySelector('.nav');
  const burger = nav && nav.querySelector('summary');
  let last = scrollY;

  // A scroll dismisses the open menu, which is right for a pointer and wrong
  // for a keyboard: tabbing to a menu link the browser has to scroll into view
  // fires this handler, and closing the menu drops focus to <body>, so the next
  // Tab restarts at the skip link. Focus on the burger itself is the pointer
  // case: clicking it leaves focus there, and the summary survives the close.
  const keyboardInside = () => {
    const a = document.activeElement;
    return !!a && a !== burger && nav.contains(a);
  };

  const update = () => {
    const y = scrollY;
    if (nav && nav.open && !keyboardInside()) nav.open = false;
    if (header) {
      if (y < NEAR_TOP || y < last - JITTER) header.classList.remove('header-hidden');
      else if (y > last + JITTER) header.classList.add('header-hidden');
    }
    if (totop) totop.classList.toggle('show', y > innerHeight * 0.9);
    last = y;
  };

  let ticking = false;
  addEventListener('scroll', () => {
    if (ticking) return;
    ticking = true;
    requestAnimationFrame(() => { update(); ticking = false; });
  }, { passive: true });

  update();

  // On a phone the site name shares its row with the tools until it would
  // take more than two lines there, or holds a word wider than its share of
  // the row; then it gets the whole row and the tools one under it. CSS cannot
  // count the lines a text takes, and a count of characters is wrong both
  // ways: at 360px a 27-character name took five lines and a 31-character one
  // two. A word with no break in it, like dash.example.com, does not wrap at
  // all: it keeps its width and runs under the tools. So the name is laid out
  // beside the tools, its line boxes counted and held to the brand's box, and
  // the class set from that, all before the next paint. Nothing here observes
  // the header, so setting the class cannot call this again.
  const topbar = document.querySelector('.topbar');
  const brand = topbar && topbar.querySelector('.brand');
  const phone = matchMedia('(max-width: 44rem)');
  const crowded = () => {
    const texts = [...brand.childNodes].filter((n) => n.nodeType === 3 && n.data.trim());
    if (!texts.length) return false;
    const range = document.createRange();
    range.setStart(texts[0], 0);
    const end = texts[texts.length - 1];
    range.setEnd(end, end.length);
    const rects = [...range.getClientRects()];
    const box = brand.getBoundingClientRect();
    if (rects.some((r) => r.left < box.left - 1 || r.right > box.right + 1)) return true;
    let n = 0, prev = -Infinity;
    for (const top of rects.map((r) => r.top).sort((a, b) => a - b)) {
      if (top - prev > 2) n++;
      prev = top;
    }
    return n > 2;
  };
  const split = () => {
    topbar.classList.remove('topbar-split');
    if (phone.matches && crowded()) topbar.classList.add('topbar-split');
  };
  if (brand) {
    let measuring = false;
    addEventListener('resize', () => {
      if (measuring) return;
      measuring = true;
      requestAnimationFrame(() => { split(); measuring = false; });
    }, { passive: true });
    phone.addEventListener('change', split);
    // The display face changes the width of every word once it lands.
    document.fonts.ready.then(split);
    split();
  }
})();
