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

    // Always POST and let the server decide whether this is a new visitor.
    // The old localStorage gate lived entirely on the client, so an incognito
    // window (or any cleared profile) presented itself as a first-ever visit
    // and bumped the tally again. The server dedupes per visitor instead.
    fetch("/api/views", { method: "POST" })
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

    // Pin a cat to a fixed viewport position (used by drag + restore).
    function placeFixed(cat, left, top) {
      cat.style.position = "fixed";
      cat.style.left = left + "px";
      cat.style.top = top + "px";
      cat.style.right = "auto";
      cat.style.bottom = "auto";
      cat.style.margin = "0";
    }

    cats.forEach(function (cat) {
      // Drag is session-only: a refresh returns the cat to its default anchor
      // (no persisted position).
      var dragging = false, moved = false, ox = 0, oy = 0;

      cat.addEventListener("pointerdown", function (e) {
        dragging = true;
        moved = false;
        var r = cat.getBoundingClientRect();
        ox = e.clientX - r.left;
        oy = e.clientY - r.top;
        try { cat.setPointerCapture(e.pointerId); } catch (_) {}
        cat.classList.add("is-dragging", "is-awake");
        cat.classList.remove("is-sleeping");
        e.preventDefault();
      });
      cat.addEventListener("pointermove", function (e) {
        if (!dragging) return;
        moved = true;
        placeFixed(cat, e.clientX - ox, e.clientY - oy);
      });
      function drop(e) {
        if (!dragging) return;
        dragging = false;
        cat.classList.remove("is-dragging");
        try { cat.releasePointerCapture(e.pointerId); } catch (_) {}
      }
      cat.addEventListener("pointerup", drop);
      cat.addEventListener("pointercancel", drop);

      // Hovering delights the cat; a real tap (no drag) replays the heart.
      cat.addEventListener("pointerenter", function () {
        if (dragging) return;
        cat.classList.remove("is-sleeping");
        cat.classList.add("is-awake", "is-happy");
      });
      cat.addEventListener("pointerleave", function () {
        cat.classList.remove("is-happy");
        updateCat(cat);
      });
      cat.addEventListener("click", function () {
        if (moved) { moved = false; return; } // that was a drag, not a tap
        cat.classList.remove("is-happy");
        void cat.offsetWidth; // reflow so the heart animation restarts
        cat.classList.add("is-happy");
      });
    });
  })();

  /* ---- Live GitHub activity ------------------------------------------- */
  (function github() {
    var section = document.querySelector("[data-github]");
    if (!section) return;

    function esc(s) {
      return String(s).replace(/[&<>"]/g, function (c) {
        return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c];
      });
    }

    fetch("/api/github")
      .then(function (r) { return r.json(); })
      .then(function (d) {
        if (!d) return;
        var hasDays = d.days && d.days.length;
        var hasRepos = d.repos && d.repos.length;
        if (!hasDays && !hasRepos) return; // nothing to show; stay hidden

        var totalEl = section.querySelector("[data-gh-total]");
        if (totalEl && typeof d.total === "number") {
          totalEl.textContent = d.total.toLocaleString("en-US");
        }

        var cal = section.querySelector("[data-gh-cal]");
        if (cal && hasDays) {
          var html = "";
          var first = new Date(d.days[0].date + "T00:00:00");
          for (var p = 0; p < first.getDay(); p++) html += '<i class="gh-day" data-empty></i>';
          for (var i = 0; i < d.days.length; i++) {
            var day = d.days[i];
            html += '<i class="gh-day" data-level="' + (day.level || 0) +
              '" title="' + day.count + ' on ' + day.date + '"></i>';
          }
          cal.innerHTML = html;
        }

        var wrap = section.querySelector("[data-gh-repos]");
        if (wrap && hasRepos) {
          var star = '<svg class="gh-star" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M12 2.6l2.9 6 6.5.9-4.7 4.6 1.1 6.5L12 18.5 6.2 21.6l1.1-6.5L2.6 9.5l6.5-.9z"/></svg>';
          wrap.innerHTML = d.repos.map(function (r) {
            return '<a href="' + esc(r.url) + '" target="_blank" rel="noopener noreferrer" class="gh-repo press">' +
              '<span class="gh-repo-top">' +
                '<span class="gh-repo-name">' + esc(r.name) + '</span>' +
                '<span class="gh-repo-stars">' + star + (r.stars || 0) + '</span>' +
              '</span>' +
              (r.description ? '<span class="gh-repo-desc">' + esc(r.description) + '</span>' : '<span class="gh-repo-desc gh-repo-desc--empty">No description</span>') +
              '<span class="gh-repo-foot">' +
                (r.language ? '<span class="gh-repo-lang"><i class="gh-lang-dot"></i>' + esc(r.language) + '</span>' : '<span></span>') +
                '<span class="gh-repo-arrow" aria-hidden="true">&#8599;</span>' +
              '</span>' +
              '</a>';
          }).join("");
        }

        section.removeAttribute("hidden");
      })
      .catch(function () { /* leave the section hidden on error */ });
  })();
})();
