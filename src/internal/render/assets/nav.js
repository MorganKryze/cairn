// Scroll chrome: hide the header on the way down and bring it back on the way
// up, reveal the back-to-top button past a screen, and wire the "jump to
// category" select. All progressive: without JS the header just stays put, the
// button stays hidden, and the select is inert.
(() => {
  const header = document.querySelector('header');
  const totop = document.querySelector('.totop');
  const nav = document.querySelector('.nav');
  const burger = nav && nav.querySelector('summary');
  let last = scrollY;

  // A scroll dismisses the open menu, which is right for a pointer and wrong
  // for a keyboard: tabbing to a menu link the browser has to scroll into view
  // fires this handler, and closing the menu underneath both takes away what
  // the user was tabbing through and drops focus to <body>, so the next Tab
  // restarts at the skip link. Focus on the burger itself is still the pointer
  // case: clicking it leaves focus there, and the summary survives the close.
  const keyboardInside = () => {
    const a = document.activeElement;
    return !!a && a !== burger && nav.contains(a);
  };

  const update = () => {
    const y = scrollY;
    if (nav && nav.open && !keyboardInside()) nav.open = false;
    if (header) {
      // near the top or scrolling up: show; scrolling down past a bit: hide
      if (y < 80 || y < last - 4) header.classList.remove('header-hidden');
      else if (y > last + 4) header.classList.add('header-hidden');
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
