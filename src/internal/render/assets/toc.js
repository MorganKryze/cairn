(() => {
  const toc = document.querySelector('.toc');
  if (!toc) return;

  const links = new Map();
  // html/template percent-encodes accented category ids in href
  for (const a of toc.querySelectorAll('a')) links.set(decodeURIComponent(a.hash).slice(1), a);
  const sections = Array.from(document.querySelectorAll('.cat'));
  let current;

  // A row that scrolls sideways has to say so. Left to itself it can cut a chip
  // exactly at the edge and read as a complete list, which is how a reader
  // misses two thirds of the categories. The edge with more behind it fades.
  //
  // Measured from the rectangles rather than from scrollLeft: that value is
  // negative in a right-to-left page in some engines and positive in others,
  // and what the fade needs is the physical side either way.
  const rail = toc.querySelector('ul');
  const edges = () => {
    const r = rail.getBoundingClientRect();
    let left = false, right = false;
    for (const li of rail.children) {
      const b = li.getBoundingClientRect();
      if (b.right < r.left + 1) left = true;
      if (b.left > r.right - 1) right = true;
    }
    const v = [left && 'left', right && 'right'].filter(Boolean).join(' ');
    if (v) rail.setAttribute('data-more', v); else rail.removeAttribute('data-more');
  };
  rail.addEventListener('scroll', edges, { passive: true });
  addEventListener('resize', edges, { passive: true });
  edges();

  const spy = () => {
    const visible = sections.filter(s => !s.hidden);
    // Nothing until a section has actually crossed: at the top of the page the
    // reader is in the tagline and the note, not in the first category, and
    // marking it there says they are somewhere they are not.
    let active = null;
    for (const s of visible) {
      if (s.getBoundingClientRect().top <= innerHeight * 0.4) active = s;
    }
    if (scrollY + innerHeight >= document.documentElement.scrollHeight - 2) {
      active = visible[visible.length - 1];
    }
    const next = active && links.get(active.getAttribute('aria-labelledby'));
    if (next === current) return;
    if (current) current.removeAttribute('aria-current');
    if (next) next.setAttribute('aria-current', 'true');
    current = next;
  };

  let ticking = false;
  const onScroll = () => {
    if (ticking) return;
    ticking = true;
    requestAnimationFrame(() => { spy(); ticking = false; });
  };
  addEventListener('scroll', onScroll, { passive: true });
  addEventListener('resize', onScroll, { passive: true });

  // search hides sections; mirror that on their trail entries
  const mo = new MutationObserver(() => {
    for (const s of sections) {
      const a = links.get(s.getAttribute('aria-labelledby'));
      if (a) a.parentElement.hidden = s.hidden;
    }
    spy();
  });
  for (const s of sections) mo.observe(s, { attributes: true, attributeFilter: ['hidden'] });

  spy();
})();
