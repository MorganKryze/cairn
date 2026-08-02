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
  // A themed favicon is the one piece of artwork this stylesheet cannot
  // reach, so it ships as a second <link> whose media query answers to the
  // system. Pressing the button has to override that the way it overrides
  // everything else, and the only handle is the query itself: "all" always
  // matches, "not all" never does, and dropping back to the original hands
  // the decision to the system again.
  const favDark = document.querySelector('link[rel="icon"][media]');
  const favQuery = favDark && favDark.media;
  const syncFavicon = () => {
    if (!favDark) return;
    favDark.media = root.dataset.theme
      ? (root.dataset.theme === 'dark' ? 'all' : 'not all')
      : favQuery;
  };
  const sync = () => {
    btn.setAttribute('aria-pressed', String(dark()));
    syncFavicon();
  };
  sync();
  btn.hidden = false;
  btn.addEventListener('click', () => {
    const next = dark() ? 'light' : 'dark';
    root.dataset.theme = next;
    try { localStorage.setItem('theme', next); } catch (e) {}
    sync();
  });
})();
