/* Command palette (⌘K / Ctrl+K / "/"). Fetches the site search index once,
   fuzzy-matches queries, and navigates on Enter. Rendered as a fixed overlay
   on <body> so no ancestor overflow can clip it. Keyboard-first, but the nav
   pill and mobile tap open it too. */
(function () {
  "use strict";

  var docs = null; // search index, lazily fetched
  var loading = null; // in-flight fetch promise
  var backdrop, input, results, items = [], selected = 0;
  var lastFocused = null;

  function loadIndex() {
    if (loading) return loading;
    loading = fetch("/api/search-index.json")
      .then(function (r) { return r.json(); })
      .then(function (data) { docs = data; return data; })
      .catch(function () { docs = []; return []; });
    return loading;
  }

  // Subsequence fuzzy score: every query char must appear in order. Contiguous
  // runs and word-start matches score higher; earlier matches win ties.
  function score(query, text) {
    var q = query.toLowerCase(), t = text.toLowerCase();
    if (!q) return 0;
    var qi = 0, ti = 0, s = 0, run = 0, prev = -2;
    while (qi < q.length && ti < t.length) {
      if (q[qi] === t[ti]) {
        run = ti === prev + 1 ? run + 1 : 1;
        s += run * 3;
        if (ti === 0 || /[\s\/\-—·]/.test(t[ti - 1])) s += 8; // word start
        prev = ti;
        qi++;
      }
      ti++;
    }
    if (qi < q.length) return -1; // not all chars matched
    return s - t.length * 0.05; // gently prefer shorter titles
  }

  function filter(query) {
    if (!docs) return [];
    if (!query) return docs.slice(0, 40);
    var scored = [];
    for (var i = 0; i < docs.length; i++) {
      var hay = docs[i].title + " " + (docs[i].desc || "") + " " + docs[i].section;
      var sc = score(query, hay);
      // weight title matches above desc/section matches
      var ts = score(query, docs[i].title);
      if (ts >= 0) sc += ts * 2;
      if (sc >= 0) scored.push({ d: docs[i], s: sc });
    }
    scored.sort(function (a, b) { return b.s - a.s; });
    return scored.slice(0, 40).map(function (x) { return x.d; });
  }

  function build() {
    backdrop = document.createElement("div");
    backdrop.className = "cmdk-backdrop";
    backdrop.setAttribute("role", "dialog");
    backdrop.setAttribute("aria-modal", "true");
    backdrop.setAttribute("aria-label", "Search and command palette");
    backdrop.innerHTML =
      '<div class="cmdk-panel">' +
      '  <div class="cmdk-search">' +
      '    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><circle cx="11" cy="11" r="7"/><path d="m21 21-4.3-4.3"/></svg>' +
      '    <input type="text" autocomplete="off" spellcheck="false" placeholder="Search pages, writing, projects, talks…" aria-label="Search" />' +
      '    <span class="cmdk-esc">ESC</span>' +
      "  </div>" +
      '  <div class="cmdk-results" role="listbox"></div>' +
      "</div>";
    document.body.appendChild(backdrop);
    input = backdrop.querySelector("input");
    results = backdrop.querySelector(".cmdk-results");

    backdrop.addEventListener("mousedown", function (e) {
      if (e.target === backdrop) close();
    });
    input.addEventListener("input", function () { render(filter(input.value.trim())); });
    input.addEventListener("keydown", onKeydown);
  }

  function render(list) {
    items = list;
    selected = 0;
    if (!list.length) {
      results.innerHTML = '<div class="cmdk-empty">No matches.</div>';
      return;
    }
    var html = "", lastSection = null, idx = 0;
    list.forEach(function (d) {
      if (d.section !== lastSection) {
        html += '<div class="cmdk-group-label">' + esc(d.section) + "</div>";
        lastSection = d.section;
      }
      html +=
        '<div class="cmdk-item" role="option" data-i="' + idx + '"' +
        (idx === 0 ? ' aria-selected="true"' : "") + ">" +
        '<span class="cmdk-item-title">' + esc(d.title) + "</span>" +
        (d.desc ? '<span class="cmdk-item-desc">' + esc(d.desc) + "</span>" : "") +
        (d.ext ? '<span class="cmdk-item-ext">↗</span>' : "") +
        "</div>";
      idx++;
    });
    results.innerHTML = html;
    Array.prototype.forEach.call(results.querySelectorAll(".cmdk-item"), function (el) {
      el.addEventListener("mousemove", function () { select(+el.dataset.i); });
      el.addEventListener("click", function () { go(items[+el.dataset.i]); });
    });
  }

  function select(i) {
    var nodes = results.querySelectorAll(".cmdk-item");
    if (!nodes.length) return;
    selected = (i + nodes.length) % nodes.length;
    nodes.forEach(function (el, j) {
      el.setAttribute("aria-selected", j === selected ? "true" : "false");
    });
    nodes[selected].scrollIntoView({ block: "nearest" });
  }

  function onKeydown(e) {
    if (e.key === "ArrowDown") { e.preventDefault(); select(selected + 1); }
    else if (e.key === "ArrowUp") { e.preventDefault(); select(selected - 1); }
    else if (e.key === "Enter") { e.preventDefault(); if (items[selected]) go(items[selected]); }
    else if (e.key === "Escape") { e.preventDefault(); close(); }
  }

  function go(d) {
    if (!d || !d.url) return;
    close();
    if (d.ext) window.open(d.url, "_blank", "noopener");
    else window.location.href = d.url;
  }

  function open() {
    if (!backdrop) build();
    lastFocused = document.activeElement;
    loadIndex().then(function () { render(filter(input.value.trim())); });
    render([]);
    // force reflow so the .open transition runs
    backdrop.style.display = "flex";
    requestAnimationFrame(function () { backdrop.classList.add("open"); });
    input.value = "";
    input.focus();
    document.documentElement.style.overflow = "hidden";
  }

  function close() {
    if (!backdrop) return;
    backdrop.classList.remove("open");
    document.documentElement.style.overflow = "";
    setTimeout(function () { backdrop.style.display = "none"; }, 180);
    if (lastFocused && lastFocused.focus) lastFocused.focus();
  }

  function esc(s) {
    return String(s).replace(/[&<>"]/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c];
    });
  }

  function isTyping(el) {
    return el && (el.tagName === "INPUT" || el.tagName === "TEXTAREA" || el.isContentEditable);
  }

  document.addEventListener("keydown", function (e) {
    var isOpen = backdrop && backdrop.classList.contains("open");
    if ((e.key === "k" || e.key === "K") && (e.metaKey || e.ctrlKey)) {
      e.preventDefault();
      isOpen ? close() : open();
    } else if (e.key === "/" && !isTyping(document.activeElement) && !isOpen) {
      e.preventDefault();
      open();
    }
  });

  // Nav pill / any [data-cmdk-open] trigger.
  document.addEventListener("click", function (e) {
    var t = e.target.closest("[data-cmdk-open]");
    if (t) { e.preventDefault(); open(); }
  });

  // Warm the index on idle so the first open is instant.
  if ("requestIdleCallback" in window) requestIdleCallback(loadIndex);
})();
