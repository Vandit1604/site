/* gallery-trail.js — cursor image-trail with synchronized ambient piano.
 *
 * On pointer-capable devices (and when motion is allowed), photos spawn along
 * the cursor's path and fade out, each triggering a soft pentatonic piano note
 * via Tone.js + the free Salamander piano samples. Falls back to the static
 * masonry grid on touch devices, reduced-motion, or if anything fails. */
(function () {
  "use strict";

  var photos = window.GALLERY_PHOTOS || [];
  var stage = document.getElementById("trail-stage");
  var grid = document.getElementById("photo-grid");
  var toggle = document.getElementById("audio-toggle");
  var hint = document.getElementById("gallery-hint");
  if (!stage || !grid || !photos.length) return;

  // ---- grid photo fade (runs for everyone who sees the grid) ---------------
  // Mark the grid as ours before wiring anything up: the opacity:0 start state
  // in gallery.css keys off this class, so it can only ever apply when this
  // script is alive to undo it.
  grid.classList.add("is-fading");
  Array.prototype.forEach.call(grid.querySelectorAll("img"), function (img) {
    if (img.complete && img.naturalWidth > 0) {
      img.classList.add("is-loaded"); // already cached: no fade, no flash
      return;
    }
    var reveal = function () {
      img.classList.add("is-loaded");
    };
    // `error` reveals too: a broken photo must never strand at opacity 0.
    img.addEventListener("load", reveal, { once: true });
    img.addEventListener("error", reveal, { once: true });
  });

  // Trail-vs-grid was decided before paint by the inline script in gallery.html
  // and is expressed by `trail-mode` on <html>. Read that decision rather than
  // making it again, so CSS and JS can't disagree about which one is showing.
  if (!document.documentElement.classList.contains("trail-mode")) return;

  // The stage is already visible via CSS; only the JS-dependent chrome is
  // revealed here, so it stays hidden if this script never runs.
  if (toggle) toggle.hidden = false;
  if (hint) hint.textContent = "Click and hold, then drag — your photos trail the cursor and the piano plays.";
  stage.style.cursor = "grab";

  // Shuffle (Fisher–Yates) so the trail order differs every visit.
  for (var i = photos.length - 1; i > 0; i--) {
    var j = Math.floor(Math.random() * (i + 1));
    var t = photos[i];
    photos[i] = photos[j];
    photos[j] = t;
  }

  // Warm the cache so the trail is smooth on first pass, without firing all
  // 24 photos (5MB) at once and starving the rest of the page. Only the first
  // few are needed to start; the tail arrives while the browser is idle.
  var EAGER = 4;
  function warm(src) {
    var img = new Image();
    img.src = src;
  }
  photos.slice(0, EAGER).forEach(warm);

  // The tail waits for the page to finish, then for the browser to go idle.
  // requestIdleCallback alone isn't enough: with nothing else in flight it
  // fires immediately and the whole 5MB lands on the load path anyway.
  var rest = photos.slice(EAGER);
  function warmRest() {
    var idle =
      window.requestIdleCallback ||
      function (fn) {
        return setTimeout(fn, 300);
      };
    idle(function () {
      rest.forEach(warm);
    });
  }
  if (document.readyState === "complete") warmRest();
  else window.addEventListener("load", warmRest, { once: true });

  // ---- visual trail --------------------------------------------------------
  var SPAWN_DISTANCE = 130; // px of travel between spawns (higher = less overlap)
  var lifetime = 2600; // ms each photo lives (higher = lingers longer, less fade)
  var lastX = null;
  var lastY = null;
  var photoIndex = 0;

  function spawn(x, y) {
    var img = document.createElement("img");
    img.src = photos[photoIndex % photos.length];
    img.alt = "";
    photoIndex++;

    img.style.position = "absolute";
    img.style.left = x + "px";
    img.style.top = y + "px";
    img.style.width = "clamp(140px, 16vw, 240px)";
    img.style.height = "auto";
    img.style.borderRadius = "0.6rem";
    img.style.boxShadow = "0 10px 30px rgba(0,0,0,0.18)";
    img.style.pointerEvents = "none";
    img.style.willChange = "transform, opacity";
    img.style.transformOrigin = "center center";
    stage.appendChild(img);

    // No rotation — keep each photo's natural orientation. Quick fade-in,
    // long hold at full opacity, gentle fade-out only at the very end.
    var base = "translate(-50%, -50%)";
    var anim = img.animate(
      [
        { opacity: 0, transform: base + " scale(0.92)" },
        { opacity: 1, transform: base + " scale(1)", offset: 0.08 },
        { opacity: 1, transform: base + " scale(1)", offset: 0.82 },
        { opacity: 0, transform: base + " scale(1)" },
      ],
      { duration: lifetime, easing: "cubic-bezier(0.16, 1, 0.3, 1)", fill: "forwards" }
    );
    anim.onfinish = function () {
      img.remove();
    };

    playNote();
  }

  // Photos (and piano notes) only appear while the pointer is held down, so
  // the visitor actively "paints" the trail — which also gives the gesture
  // the browser needs to start audio.
  var pressed = false;

  function pointerXY(e) {
    var rect = stage.getBoundingClientRect();
    return { x: e.clientX - rect.left, y: e.clientY - rect.top };
  }

  stage.addEventListener("pointerdown", function (e) {
    pressed = true;
    stage.style.cursor = "grabbing";
    if (stage.setPointerCapture) {
      try {
        stage.setPointerCapture(e.pointerId);
      } catch (err) {
        /* ignore */
      }
    }
    var p = pointerXY(e);
    spawn(p.x, p.y); // immediate feedback on press
    lastX = p.x;
    lastY = p.y;
  });

  stage.addEventListener("pointermove", function (e) {
    if (!pressed) return;
    var p = pointerXY(e);
    if (lastX === null) {
      lastX = p.x;
      lastY = p.y;
      return;
    }
    if (Math.hypot(p.x - lastX, p.y - lastY) >= SPAWN_DISTANCE) {
      spawn(p.x, p.y);
      lastX = p.x;
      lastY = p.y;
    }
  });

  function endPress() {
    pressed = false;
    lastX = null;
    lastY = null;
    stage.style.cursor = "grab";
  }
  stage.addEventListener("pointerup", endPress);
  stage.addEventListener("pointercancel", endPress);

  // ---- ambient piano (Tone.js) ---------------------------------------------
  // On by default. Browsers won't let an AudioContext make sound until the
  // user interacts, so it actually begins on the first click/tap anywhere.
  var audioOn = true;
  var audioStarted = false;
  var sampler = null;
  var samplerReady = false;

  // Pentatonic: there are no wrong notes, so any order of these is consonant.
  // Four keys rather than one, because the relative minor of a pentatonic is
  // the same five pitches — only a genuine transposition changes the colour.
  // Every note sits at C4 or above: the lowest Salamander sample is C4, and
  // anything under it gets pitch-shifted down into mud.
  var SCALES = [
    ["C4", "D4", "E4", "G4", "A4", "C5", "D5", "E5", "G5", "A5"], // C — open, bright
    ["D4", "E4", "F#4", "A4", "B4", "D5", "E5", "F#5", "A5", "B5"], // D — airy
    ["F4", "G4", "A4", "C5", "D5", "F5", "G5", "A5"], // F — warm
    ["G4", "A4", "B4", "D5", "E5", "G5", "A5", "B5"], // G — folk
  ];
  var scale = SCALES[Math.floor(Math.random() * SCALES.length)];
  var noteIndex = Math.floor(Math.random() * scale.length);
  var lastNote = 0;

  // The melody used to be `scale[i++]`: a strict ascending run that hit the top
  // and snapped back down an octave, identical on every drag. That's a scale
  // exercise, not a tune. Walk it instead — mostly neighbours with the odd
  // leap, which is roughly how a melody actually moves — and reflect off the
  // ends rather than wrapping, since the wrap was the octave snap.
  function nextNote() {
    var r = Math.random();
    var dir = Math.random() < 0.5 ? 1 : -1;
    var step = r < 0.62 ? 1 : r < 0.88 ? 2 : 3;
    var next = noteIndex + dir * step;
    var top = scale.length - 1;
    if (next < 0) next = -next;
    if (next > top) next = 2 * top - next;
    noteIndex = Math.max(0, Math.min(top, next));
    return scale[noteIndex];
  }

  // Tone.js is fetched on demand, not at page load. It's ~200KB from a CDN and
  // browsers won't let it make a sound before a user gesture, so there is no
  // version of "load it early" that helps the visitor.
  var tonePromise = null;
  function loadTone() {
    if (tonePromise) return tonePromise;
    tonePromise = new Promise(function (resolve, reject) {
      if (typeof Tone !== "undefined") return resolve();
      var s = document.createElement("script");
      s.src = "https://unpkg.com/tone@14.8.49/build/Tone.js";
      s.async = true;
      s.onload = function () {
        resolve();
      };
      s.onerror = reject;
      document.head.appendChild(s);
    });
    return tonePromise;
  }

  function buildSampler() {
    if (sampler || typeof Tone === "undefined") return;
    var reverb = new Tone.Reverb({ decay: 4, wet: 0.35 }).toDestination();
    sampler = new Tone.Sampler({
      urls: {
        C4: "C4.mp3",
        "D#4": "Ds4.mp3",
        "F#4": "Fs4.mp3",
        A4: "A4.mp3",
        C5: "C5.mp3",
        "D#5": "Ds5.mp3",
        "F#5": "Fs5.mp3",
        A5: "A5.mp3",
      },
      baseUrl: "https://tonejs.github.io/audio/salamander/",
      release: 1.4,
      onload: function () {
        samplerReady = true;
      },
    });
    sampler.volume.value = -3;
    sampler.connect(reverb);
  }

  function playNote() {
    if (!audioOn || !samplerReady || !sampler) return;
    var now = (window.performance && performance.now()) || 0;
    if (now - lastNote < 90) return; // keep it from turning to mush
    lastNote = now;
    try {
      sampler.triggerAttackRelease(nextNote(), "8n", undefined, 0.55 + Math.random() * 0.2);
    } catch (err) {
      /* ignore transient timing errors */
    }
  }

  function setToggleState() {
    var icon = document.getElementById("audio-icon");
    var label = document.getElementById("audio-label");
    toggle.setAttribute("aria-pressed", audioOn ? "true" : "false");
    if (icon) icon.textContent = audioOn ? "♫" : "♪";
    if (label) label.textContent = audioOn ? "Mute piano" : "Play piano";
  }

  // Start the AudioContext on the first real user gesture (required by
  // browsers). Since audio is on by default, this kicks the piano to life
  // the moment the visitor first clicks/taps anywhere on the page.
  function startAudio() {
    if (audioStarted) return;
    audioStarted = true;
    loadTone()
      .then(function () {
        return Tone.start();
      })
      .then(buildSampler)
      .catch(function () {
        // The piano is a bonus, not the feature. If the CDN is blocked or slow
        // the trail carries on in silence.
        audioStarted = false;
      });
  }
  document.addEventListener(
    "pointerdown",
    function () {
      if (audioOn) startAudio();
    },
    { once: true }
  );

  if (toggle) {
    setToggleState(); // reflect the default-on state
    toggle.addEventListener("click", function () {
      audioOn = !audioOn;
      if (audioOn) startAudio(); // loads Tone.js on demand if this is the first ask
      setToggleState();
    });
  }
})();
