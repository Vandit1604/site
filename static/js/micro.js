/* Microinteraction layer: the persistent view counter, the hero typewriter
   role-line, and the hero name's word-stagger entrance. All three degrade to a
   clean static state under prefers-reduced-motion. */
(function () {
  "use strict";

  var reduce =
    window.matchMedia &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  /* ---- Persistent view counter -------------------------------------- */
  (function views() {
    var targets = document.querySelectorAll("[data-views]");
    if (!targets.length) return;

    // Count each browser once: POST (increment) on a first-ever visit, GET
    // (read-only) on return. Keeps the tally unique-per-visitor, not per load.
    var seen = false;
    try { seen = localStorage.getItem("vs_seen") === "1"; } catch (e) {}
    if (!seen) { try { localStorage.setItem("vs_seen", "1"); } catch (e) {} }

    fetch("/api/views", seen ? undefined : { method: "POST" })
      .then(function (r) { return r.json(); })
      .then(function (data) {
        var n = typeof data.count === "number" ? data.count : 0;
        targets.forEach(function (el) { countUp(el, n); });
      })
      .catch(function () {
        targets.forEach(function (el) { el.textContent = "—"; });
      });
  })();

  function countUp(el, target) {
    var fmt = function (v) { return v.toLocaleString("en-US"); };
    if (reduce || target <= 0) { el.textContent = fmt(target); return; }
    // Start near the target so large counts don't spin from zero for seconds.
    var start = Math.max(0, target - Math.min(target, 120));
    var dur = 900, t0 = null;
    function frame(ts) {
      if (t0 === null) t0 = ts;
      var p = Math.min(1, (ts - t0) / dur);
      var eased = 1 - Math.pow(1 - p, 3); // ease-out cubic
      el.textContent = fmt(Math.round(start + (target - start) * eased));
      if (p < 1) requestAnimationFrame(frame);
    }
    requestAnimationFrame(frame);
  }

  /* ---- Hero name word-stagger --------------------------------------- */
  (function heroName() {
    var h = document.querySelector("[data-hero-name]");
    if (!h || reduce) return;
    var words = h.textContent.trim().split(/\s+/);
    h.textContent = "";
    h.classList.add("hero-anim");
    words.forEach(function (w, i) {
      var span = document.createElement("span");
      span.className = "hero-word";
      span.style.setProperty("--i", i);
      span.textContent = w;
      h.appendChild(span);
      if (i < words.length - 1) h.appendChild(document.createTextNode(" "));
    });
  })();

  /* ---- Typewriter role line ----------------------------------------- */
  (function typewriter() {
    var el = document.querySelector("[data-typewriter]");
    if (!el) return;

    var phrases = [
      "building distributed systems",
      "storage & p2p over libp2p",
      "observability with Prometheus",
      "merged PRs to Kubernetes",
    ];

    // Reduced motion: hold the static first phrase, no cycling.
    if (reduce) { el.textContent = phrases[0]; return; }

    var pi = 0, ci = phrases[0].length, deleting = false;
    el.textContent = phrases[0];

    function tick() {
      var word = phrases[pi];
      if (!deleting) {
        ci++;
        el.textContent = word.slice(0, ci);
        if (ci >= word.length) { deleting = true; return schedule(1900); }
      } else {
        ci--;
        el.textContent = word.slice(0, ci);
        if (ci <= 0) { deleting = false; pi = (pi + 1) % phrases.length; return schedule(320); }
      }
      schedule(deleting ? 34 : 62);
    }
    function schedule(ms) { setTimeout(tick, ms); }

    // Let the hero entrance play first, then start typing.
    schedule(2200);
  })();

  /* ---- Cursor: reactive gutter dots + the footer cat -------------------- */
  (function pointer() {
    // Skip touch devices (no hovering cursor, no hover states).
    if (window.matchMedia && window.matchMedia("(pointer: coarse)").matches) return;
    var root = document.documentElement;
    var cats = Array.prototype.slice.call(document.querySelectorAll(".cat"));
    var WAKE = 240; // px: how close the cursor must get before a cat wakes
    var x = 0, y = 0, queued = false;

    function clamp(v) { return v < -1 ? -1 : v > 1 ? 1 : v; }

    function updateCat(cat) {
      var r = cat.getBoundingClientRect();
      if (!r.width) return; // hidden (mobile)
      var dx = x - (r.left + r.width / 2);
      var dy = y - (r.top + r.height / 2);
      // Eyes follow the cursor (offset fed to .cat__pupils in grid.css).
      cat.style.setProperty("--px", clamp(dx / 60).toFixed(2));
      cat.style.setProperty("--py", clamp(dy / 45).toFixed(2));
      if (Math.sqrt(dx * dx + dy * dy) < WAKE) {
        cat.classList.remove("is-sleeping");
        if (!cat.classList.contains("is-happy")) cat.classList.add("is-awake");
      } else {
        cat.classList.add("is-sleeping");
        cat.classList.remove("is-awake");
      }
    }

    window.addEventListener(
      "pointermove",
      function (e) {
        x = e.clientX;
        y = e.clientY;
        if (queued) return;
        queued = true;
        requestAnimationFrame(function () {
          root.style.setProperty("--mx", x);
          root.style.setProperty("--my", y);
          for (var i = 0; i < cats.length; i++) updateCat(cats[i]);
          queued = false;
        });
      },
      { passive: true }
    );

    // Hovering a cat delights it; clicking replays the happy burst.
    cats.forEach(function (cat) {
      cat.addEventListener("pointerenter", function () {
        cat.classList.remove("is-sleeping");
        cat.classList.add("is-awake", "is-happy");
      });
      cat.addEventListener("pointerleave", function () {
        cat.classList.remove("is-happy");
        updateCat(cat);
      });
      cat.addEventListener("click", function () {
        cat.classList.remove("is-happy");
        void cat.offsetWidth; // reflow so the heart animation restarts
        cat.classList.add("is-happy");
      });
    });
  })();
})();
