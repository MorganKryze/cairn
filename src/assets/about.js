// The pre-paint script in <head> hides a dismissed note before first render;
// this only wires the button.
(() => {
  const x = document.getElementById('about-x');
  if (!x) return;
  x.hidden = false;
  x.addEventListener('click', () => {
    document.documentElement.setAttribute('data-noabout', '');
    document.cookie = 'about=off; path=/; max-age=31536000; samesite=lax';
  });
})();
