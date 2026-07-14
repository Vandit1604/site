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
    el.style.setProperty("--reveal-delay", Math.min(idx, 8) * 40 + "ms");
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
    // Eager: reveal as soon as a sliver approaches the viewport (15% below the
    // fold counts), so content never sits invisible waiting to be scrolled to.
    { threshold: 0.01, rootMargin: "0px 0px 15% 0px" }
  );

  els.forEach(function (el) {
    io.observe(el);
  });

  // Failsafe: if nothing revealed shortly after load, the observer never fired
  // (a dead/headless renderer, a scripting quirk). Reveal everything so a
  // section can never ship blank. Real users get the hero revealed on first
  // paint, so this no-ops for them and scroll reveal stays fully intact.
  setTimeout(function () {
    if (!document.querySelector("[data-reveal].is-visible")) {
      els.forEach(function (el) {
        el.classList.add("is-visible");
      });
    }
  }, 1200);
})();

/* Scroll-aware floating nav dock: slide it out of the way while scrolling down
   (reading), bring it back on scroll up, and always show it near the very top
   and bottom of the page so it never permanently covers content. */
(function () {
  "use strict";
  var docks = document.querySelectorAll(".nav-dock");
  if (!docks.length) return;

  var lastY = window.scrollY || 0;
  var ticking = false;

  function setHidden(hide) {
    for (var i = 0; i < docks.length; i++) {
      docks[i].classList.toggle("nav-dock--hidden", hide);
    }
  }

  function onScroll() {
    var y = window.scrollY || 0;
    var doc = document.documentElement;
    var nearTop = y < 90;
    var nearBottom = window.innerHeight + y >= doc.scrollHeight - 60;
    var delta = y - lastY;

    if (nearTop || nearBottom) setHidden(false);
    else if (delta > 6) setHidden(true); // scrolling down
    else if (delta < -6) setHidden(false); // scrolling up

    lastY = y;
    ticking = false;
  }

  window.addEventListener(
    "scroll",
    function () {
      if (!ticking) {
        ticking = true;
        requestAnimationFrame(onScroll);
      }
    },
    { passive: true }
  );
})();
