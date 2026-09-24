// The two cookies a page writes for itself. The pre-paint script in <head>
// hides a dismissed note before first render; this wires its button.
//
// The other is the language pinned from the switcher, whose links carry
// ?choose. cairn's server turns that into the locale cookie and redirects the
// query away, so a page it serves never sees one. A static host hands the page
// the query as it came, and this writes the cookie the export's root page reads.
(() => {
  const secure = location.protocol === 'https:' ? '; secure' : '';
  // scoped to the mount point, so a sub-path install does not write across
  // the whole domain (data-base mirrors -base-path)
  const path = (document.documentElement.dataset.base || '') + '/';

  if (new URLSearchParams(location.search).has('choose')) {
    document.cookie = `locale=${document.documentElement.lang}; path=${path}; max-age=31536000; samesite=lax${secure}`;
    history.replaceState(null, '', location.pathname + location.hash);
  }

  const x = document.getElementById('about-x');
  if (!x) return;
  x.hidden = false;
  x.addEventListener('click', () => {
    document.documentElement.setAttribute('data-noabout', '');
    const h = document.documentElement.getAttribute('data-about') || 'off';
    document.cookie = `about=${h}; path=${path}; max-age=31536000; samesite=lax${secure}`;
  });
})();
