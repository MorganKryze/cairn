// Scroll chrome for the header, the back-to-top button and the jump-to-category
// select. All progressive: without JS the header stays put, the button stays
// hidden and the select is inert.
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

  const sel = document.querySelector('.toc-select');
  if (sel) {
    sel.addEventListener('change', () => {
      if (sel.value) location.hash = sel.value;
      sel.selectedIndex = 0;
    });
  }

  update();
})();
