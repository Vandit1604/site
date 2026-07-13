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

  // Progressive enhancement: only take over for fine-pointer, motion-ok devices.
  var canHover = window.matchMedia("(hover: hover) and (pointer: fine)").matches;
  var reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  if (!canHover || reduced) return;

  // Activate trail mode: hide the grid, reveal the stage + audio toggle.
  grid.hidden = true;
  stage.hidden = false;
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

  // Warm the browser cache so the trail is smooth on first pass.
  photos.forEach(function (src) {
    var img = new Image();
    img.src = src;
  });

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
  // C major pentatonic — no "wrong" notes, always pleasant.
  var scale = ["C4", "D4", "E4", "G4", "A4", "C5", "D5", "E5", "G5", "A5"];
  var noteIndex = 0;
  var lastNote = 0;

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
    var note = scale[noteIndex % scale.length];
    noteIndex++;
    try {
      sampler.triggerAttackRelease(note, "8n", undefined, 0.55 + Math.random() * 0.2);
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
    if (audioStarted || typeof Tone === "undefined") return;
    audioStarted = true;
    Tone.start().then(buildSampler);
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
      if (typeof Tone === "undefined") return;
      audioOn = !audioOn;
      if (audioOn) startAudio();
      setToggleState();
    });
  }
})();
