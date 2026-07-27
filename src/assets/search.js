(() => {
  const input = document.getElementById('q');
  if (!input) return;
  input.disabled = false;

  const kbd = document.querySelector('.search-kbd');
  kbd.textContent = /Mac|iPhone|iPad/.test(navigator.platform) ? '⌘K' : 'Ctrl K';
  kbd.hidden = false;

  document.addEventListener('keydown', e => {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
      e.preventDefault();
      input.focus();
      input.select();
      return;
    }
    if (e.metaKey || e.ctrlKey || e.altKey || e.key.length !== 1) return;
    const a = document.activeElement;
    if (a === input || /^(INPUT|TEXTAREA|SELECT)$/.test(a.tagName) || a.isContentEditable) return;
    // focus on keydown: the default action then types the character here
    input.focus();
  });

  const COMBINING = /[̀-ͯ]/g;
  const norm = s => s.normalize('NFD').replace(COMBINING, '').toLowerCase();
  const cats = Array.from(document.querySelectorAll('.cat'));
  const cards = Array.from(document.querySelectorAll('.card'), (el, dom) => {
    const main = el.querySelector('.card-main').cloneNode(true);
    main.querySelectorAll('.visually-hidden, .status-pill').forEach(n => n.remove());
    return {
      el,
      dom,                                    // original position, the tie-breaker
      cat: cats.indexOf(el.closest('.cat')),  // which category row it lives in
      name: norm(el.querySelector('.card-name').textContent),
      text: norm(main.textContent + ' ' + (el.dataset.tags || '')),
    };
  });
  const empty = document.getElementById('empty');

  // The visible matches, in the order they now read on screen, and the index of
  // the "selected" one (the one Enter opens). -1 when nothing is selected.
  let matches = [];
  let sel = -1;

  const setSel = i => {
    if (sel >= 0 && matches[sel]) matches[sel].el.classList.remove('sel');
    sel = i;
    if (sel >= 0 && matches[sel]) {
      const el = matches[sel].el;
      el.classList.add('sel');
      el.scrollIntoView({ block: 'nearest' });
    }
  };

  const apply = () => {
    const q = norm(input.value).trim();
    const words = q.split(/\s+/).filter(Boolean);
    // Name matches win: when the query hits a service's name, show only those,
    // so a full name doesn't drag in cards that merely mention it. Fall back to
    // the full text (descriptions, tags) only when no name matches.
    const nameHit = c => words.every(w => c.name.includes(w));
    const byName = words.length > 0 && cards.some(nameHit);
    const hit = byName ? nameHit : c => words.every(w => c.text.includes(w));
    // exact name > prefix > name substring > keyword-only
    const score = c => c.name === q ? 4 : c.name.startsWith(q) ? 3 : c.name.includes(q) ? 2 : 1;

    const hits = [];
    for (const c of cards) {
      const ok = hit(c);
      c.el.hidden = !ok;
      c.el.classList.remove('sel');
      if (ok && words.length) {
        c.score = score(c);
        c.el.style.order = String(-c.score); // the better the match, the further left in its row
        hits.push(c);
      } else {
        c.el.style.order = '';
      }
    }
    // read them in on-screen order: category by category, best score first,
    // original order breaking ties
    hits.sort((a, b) => a.cat - b.cat || b.score - a.score || a.dom - b.dom);
    matches = hits;

    for (const s of cats) s.hidden = !s.querySelector('.card:not([hidden])');
    empty.hidden = matches.length > 0;
    // Preselect the strongest match: it also sits first-left in its category, so
    // typing a name lands you on that one card, top-left of its row.
    sel = -1;
    if (matches.length) {
      let best = 0;
      for (let i = 1; i < matches.length; i++) if (matches[i].score > matches[best].score) best = i;
      setSel(best);
    }
  };

  input.addEventListener('input', apply);

  // ponytail: a class + a real link click, not a full ARIA combobox. Upgrade to
  // listbox/aria-activedescendant if screen-reader nav is ever asked for.
  input.addEventListener('keydown', e => {
    if (e.key === 'Escape') { input.value = ''; apply(); return; }
    if (!matches.length) return;
    if (e.key === 'ArrowDown') { e.preventDefault(); setSel(Math.min(sel + 1, matches.length - 1)); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); setSel(Math.max(sel - 1, 0)); }
    else if (e.key === 'Enter' && sel >= 0) { e.preventDefault(); matches[sel].el.querySelector('.card-name').click(); }
  });
})();
