// The dialog a visitor meets before a link that leaves this site, when the
// operator has asked for one with service_links.confirm.
//
// The markup does the hard parts. <dialog>.showModal() traps focus, closes on
// Escape, exposes aria-modal and returns focus to whatever was focused before,
// all from the browser. The continue button is an anchor with a real href, so
// following it needs no scripted navigation and no popup permission.
//
// Which links are guarded is decided on the server: this script only reacts to
// data-leave. Without it every one of them is an ordinary link and the visitor
// simply arrives, which is the no-JavaScript behaviour cairn keeps everywhere.
(function () {
  var dialog = document.getElementById("leave");
  if (!dialog || typeof dialog.showModal !== "function") return;

  var go = dialog.querySelector(".leave-go");
  var host = dialog.querySelector(".leave-host");
  var rest = dialog.querySelector(".leave-rest");
  var copy = dialog.querySelector(".leave-copy");
  var stay = dialog.querySelector(".leave-stay");
  var copyLabel = copy ? copy.textContent : "";

  // The origin and the rest are separated rather than printed as one string,
  // because the origin is the part that decides who you are talking to and a
  // long path is very good at pushing it out of sight.
  function split(href) {
    try {
      var u = new URL(href, location.href);
      return [u.origin, href.slice(href.indexOf(u.origin) + u.origin.length)];
    } catch (e) {
      return [href, ""];
    }
  }

  document.addEventListener("click", function (e) {
    var a = e.target.closest ? e.target.closest("a[data-leave]") : null;
    if (!a) return;
    // A modified click is the visitor already choosing where this opens, and
    // taking that over to show a dialog about tabs would be its own rudeness.
    if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0) return;
    e.preventDefault();

    var parts = split(a.href);
    host.textContent = parts[0];
    rest.textContent = parts[1];
    go.href = a.href;
    if (copy) copy.textContent = copyLabel;
    dialog.showModal();
  });

  if (copy) {
    copy.addEventListener("click", function () {
      var done = copy.getAttribute("data-done");
      // Only over https or on localhost. Where it is missing the address is
      // still on screen and still selectable, which is why nothing here hides
      // the button: it stays honest by simply not changing its label.
      if (!navigator.clipboard) return;
      navigator.clipboard.writeText(go.href).then(function () {
        copy.textContent = done;
      });
    });
  }

  if (stay) stay.addEventListener("click", function () { dialog.close(); });

  // The one thing showModal() does not give us. Chromium lets Tab walk off the
  // end of a modal dialog: the press after the last control leaves the page for
  // the browser's own toolbar, so nothing on screen moves and one press in four
  // reads as a dead key. A bare <dialog> with three buttons does the same, so
  // this is the platform and not the markup. WAI-ARIA's dialog pattern asks for
  // the wrap, and three controls are few enough that the visitor notices the
  // gap. Painted, not merely present: a control the layout dropped is not a
  // place to send the focus.
  dialog.addEventListener("keydown", function (e) {
    if (e.key !== "Tab") return;
    var stops = [];
    var all = dialog.querySelectorAll("a[href], button:not([disabled])");
    for (var i = 0; i < all.length; i++) {
      if (all[i].getClientRects().length) stops.push(all[i]);
    }
    if (!stops.length) return;
    var edge = e.shiftKey ? stops[0] : stops[stops.length - 1];
    if (document.activeElement !== edge) return;
    e.preventDefault();
    (e.shiftKey ? stops[stops.length - 1] : stops[0]).focus();
  });

  // A click on the backdrop lands on the dialog element itself, since its
  // children cover the rest of it.
  dialog.addEventListener("click", function (e) {
    if (e.target === dialog) dialog.close();
  });
})();
