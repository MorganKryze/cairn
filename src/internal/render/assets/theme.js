// The pre-paint script in <head> applies a stored theme before first render;
// this wires the toggle. The choice lives in localStorage, not a cookie.
(() => {
  const btn = document.getElementById('theme-toggle');
  if (!btn) return;
  const root = document.documentElement;
  const dark = () => (root.dataset.theme
    ? root.dataset.theme === 'dark'
    : matchMedia('(prefers-color-scheme: dark)').matches);
  // A toggle that never says whether it is pressed is a button and no state to
  // a screen reader (WCAG 4.1.2). Pressed means dark. The server cannot render
  // it: the button ships hidden, and by the time it is revealed the pre-paint
  // script may already have applied a stored theme, so only here is the
  // current one known.
  const sync = () => btn.setAttribute('aria-pressed', String(dark()));
  sync();
  btn.hidden = false;
  btn.addEventListener('click', () => {
    const next = dark() ? 'light' : 'dark';
    root.dataset.theme = next;
    try { localStorage.setItem('theme', next); } catch (e) {}
    sync();
  });
})();
