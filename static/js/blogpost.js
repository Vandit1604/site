// Blog reading enhancements: a top scroll-progress bar, and a table of contents
// built from the post's own headings (shown in the left gutter on wide screens,
// with the current section highlighted as you scroll). Pure vanilla, no deps.
(function () {
  "use strict";

  var content = document.querySelector(".blog-content");
  if (!content) return;

  var article = document.querySelector("article");
  var bar = document.querySelector("[data-blog-progress]");

  // ---- Reading progress ----
  function updateProgress() {
    if (!bar || !article) return;
    var rect = article.getBoundingClientRect();
    var total = article.offsetHeight - window.innerHeight;
    var pct = total > 0 ? (-rect.top / total) * 100 : 0;
    bar.style.width = Math.min(100, Math.max(0, pct)) + "%";
  }

  // ---- Table of contents ----
  var nav = document.querySelector("[data-blog-toc]");
  var list = document.querySelector("[data-blog-toc-list]");
  var headings = Array.prototype.slice
    .call(content.querySelectorAll("h2, h3"))
    .filter(function (h) { return h.id; });
  var linkFor = {};

  // Only worth a TOC on longer posts.
  if (nav && list && headings.length >= 3) {
    headings.forEach(function (h) {
      var li = document.createElement("li");
      li.className =
        "blog-toc__item" + (h.tagName === "H3" ? " blog-toc__item--sub" : "");
      var a = document.createElement("a");
      a.href = "#" + h.id;
      a.textContent = h.textContent;
      a.className = "blog-toc__link";
      li.appendChild(a);
      list.appendChild(li);
      linkFor[h.id] = a;
    });
    nav.hidden = false;
    nav.classList.add("is-visible");

    // Highlight the section currently near the top of the viewport.
    var observer = new IntersectionObserver(
      function (entries) {
        entries.forEach(function (entry) {
          var a = linkFor[entry.target.id];
          if (!a) return;
          if (entry.isIntersecting) {
            Object.keys(linkFor).forEach(function (id) {
              linkFor[id].classList.remove("is-active");
            });
            a.classList.add("is-active");
          }
        });
      },
      { rootMargin: "0px 0px -75% 0px", threshold: 0 }
    );
    headings.forEach(function (h) { observer.observe(h); });
  }

  // ---- Scroll wiring (rAF-throttled) ----
  var ticking = false;
  window.addEventListener(
    "scroll",
    function () {
      if (!ticking) {
        window.requestAnimationFrame(function () {
          updateProgress();
          ticking = false;
        });
        ticking = true;
      }
    },
    { passive: true }
  );
  window.addEventListener("resize", updateProgress);
  updateProgress();
})();
