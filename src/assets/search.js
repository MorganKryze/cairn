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
  const cards = Array.from(document.querySelectorAll('.card'), el => {
    const main = el.querySelector('.card-main').cloneNode(true);
    main.querySelectorAll('.visually-hidden, .status-pill').forEach(n => n.remove());
    return {
      el,
      name: norm(el.querySelector('.card-name').textContent),
      text: norm(main.textContent + ' ' + (el.dataset.tags || '')),
    };
  });
  const cats = Array.from(document.querySelectorAll('.cat'));
  const empty = document.getElementById('empty');

  input.addEventListener('input', () => {
    const words = norm(input.value).split(/\s+/).filter(Boolean);
    // Name matches win: when the query hits a service's name, show only those,
    // so a full name doesn't drag in cards that merely mention it. Fall back to
    // the full text (descriptions, tags) only when no name matches.
    const nameHit = c => words.every(w => c.name.includes(w));
    const byName = words.length > 0 && cards.some(nameHit);
    const hit = byName ? nameHit : c => words.every(w => c.text.includes(w));
    let shown = 0;
    for (const c of cards) {
      const ok = hit(c);
      c.el.hidden = !ok;
      if (ok) shown++;
    }
    for (const s of cats) s.hidden = !s.querySelector('.card:not([hidden])');
    empty.hidden = shown > 0;
  });
})();
