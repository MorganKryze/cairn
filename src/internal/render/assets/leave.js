// Guards the links the server marked with data-leave. Without this script they
// are ordinary links and the visitor simply arrives.
(function () {
  var dialog = document.getElementById("leave");
  if (!dialog || typeof dialog.showModal !== "function") return;

  var go = dialog.querySelector(".leave-go");
  var host = dialog.querySelector(".leave-host");
  var rest = dialog.querySelector(".leave-rest");
  var copy = dialog.querySelector(".leave-copy");
  var stay = dialog.querySelector(".leave-stay");
  var copyLabel = copy ? copy.textContent : "";

  // The origin is what says who you are about to talk to, and a long path is
  // very good at pushing it out of sight.
  function splitOrigin(href) {
    try {
      var u = new URL(href, location.href);
      return [u.origin, href.slice(href.indexOf(u.origin) + u.origin.length)];
    } catch (e) {
      return [href, ""];
    }
  }

  function isPlainClick(e) {
    return !(
      e.defaultPrevented ||
      e.metaKey ||
      e.ctrlKey ||
      e.shiftKey ||
      e.altKey ||
      e.button !== 0
    );
  }

  document.addEventListener("click", function (e) {
    var a = e.target.closest ? e.target.closest("a[data-leave]") : null;
    // A modified click is the visitor already choosing where this opens.
    if (!a || !isPlainClick(e)) return;
    e.preventDefault();

    var parts = splitOrigin(a.href);
    host.textContent = parts[0];
    rest.textContent = parts[1];
    go.href = a.href;
    if (copy) copy.textContent = copyLabel;
    dialog.showModal();
  });

  if (copy) {
    copy.addEventListener("click", function () {
      var done = copy.getAttribute("data-done");
      // navigator.clipboard is https and localhost only. Where it is missing
      // the button stays put and its label stays unchanged: the address is on
      // screen and selectable either way.
      if (!navigator.clipboard) return;
      navigator.clipboard.writeText(go.href).then(function () {
        copy.textContent = done;
      });
    });
  }

  if (stay)
    stay.addEventListener("click", function () {
      dialog.close();
    });

  // Chromium lets Tab off the end of a modal dialog, into the browser's own
  // toolbar, so one press in four moves nothing on screen. A bare <dialog>
  // does the same. WAI-ARIA's dialog pattern asks for the wrap.
  dialog.addEventListener("keydown", function (e) {
    if (e.key !== "Tab") return;
    var painted = [];
    var all = dialog.querySelectorAll("a[href], button:not([disabled])");
    for (var i = 0; i < all.length; i++) {
      // a control the layout dropped is not a place to send the focus
      if (all[i].getClientRects().length) painted.push(all[i]);
    }
    if (!painted.length) return;
    var last = painted[painted.length - 1];
    var edge = e.shiftKey ? painted[0] : last;
    if (document.activeElement !== edge) return;
    e.preventDefault();
    (e.shiftKey ? last : painted[0]).focus();
  });

  // The backdrop is the dialog element itself: its children cover the rest.
  dialog.addEventListener("click", function (e) {
    if (e.target === dialog) dialog.close();
  });
})();
