// The pre-paint script in <head> hides a dismissed note before first render;
// this only wires the button.
(() => {
  const x = document.getElementById('about-x');
  if (!x) return;
  x.hidden = false;
  x.addEventListener('click', () => {
    document.documentElement.setAttribute('data-noabout', '');
    const h = document.documentElement.getAttribute('data-about') || 'off';
    const secure = location.protocol === 'https:' ? '; secure' : '';
    // scope the cookie to the mount point, so a sub-path install does not
    // write across the whole domain (data-base mirrors -base-path)
    const path = (document.documentElement.dataset.base || '') + '/';
    document.cookie = `about=${h}; path=${path}; max-age=31536000; samesite=lax${secure}`;
  });
})();
