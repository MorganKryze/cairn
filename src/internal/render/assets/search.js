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
    // Space is out: it pages the document down, and it is how a keyboard user
    // presses a button or opens a <details>. Nobody starts a query with one.
    if (e.metaKey || e.ctrlKey || e.altKey || e.key.length !== 1 || e.key === ' ') return;
    const a = document.activeElement;
    // Every element that does something with a plain key, not just the text
    // fields: stealing focus from a focused button ate its activation.
    if (a === input || /^(INPUT|TEXTAREA|SELECT|BUTTON|SUMMARY|A)$/.test(a.tagName) || a.isContentEditable) return;
    // focus on keydown: the default action then types the character here
    input.focus();
  });

  const COMBINING = /[̀-ͯ]/g;
  const norm = s => s.normalize('NFD').replace(COMBINING, '').toLowerCase();
  const cats = Array.from(document.querySelectorAll('.cat'));
  // Filtering is a visual change; a live region is what makes it reach a
  // screen reader (WCAG 4.1.3). It also names the card Enter would open.
  const count = document.getElementById('count');
  const announce = (n, name) => {
    if (!count) return;
    if (!n) { count.textContent = ''; return; }
    // data-single/-plural, not data-one: html/template strips "data-" and would
    // read "one" as an on* event handler, JSON-quoting the value.
    const label = n === 1 ? count.dataset.single : count.dataset.plural.replace('%d', n);
    count.textContent = name ? `${label}, ${name}` : label;
  };
  const cards = Array.from(document.querySelectorAll('.card'), (el, pos) => {
    const main = el.querySelector('.card-main').cloneNode(true);
    main.querySelectorAll('.visually-hidden, .status-pill, .card-more').forEach(n => n.remove());
    return {
      el,
      pos,
      cat: cats.indexOf(el.closest('.cat')),
      label: el.querySelector('.card-name').textContent.trim(),
      name: norm(el.querySelector('.card-name').textContent),
      text: norm(main.textContent + ' ' + (el.dataset.tags || '')),
    };
  });
  const empty = document.getElementById('empty');

  // the visible matches in on-screen order; sel is the one Enter opens, -1 none
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
    // so a full name does not drag in cards that merely mention it. The full
    // text (descriptions, tags) is the fallback.
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
        c.el.style.order = String(-c.score); // the better the match, the earlier in its row
        hits.push(c);
      } else {
        c.el.style.order = '';
      }
    }
    // on-screen order: category, then score, then original position
    hits.sort((a, b) => a.cat - b.cat || b.score - a.score || a.pos - b.pos);
    matches = hits;

    for (const s of cats) s.hidden = !s.querySelector('.card:not([hidden])');
    // An empty box is not a search that found nothing: clearing the field
    // brings every card back, and `matches` is empty then for the keyboard's
    // sake. #empty carries role="status", so reading it as a result announced
    // "no results" on the way back to the full list.
    empty.hidden = !(words.length && matches.length === 0);
    // the strongest match, which also sits first in its category row
    sel = -1;
    if (matches.length) {
      let best = 0;
      for (let i = 1; i < matches.length; i++) if (matches[i].score > matches[best].score) best = i;
      setSel(best);
    }
    announce(words.length ? matches.length : 0, sel >= 0 ? matches[sel].label : '');
  };

  input.addEventListener('input', apply);

  // A class and a real link click, not a full ARIA combobox: the live region
  // above carries the count and the pick, which is what a listbox would
  // announce anyway.
  input.addEventListener('keydown', e => {
    if (e.key === 'Escape') { input.value = ''; apply(); return; }
    if (!matches.length) return;
    if (e.key === 'ArrowDown') { e.preventDefault(); setSel(Math.min(sel + 1, matches.length - 1)); announce(matches.length, matches[sel].label); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); setSel(Math.max(sel - 1, 0)); announce(matches.length, matches[sel].label); }
    else if (e.key === 'Enter' && sel >= 0) { e.preventDefault(); matches[sel].el.querySelector('.card-name').click(); }
  });
})();
