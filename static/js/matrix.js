/* Binary side columns: fill the gutters beside the frame with rows of
   0101… (per-digit spans), then randomly flash individual digits brighter
   blue — a matrix-style twinkle. Desktop-only. */
(function () {
  var gutters = [
    document.querySelector(".binary-gutter--left"),
    document.querySelector(".binary-gutter--right"),
  ].filter(Boolean);
  if (!gutters.length) return;

  var reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  var spans = [];

  function build() {
    spans = [];
    if (window.innerWidth <= 960) {
      gutters.forEach(function (g) {
        g.innerHTML = "";
      });
      return;
    }
    var gutter = (window.innerWidth - 896) / 2 - 24; // 56rem column, ~24px pad
    if (gutter <= 0) return;
    var charW = 10; // ~13px mono + letter-spacing
    var lineH = 21;
    var perLine = Math.ceil(gutter / charW);
    var lines = Math.ceil(window.innerHeight / lineH) + 2;
    var total = perLine * lines;
    gutters.forEach(function (g) {
      var html = "";
      for (var i = 0; i < total; i++) html += "<span>" + (i % 2) + "</span>";
      g.innerHTML = html;
      for (var j = 0; j < g.children.length; j++) spans.push(g.children[j]);
    });
  }

  build();
  var t;
  window.addEventListener("resize", function () {
    clearTimeout(t);
    t = setTimeout(build, 150);
  });

  if (reduce) return; // keep the columns static for reduced-motion

  // Continuously light up a few random digits; each clears itself when its
  // flash finishes so it can be picked again.
  function flash() {
    if (spans.length) {
      // Light up ~2.5% of digits per tick; with the slow 2.8s fade and the
      // slower tick below, lit digits accumulate into a dense, calm field.
      var n = Math.max(12, Math.round(spans.length * 0.025));
      for (var i = 0; i < n; i++) {
        var s = spans[(Math.random() * spans.length) | 0];
        if (s && !s.classList.contains("lit")) {
          s.classList.add("lit");
          (function (el) {
            el.addEventListener(
              "animationend",
              function () {
                el.classList.remove("lit");
              },
              { once: true }
            );
          })(s);
        }
      }
    }
    setTimeout(flash, 260);
  }
  flash();
})();
