// Scroll-reveal for [data-reveal] elements. Siblings under the same parent
// stagger slightly so groups (cards, list rows) cascade in like iOS.
(function () {
  "use strict";

  var els = document.querySelectorAll("[data-reveal]");
  if (!els.length) return;

  var reduce =
    window.matchMedia &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  // No IntersectionObserver or reduced motion: just show everything.
  if (reduce || !("IntersectionObserver" in window)) {
    els.forEach(function (el) {
      el.classList.add("is-visible");
    });
    return;
  }

  // Stagger by position among reveal-siblings (capped so long lists don't drag).
  els.forEach(function (el) {
    var parent = el.parentElement;
    var idx = 0;
    if (parent) {
      var sibs = Array.prototype.filter.call(parent.children, function (c) {
        return c.hasAttribute("data-reveal");
      });
      idx = sibs.indexOf(el);
    }
    el.style.setProperty("--reveal-delay", Math.min(idx, 6) * 70 + "ms");
  });

  var io = new IntersectionObserver(
    function (entries) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          entry.target.classList.add("is-visible");
          io.unobserve(entry.target);
        }
      });
    },
    { threshold: 0.12, rootMargin: "0px 0px -8% 0px" }
  );

  els.forEach(function (el) {
    io.observe(el);
  });
})();
