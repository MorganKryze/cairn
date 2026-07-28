// The pre-paint script in <head> applies a stored theme before first render;
// this wires the toggle. The choice lives in localStorage, not a cookie.
(() => {
  const btn = document.getElementById('theme-toggle');
  if (!btn) return;
  btn.hidden = false;
  btn.addEventListener('click', () => {
    const root = document.documentElement;
    const dark = root.dataset.theme
      ? root.dataset.theme === 'dark'
      : matchMedia('(prefers-color-scheme: dark)').matches;
    const next = dark ? 'light' : 'dark';
    root.dataset.theme = next;
    try { localStorage.setItem('theme', next); } catch (e) {}
  });
})();
