// The root page of a static export. The server answers / with a redirect to
// the language it picks from the locale cookie, then from Accept-Language; a
// static host can do neither, so the browser makes the same choice here, from
// the same list and in the same order, before anything paints.
(() => {
  const root = document.documentElement;
  const locales = root.dataset.locales.split(' ');
  const base = (tag) => tag.split('-')[0].toLowerCase();
  const pick = () => {
    const chosen = document.cookie.match(/(?:^|; )locale=([^;]*)/);
    if (chosen && locales.includes(chosen[1])) return chosen[1];
    for (const tag of navigator.languages || [navigator.language]) {
      const hit = locales.find((l) => l.toLowerCase() === tag.toLowerCase() || base(l) === base(tag));
      if (hit) return hit;
    }
    return locales[0];
  };
  location.replace(`${root.dataset.base || ''}/${pick()}/`);
})();
