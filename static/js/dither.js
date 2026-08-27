/* Animated dithered background for the hero (ReactBits "Dither" in spirit):
   flowing FBM noise waves, ordered (Bayer 8x8) dithering, chunky pixels,
   monochrome and theme-aware. Self-contained WebGL1, no dependencies. Degrades
   to a transparent canvas where WebGL is unavailable, and holds a single static
   frame under prefers-reduced-motion. */
(function () {
  "use strict";
  var canvases = document.querySelectorAll("[data-dither]");
  if (!canvases.length) return;
  Array.prototype.forEach.call(canvases, initDither);

function initDither(canvas) {
  var gl =
    canvas.getContext("webgl", { alpha: true, antialias: false, premultipliedAlpha: false }) ||
    canvas.getContext("experimental-webgl");
  if (!gl) return;

  var reduce =
    window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  var VERT = "attribute vec2 aPos; void main(){ gl_Position = vec4(aPos,0.0,1.0); }";
  var FRAG =
    "precision highp float;" +
    "uniform vec2 uRes; uniform float uTime; uniform vec3 uInk; uniform float uStrength; uniform float uPixel; uniform sampler2D uBayer;" +
    "float hash(vec2 p){ p=fract(p*vec2(123.34,456.21)); p+=dot(p,p+45.32); return fract(p.x*p.y); }" +
    "float vnoise(vec2 p){ vec2 i=floor(p),f=fract(p); float a=hash(i),b=hash(i+vec2(1.,0.)),c=hash(i+vec2(0.,1.)),d=hash(i+vec2(1.,1.)); vec2 u=f*f*(3.-2.*f); return mix(mix(a,b,u.x),mix(c,d,u.x),u.y); }" +
    "float fbm(vec2 p){ float v=0.,a=.5; for(int i=0;i<5;i++){ v+=a*vnoise(p); p*=2.02; a*=.5; } return v; }" +
    "void main(){" +
    "  vec2 block=floor(gl_FragCoord.xy/uPixel);" +
    "  vec2 uv=(block*uPixel)/uRes;" +
    "  float asp=uRes.x/max(uRes.y,1.0);" +
    "  vec2 p=vec2(uv.x*asp, uv.y)*3.0;" +
    "  float t=uTime*0.12;" +
    "  float w=fbm(p+vec2(t,t*0.5));" +
    "  w=fbm(p+1.6*vec2(w)+vec2(0.0,t));" +
    "  float lum=smoothstep(0.15,0.95,w);" +
    "  float th=texture2D(uBayer, block/8.0).r;" +
    "  float on=step(th, lum);" +
    "  gl_FragColor=vec4(uInk, on*uStrength);" +
    "}";

  function compile(type, src) {
    var s = gl.createShader(type);
    gl.shaderSource(s, src);
    gl.compileShader(s);
    if (!gl.getShaderParameter(s, gl.COMPILE_STATUS)) return null;
    return s;
  }
  var vs = compile(gl.VERTEX_SHADER, VERT);
  var fs = compile(gl.FRAGMENT_SHADER, FRAG);
  if (!vs || !fs) return;
  var prog = gl.createProgram();
  gl.attachShader(prog, vs);
  gl.attachShader(prog, fs);
  gl.linkProgram(prog);
  if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) return;
  gl.useProgram(prog);

  // Fullscreen triangle.
  var buf = gl.createBuffer();
  gl.bindBuffer(gl.ARRAY_BUFFER, buf);
  gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW);
  var aPos = gl.getAttribLocation(prog, "aPos");
  gl.enableVertexAttribArray(aPos);
  gl.vertexAttribPointer(aPos, 2, gl.FLOAT, false, 0, 0);

  // 8x8 Bayer threshold matrix as a LUMINANCE texture (POT, so REPEAT is legal).
  var BAYER = [
    0, 32, 8, 40, 2, 34, 10, 42, 48, 16, 56, 24, 50, 18, 58, 26,
    12, 44, 4, 36, 14, 46, 6, 38, 60, 28, 52, 20, 62, 30, 54, 22,
    3, 35, 11, 43, 1, 33, 9, 41, 51, 19, 59, 27, 49, 17, 57, 25,
    15, 47, 7, 39, 13, 45, 5, 37, 63, 31, 55, 23, 61, 29, 53, 21,
  ];
  var px = new Uint8Array(64);
  for (var i = 0; i < 64; i++) px[i] = Math.round(((BAYER[i] + 0.5) / 64) * 255);
  var tex = gl.createTexture();
  gl.bindTexture(gl.TEXTURE_2D, tex);
  gl.texImage2D(gl.TEXTURE_2D, 0, gl.LUMINANCE, 8, 8, 0, gl.LUMINANCE, gl.UNSIGNED_BYTE, px);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);

  var uRes = gl.getUniformLocation(prog, "uRes");
  var uTime = gl.getUniformLocation(prog, "uTime");
  var uInk = gl.getUniformLocation(prog, "uInk");
  var uStrength = gl.getUniformLocation(prog, "uStrength");
  var uPixel = gl.getUniformLocation(prog, "uPixel");
  gl.uniform1i(gl.getUniformLocation(prog, "uBayer"), 0);
  gl.uniform1f(uPixel, 3.0);
  gl.uniform1f(uStrength, 0.5);

  function setInk() {
    var dark = document.documentElement.classList.contains("dark");
    if (dark) gl.uniform3f(uInk, 0.92, 0.94, 1.0);
    else gl.uniform3f(uInk, 0.05, 0.05, 0.08);
  }
  setInk();
  new MutationObserver(function () {
    setInk();
    if (reduce) draw(0);
  }).observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });

  function resize() {
    var w = Math.max(1, canvas.clientWidth);
    var h = Math.max(1, canvas.clientHeight);
    if (canvas.width !== w || canvas.height !== h) {
      canvas.width = w;
      canvas.height = h;
    }
    gl.viewport(0, 0, canvas.width, canvas.height);
    gl.uniform2f(uRes, canvas.width, canvas.height);
  }

  gl.enable(gl.BLEND);
  gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);

  function draw(t) {
    gl.uniform1f(uTime, t);
    gl.clearColor(0, 0, 0, 0);
    gl.clear(gl.COLOR_BUFFER_BIT);
    gl.drawArrays(gl.TRIANGLES, 0, 3);
  }

  var start = null, raf = 0;
  function frame(now) {
    // Skip work while hidden (e.g. the intro cover after it is dismissed).
    if (!canvas.clientWidth || !canvas.clientHeight) {
      raf = requestAnimationFrame(frame);
      return;
    }
    if (start === null) start = now;
    resize();
    draw((now - start) / 1000);
    raf = requestAnimationFrame(frame);
  }

  window.addEventListener("resize", function () {
    resize();
    if (reduce) draw(0);
  });

  resize();
  if (reduce) {
    draw(0);
  } else {
    raf = requestAnimationFrame(frame);
    document.addEventListener("visibilitychange", function () {
      if (document.hidden) {
        cancelAnimationFrame(raf);
        raf = 0;
      } else if (!raf) {
        start = null;
        raf = requestAnimationFrame(frame);
      }
    });
  }
}
})();
