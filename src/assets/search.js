(() => {
  const input = document.getElementById('q');
  if (!input) return;
  document.getElementById('search').hidden = false;

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
    return { el, text: norm(main.textContent + ' ' + (el.dataset.tags || '')) };
  });
  const cats = Array.from(document.querySelectorAll('.cat'));
  const empty = document.getElementById('empty');

  input.addEventListener('input', () => {
    const words = norm(input.value).split(/\s+/).filter(Boolean);
    let shown = 0;
    for (const c of cards) {
      const hit = words.every(w => c.text.includes(w));
      c.el.hidden = !hit;
      if (hit) shown++;
    }
    for (const s of cats) s.hidden = !s.querySelector('.card:not([hidden])');
    empty.hidden = shown > 0;
  });
})();
