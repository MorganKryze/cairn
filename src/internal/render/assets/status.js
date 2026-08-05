// The pages are static HTML, so a tab left open on a wall display keeps showing
// whatever the monitor said the moment it was opened.
//
// This refetches the page it is already on and swaps the status pills out of
// the answer, nothing else. No API, no partial endpoint, no state of its own:
// the server already renders the markup wanted, and the request is a
// conditional GET that comes back 304 most of the time.
(function () {
  var every = (Number(document.currentScript.dataset.poll) || 60) * 1000;

  function slots(root) {
    var out = {};
    root.querySelectorAll('.status-slot[data-status]').forEach(function (el) {
      out[el.dataset.status] = el;
    });
    return out;
  }

  function refresh() {
    fetch(location.href)
      .then(function (r) {
        return r.ok ? r.text() : null;
      })
      .then(function (html) {
        if (!html) return;
        // Parsed, never assigned: DOMParser builds an inert document with no
        // browsing context, and what moves across is a .status-slot element
        // cairn's own template rendered. No markup is built from a string
        // here, so there is no sink to inject into.
        var fresh = slots(new DOMParser().parseFromString(html, 'text/html'));
        var live = slots(document);
        Object.keys(live).forEach(function (id) {
          if (!(id in fresh)) return;
          // replacing an unchanged pill restarts the online dot's pulse every
          // interval, a beacon blinking for no reason
          if (live[id].outerHTML === fresh[id].outerHTML) return;
          // A pill is a link: pulling it out from under the keyboard drops
          // focus to the top of the document mid-read.
          if (live[id].contains(document.activeElement)) return;
          live[id].replaceWith(fresh[id]);
        });
      })
      .catch(function () {
        // Unreachable or nonsense: the pills stay as they are and the next
        // tick tries again. This has no business raising an error at the
        // visitor.
      });
  }

  setInterval(function () {
    if (!document.hidden) refresh();
  }, every);
})();
