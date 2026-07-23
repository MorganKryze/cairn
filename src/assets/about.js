(() => {
  const sec = document.querySelector('.about');
  if (!sec) return;
  if (document.cookie.split('; ').includes('about=off')) {
    sec.hidden = true;
    return;
  }
  const x = document.getElementById('about-x');
  x.hidden = false;
  x.addEventListener('click', () => {
    sec.hidden = true;
    document.cookie = 'about=off; path=/; max-age=31536000; samesite=lax';
  });
})();
