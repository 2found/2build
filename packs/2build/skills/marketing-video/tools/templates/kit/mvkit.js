/* mvkit — the shot engine behind `mvf scaffold`'s generated scenes.
 *
 * A generated scene (compositions/<beat>.html, data-mvf="generated") carries one resolved
 * spec: every time is already a number in the scene's local seconds (mvf resolved the word
 * anchors from the measured take), every camera move is already clamped inside the plate.
 * This file only plays it on the scene's paused timeline, so the scene stays a pure function
 * of time (HyperFrames contract: no clocks, no randomness that is not seeded, no repeat: -1).
 *
 * Layers per shot: #<shot> (visibility, transitions) > .mvk-kbo (keys: punch-ins on words)
 * > .mvk-kb (the drift over the whole shot) > img/video, and beside .mvk-kb any 2.5D layers (see layer()). Two camera layers so no two tweens
 * ever write the same property: a drift left running under a punch-in would write its scale
 * every frame after the punch-in ends and snap it back in the render only.
 */
(function () {
  "use strict";

  // Seeded PRNG: weather and grain must render the same frame every time.
  function rng(seed) {
    let a = (seed >>> 0) || 1;
    return function () {
      a = (a + 0x6d2b79f5) >>> 0;
      let t = a;
      t = Math.imul(t ^ (t >>> 15), t | 1);
      t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
  }

  const $ = (id) => "#" + id;
  // a point k of the way from a to b, numeric keys only (k < 0 leans back before a move: anticipation)
  const lerp = (a, b, k) => Object.fromEntries(Object.keys(b).map((key) =>
    [key, typeof a[key] === "number" && typeof b[key] === "number" ? a[key] + (b[key] - a[key]) * k : b[key]]));
  // GSAP shifts every existing child later when one is placed at a negative time: never let that happen.
  const at = (t) => Math.max(0, t);

  function cam(c, W, H) {
    return { scale: c.s, x: (c.x || 0) * W, y: (c.y || 0) * H };
  }

  // ---------- transitions into a shot (and the matching exit of the one before) ----------
  // Visibility is not animated here: every shot is a timed clip (data-start / data-duration on its div), so
  // the HyperFrames runtime shows it over exactly its window. Zero-duration opacity sets at t=0 do not survive
  // the renderer's seek order (frame 0 came out black). A transition only animates inside the window; its
  // start state is inline in the HTML (the resolver writes it), so the first frame of the window is right too.
  const IN = {
    cut() {},
    flash(tl, s, sp) {
      tl.fromTo($(sp.id + "-flash"), { opacity: s.in.peak ?? 0.85 }, { opacity: 0, duration: s.in.dur ?? 0.28, ease: "power2.out", immediateRender: false }, at(s.start));
    },
    dip(tl, s, sp) {
      tl.fromTo($(sp.id + "-black"), { opacity: 0.55 }, { opacity: 0, duration: s.in.dur ?? 0.18, ease: "power1.out", immediateRender: false }, at(s.start));
    },
    whip(tl, s) {
      const dir = s.in.dir === "right" ? -1 : 1;
      tl.fromTo($(s.id) + " .mvk-kbo", { xPercent: 22 * dir, filter: "blur(22px)" },
        { xPercent: 0, filter: "blur(0px)", duration: s.in.dur ?? 0.26, ease: "power3.out", immediateRender: false }, at(s.start));
    },
    zoom(tl, s) {
      tl.fromTo($(s.id) + " .mvk-kbo", { scale: 1.32, filter: "blur(14px)" },
        { scale: 1, filter: "blur(0px)", duration: s.in.dur ?? 0.32, ease: "power3.out", immediateRender: false }, at(s.start));
    },
    fade(tl, s) {
      const d = s.in.dur ?? 0.4;
      tl.fromTo($(s.id), { opacity: 0 }, { opacity: 1, duration: d, ease: "power1.inOut", immediateRender: false }, at(s.start - d));
    },
    blur(tl, s) {
      const d = s.in.dur ?? 0.3;
      tl.fromTo($(s.id), { opacity: 0, filter: "blur(18px)" }, { opacity: 1, filter: "blur(0px)", duration: d, ease: "power2.out", immediateRender: false }, at(s.start - d * 0.5));
    },
    bloom(tl, s) {
      // a soft round reveal growing from the focus point: ink spreading in water
      const d = s.in.dur ?? 0.6;
      tl.fromTo($(s.id), { "--mvk-r": "0%" }, { "--mvk-r": "160%", duration: d, ease: "power2.inOut", immediateRender: false }, at(s.start - d));
    },
    wipe(tl, s) {
      const d = s.in.dur ?? 0.45;
      tl.fromTo($(s.id), { "--mvk-w": "-30%" }, { "--mvk-w": "130%", duration: d, ease: "power2.inOut", immediateRender: false }, at(s.start - d));
    },
    iris(tl, s) {
      const d = s.in.dur ?? 0.32, o = s.origin || [0.5, 0.45];
      const pos = `${o[0] * 100}% ${o[1] * 100}%`;
      tl.fromTo($(s.id), { clipPath: `circle(0% at ${pos})` }, { clipPath: `circle(150% at ${pos})`, duration: d, ease: "power3.in", immediateRender: false }, at(s.start));
    },
    push(tl, s) {
      tl.fromTo($(s.id) + " .mvk-kbo", { yPercent: 100 }, { yPercent: 0, duration: s.in.dur ?? 0.34, ease: "power3.inOut", immediateRender: false }, at(s.start));
    },
  };

  // Exits are placed by the resolver (s.out.at, s.out.dur): a whip/zoom out ends on the next shot's start,
  // a push out runs with the next shot's push in.
  const OUT = {
    whip(tl, s) {
      const dir = s.out.dir === "right" ? -1 : 1;
      tl.fromTo($(s.id) + " .mvk-kbo", { xPercent: 0, filter: "blur(0px)" },
        { xPercent: -22 * dir, filter: "blur(22px)", duration: s.out.dur, ease: "power3.in", immediateRender: false }, at(s.out.at));
    },
    zoom(tl, s) {
      const s0 = s.out.s0 ?? 1;  // the scale the last key left
      tl.fromTo($(s.id) + " .mvk-kbo", { scale: s0 }, { scale: s0 * 1.25, duration: s.out.dur, ease: "power3.in", immediateRender: false }, at(s.out.at));
    },
    push(tl, s) {
      tl.fromTo($(s.id) + " .mvk-kbo", { yPercent: 0 }, { yPercent: -35, duration: s.out.dur, ease: "power3.inOut", immediateRender: false }, at(s.out.at));
    },
  };

  // ---------- effects ----------
  const FX = {
    slam(tl, f) {
      tl.fromTo($(f.el), { opacity: 0, scale: f.from ?? 1.7 }, { opacity: 1, scale: 1, duration: 0.16, ease: "power4.out", immediateRender: false }, at(f.at));
      tl.to($(f.el), { opacity: 0, scale: 0.97, duration: 0.2, ease: "power1.in" }, at(f.at + f.dur - 0.2));
    },
    title(tl, f) {
      // per word: blur 6px -> 0 and opacity 0 -> 1 in 0.13 s, words 0.08 s apart, no translate; held dead still
      const ws = document.querySelectorAll($(f.el) + " .mvk-w");
      ws.forEach((w, i) => tl.fromTo(w, { opacity: 0, filter: "blur(6px)" },
        { opacity: 1, filter: "blur(0px)", duration: 0.13, ease: "power2.out", immediateRender: false }, at(f.at + i * (f.stagger ?? 0.08))));
      tl.fromTo($(f.el), { opacity: 1 }, { opacity: 0, filter: "blur(6px)", duration: 0.18, ease: "power2.in", immediateRender: false }, at(f.at + f.dur - 0.18));
    },
    type(tl, f) {
      const cs = document.querySelectorAll($(f.el) + " .mvk-c");
      const step = f.step ?? Math.min(0.05, (f.dur * 0.5) / Math.max(1, cs.length));
      cs.forEach((c, i) => tl.fromTo(c, { opacity: 0 }, { opacity: 1, duration: 0.02, ease: "none", immediateRender: false }, at(f.at + i * step)));
      tl.to($(f.el), { opacity: 0, duration: 0.16, ease: "power2.in" }, at(f.at + f.dur - 0.16));
    },
    stamp(tl, f) {
      tl.fromTo($(f.el), { opacity: 0, scale: 2.2, rotation: (f.rot ?? -7) - 8 },
        { opacity: 1, scale: 1, rotation: f.rot ?? -7, duration: 0.18, ease: "back.out(1.6)", immediateRender: false }, at(f.at));
      tl.to($(f.el), { opacity: 0, duration: 0.2, ease: "power1.in" }, at(f.at + f.dur - 0.2));
    },
    label(tl, f) {
      tl.fromTo($(f.el), { opacity: 0, clipPath: "inset(0 100% 0 0)" },
        { opacity: 1, clipPath: "inset(0 0% 0 0)", duration: 0.35, ease: "power3.out", immediateRender: false }, at(f.at));
      tl.to($(f.el), { opacity: 0, duration: 0.2, ease: "power1.in" }, at(f.at + f.dur - 0.2));
    },
    counter(tl, f) {
      const el = document.querySelector($(f.el) + " .mvk-num");
      const o = { v: f.from ?? 0 };
      const fmt = (v) => Math.round(v).toLocaleString(f.locale || "vi-VN");
      tl.fromTo($(f.el), { opacity: 0, scale: 0.9 }, { opacity: 1, scale: 1, duration: 0.18, ease: "power3.out", immediateRender: false }, at(f.at));
      tl.fromTo(o, { v: f.from ?? 0 }, { v: f.to, duration: f.run ?? Math.min(1.2, f.dur * 0.6), ease: "power2.out", immediateRender: false,
        onUpdate: () => { if (el) el.textContent = fmt(o.v) + (f.suffix || ""); } }, at(f.at));
      tl.to($(f.el), { opacity: 0, duration: 0.2, ease: "power1.in" }, at(f.at + f.dur - 0.2));
    },
    endtag(tl, f) {
      tl.fromTo($(f.el) + " .mvk-end-fade", { opacity: 0 }, { opacity: 1, duration: 0.5, ease: "power1.out", immediateRender: false }, at(f.at));
      tl.fromTo($(f.el) + " .mvk-end-title", { opacity: 0, filter: "blur(10px)" }, { opacity: 1, filter: "blur(0px)", duration: 0.5, ease: "power3.out", immediateRender: false }, at(f.at + 0.1));
      tl.fromTo($(f.el) + " .mvk-end-line", { opacity: 0 }, { opacity: 1, duration: 0.35, ease: "power2.out", immediateRender: false }, at(f.at + 0.45));
    },
    flash(tl, f, sp) {
      tl.fromTo($(sp.id + "-flash"), { opacity: f.peak ?? 0.8 }, { opacity: 0, duration: f.dur ?? 0.3, ease: "power2.out", immediateRender: false }, at(f.at));
    },
    black(tl, f, sp) {
      tl.fromTo($(sp.id + "-black"), { opacity: 0 }, { opacity: f.to ?? 1, duration: f.in ?? 0.12, ease: "power1.in", immediateRender: false }, at(f.at));
      if (f.hold != null) tl.to($(sp.id + "-black"), { opacity: 0, duration: f.out ?? 0.2, ease: "power1.out" }, at(f.at + (f.in ?? 0.12) + f.hold));
    },
    shake(tl, f, sp) {
      const n = Math.max(2, Math.round(f.dur / 0.045)), amp = f.amp ?? 16, r = rng(f.seed || 7);
      for (let i = 0; i < n; i++) {
        const a = amp * (1 - i / n);
        tl.to($(sp.id + "-world"), { x: (r() * 2 - 1) * a, y: (r() * 2 - 1) * a * 0.7, duration: 0.045, ease: "none" }, at(f.at + i * 0.045));
      }
      tl.to($(sp.id + "-world"), { x: 0, y: 0, duration: 0.06, ease: "power1.out" }, at(f.at + n * 0.045));
    },
    punch(tl, f, sp) {
      tl.to($(sp.id + "-world"), { scale: 1 + (f.amt ?? 0.06), duration: 0.08, ease: "power2.out" }, at(f.at));
      tl.to($(sp.id + "-world"), { scale: 1, duration: f.dur ?? 0.35, ease: "power2.inOut" }, at(f.at + 0.08));
    },
    dim(tl, f) {
      tl.to($(f.el), { opacity: f.to ?? 0.5, duration: f.dur ?? 0.4, ease: "power1.inOut" }, at(f.at));
    },
    // ---------- explain and show ----------
    pointer(tl, f) {
      // the lettering pops, the leader line draws on from it, the dot lands on what it names
      const q = (c) => $(f.el) + " " + c;
      tl.fromTo($(f.el), { opacity: 0 }, { opacity: 1, duration: 0.05, ease: "none", immediateRender: false }, at(f.at));
      tl.fromTo(q(".mvk-pt-text > span"), { scale: 0.6, rotation: -4 }, { scale: 1, rotation: 0, duration: 0.22, ease: "back.out(2.2)", immediateRender: false }, at(f.at));
      tl.fromTo(q(".mvk-pt-line"), { strokeDashoffset: 1 }, { strokeDashoffset: 0, duration: 0.3, ease: "power2.inOut", immediateRender: false }, at(f.at + 0.12));
      tl.fromTo(q(".mvk-pt-dot"), { scale: 0 }, { scale: 1, duration: 0.18, ease: "back.out(3)", immediateRender: false }, at(f.at + 0.38));
      tl.to($(f.el), { opacity: 0, duration: 0.2, ease: "power1.in" }, at(f.at + f.dur - 0.2));
    },
    html(tl, f) {
      // raw markup: in "fade" (default), "pop", "draw" (every [pathLength] stroke draws on) or "none"; tweens do the rest
      const how = f.in || "fade";
      if (how === "pop") tl.fromTo($(f.el), { opacity: 0, scale: 0.7 }, { opacity: 1, scale: 1, duration: 0.24, ease: "back.out(2)", immediateRender: false }, at(f.at));
      else tl.fromTo($(f.el), { opacity: 0 }, { opacity: 1, duration: how === "fade" ? 0.18 : 0.01, ease: "none", immediateRender: false }, at(f.at));
      if (how === "draw") {
        document.querySelectorAll($(f.el) + " [pathLength]").forEach((el, i) => tl.fromTo(el, { strokeDasharray: 1, strokeDashoffset: 1 },
          { strokeDashoffset: 0, duration: f.run ?? 0.35, ease: "power2.inOut", immediateRender: false }, at(f.at + i * (f.stagger ?? 0.08))));
      }
      if (f.out !== "none") tl.to($(f.el), { opacity: 0, duration: 0.2, ease: "power1.in" }, at(f.at + f.dur - 0.2));
    },
    callout(tl, f) {
      // a frame drawn around what the viewer must look at; with spot, the rest of the frame darkens
      tl.fromTo($(f.el), { opacity: 0, scale: 1.12 }, { opacity: 1, scale: 1, duration: 0.24, ease: "power3.out", immediateRender: false }, at(f.at));
      const spot = document.querySelector($(f.el) + " .mvk-callout-spot");
      if (spot) tl.fromTo(spot, { opacity: 0 }, { opacity: 1, duration: 0.3, ease: "power1.out", immediateRender: false }, at(f.at));
      tl.fromTo($(f.el) + " .mvk-callout-box", { clipPath: "inset(0 100% 100% 0)" },
        { clipPath: "inset(0 0% 0% 0)", duration: 0.32, ease: "power2.inOut", immediateRender: false }, at(f.at));
      const lab = document.querySelector($(f.el) + " .mvk-callout-label");
      if (lab) tl.fromTo(lab, { opacity: 0, y: 12 }, { opacity: 1, y: 0, duration: 0.22, ease: "power2.out", immediateRender: false }, at(f.at + 0.22));
      tl.to($(f.el), { opacity: 0, duration: 0.2, ease: "power1.in" }, at(f.at + f.dur - 0.2));
    },
    tap(tl, f) {
      tl.fromTo($(f.el), { opacity: 0 }, { opacity: 1, duration: 0.04, ease: "none", immediateRender: false }, at(f.at));
      tl.fromTo($(f.el) + " .mvk-tap-dot", { scale: 0.4 }, { scale: 1, duration: 0.1, ease: "back.out(2)", immediateRender: false }, at(f.at));
      tl.fromTo($(f.el) + " .mvk-tap-ring", { scale: 0.25, opacity: 0.95 },
        { scale: 1.9, opacity: 0, duration: Math.min(0.55, f.dur), ease: "power2.out", immediateRender: false }, at(f.at));
      tl.to($(f.el), { opacity: 0, duration: 0.12, ease: "none" }, at(f.at + f.dur - 0.12));
    },
    steps(tl, f) {
      // items land one by one (on their words when the resolver gave times); with focus, earlier ones step back
      const items = document.querySelectorAll($(f.el) + " .mvk-step");
      const gap = Math.min(0.9, (f.dur * 0.6) / Math.max(1, items.length));
      const ts = f.times || Array.from(items, (_, i) => i * gap);
      tl.fromTo($(f.el), { opacity: 0 }, { opacity: 1, duration: 0.01, ease: "none", immediateRender: false }, at(f.at));
      items.forEach((it, i) => {
        tl.fromTo(it, { opacity: 0, x: -40 }, { opacity: 1, x: 0, duration: 0.26, ease: "power3.out", immediateRender: false }, at(f.at + ts[i]));
        if (f.focus !== false && i > 0) tl.to(items[i - 1], { opacity: 0.5, duration: 0.2, ease: "power1.out" }, at(f.at + ts[i]));
      });
      tl.to($(f.el), { opacity: 0, duration: 0.2, ease: "power1.in" }, at(f.at + f.dur - 0.2));
    },
    compare(tl, f) {
      const ts = f.times || [0, Math.min(1.2, f.dur * 0.4)];
      tl.fromTo($(f.el), { opacity: 0 }, { opacity: 1, duration: 0.01, ease: "none", immediateRender: false }, at(f.at));
      ["l", "r"].forEach((side, k) => {
        const col = $(f.el) + " .mvk-cmp-" + side;
        tl.fromTo(col, { opacity: 0, y: 24 }, { opacity: 1, y: 0, duration: 0.28, ease: "power3.out", immediateRender: false }, at(f.at + (ts[k] ?? 0)));
        document.querySelectorAll(col + " p").forEach((p, i) =>
          tl.fromTo(p, { opacity: 0 }, { opacity: 1, duration: 0.18, ease: "power1.out", immediateRender: false }, at(f.at + (ts[k] ?? 0) + 0.12 + i * 0.12)));
      });
      tl.to($(f.el), { opacity: 0, duration: 0.2, ease: "power1.in" }, at(f.at + f.dur - 0.2));
    },
    bars(tl, f) {
      tl.fromTo($(f.el), { opacity: 0 }, { opacity: 1, duration: 0.2, ease: "power1.out", immediateRender: false }, at(f.at));
      document.querySelectorAll($(f.el) + " .mvk-bar").forEach((b, i) => {
        const t = f.at + 0.15 + i * (f.stagger ?? 0.14);
        tl.fromTo(b.querySelector("i"), { scaleX: 0 }, { scaleX: 1, duration: 0.6, ease: "power3.out", immediateRender: false }, at(t));
        tl.fromTo(b.querySelector("em"), { opacity: 0 }, { opacity: 1, duration: 0.2, ease: "none", immediateRender: false }, at(t + 0.45));
      });
      tl.to($(f.el), { opacity: 0, duration: 0.2, ease: "power1.in" }, at(f.at + f.dur - 0.2));
    },
    dots(tl, f) {
      // the grid lands, then the lit dots light one by one (the number as a share the eye can count)
      const q = (c) => $(f.el) + " " + c;
      tl.fromTo($(f.el), { opacity: 0 }, { opacity: 1, duration: 0.2, ease: "power1.out", immediateRender: false }, at(f.at));
      const all = document.querySelectorAll(q(".mvk-dot-grid i")), lit = document.querySelectorAll(q(".mvk-dot-grid i.lit b"));
      const step = Math.min(0.012, 0.45 / Math.max(1, all.length));
      all.forEach((d, i) => tl.fromTo(d, { scale: 0 }, { scale: 1, duration: 0.16, ease: "back.out(2)", immediateRender: false }, at(f.at + 0.1 + i * step)));
      const t1 = f.at + 0.2 + all.length * step + (f.hold ?? 0.25), ls = Math.min(0.12, 0.9 / Math.max(1, lit.length));
      lit.forEach((d, i) => tl.fromTo(d, { opacity: 0 }, { opacity: 1, duration: 0.14, ease: "power2.out", immediateRender: false }, at(t1 + i * ls)));
      const sm = document.querySelector(q("small"));
      if (sm) tl.fromTo(sm, { opacity: 0 }, { opacity: 0.9, duration: 0.25, ease: "none", immediateRender: false }, at(t1));
      tl.to($(f.el), { opacity: 0, duration: 0.2, ease: "power1.in" }, at(f.at + f.dur - 0.2));
    },
    ladder(tl, f) {
      // both ladders climb from the bottom at once; the marked rung lights last
      const q = (c) => $(f.el) + " " + c;
      tl.fromTo($(f.el), { opacity: 0 }, { opacity: 1, duration: 0.2, ease: "power1.out", immediateRender: false }, at(f.at));
      let last = f.at;
      ["l", "r"].forEach((side) => {
        const ps = document.querySelectorAll(q(".mvk-lad-" + side + " p")), st = Math.min(0.08, 0.8 / Math.max(1, ps.length));
        ps.forEach((p, i) => {
          const t = f.at + 0.15 + i * st;
          tl.fromTo(p, { opacity: 0, y: 14 }, { opacity: 1, y: 0, duration: 0.2, ease: "power2.out", immediateRender: false }, at(t));
          last = Math.max(last, t + 0.2);
        });
      });
      const hot = document.querySelector(q("p.hot"));
      if (hot) tl.fromTo(hot, { scale: 1 }, { scale: 1.12, duration: 0.18, ease: "power2.out", yoyo: true, repeat: 1, immediateRender: false }, at(last + 0.15));
      tl.to($(f.el), { opacity: 0, duration: 0.2, ease: "power1.in" }, at(f.at + f.dur - 0.2));
    },
    quote(tl, f) {
      tl.fromTo($(f.el), { opacity: 0 }, { opacity: 1, duration: 0.01, ease: "none", immediateRender: false }, at(f.at));
      tl.fromTo($(f.el) + " .mvk-q-mark", { opacity: 0, scale: 0.6 }, { opacity: 1, scale: 1, duration: 0.25, ease: "back.out(1.7)", immediateRender: false }, at(f.at));
      const ws = document.querySelectorAll($(f.el) + " .mvk-w");
      ws.forEach((w, i) => tl.fromTo(w, { opacity: 0, filter: "blur(6px)" },
        { opacity: 1, filter: "blur(0px)", duration: 0.13, ease: "power2.out", immediateRender: false }, at(f.at + 0.15 + i * (f.stagger ?? 0.06))));
      const sm = document.querySelector($(f.el) + " small");
      if (sm) tl.fromTo(sm, { opacity: 0 }, { opacity: 0.9, duration: 0.25, ease: "none", immediateRender: false }, at(f.at + 0.3 + ws.length * (f.stagger ?? 0.06)));
      tl.to($(f.el), { opacity: 0, duration: 0.2, ease: "power1.in" }, at(f.at + f.dur - 0.2));
    },
    lower(tl, f) {
      tl.fromTo($(f.el), { opacity: 1, clipPath: "inset(0 100% 0 0)" },
        { clipPath: "inset(0 0% 0 0)", duration: 0.36, ease: "power3.out", immediateRender: false }, at(f.at));
      const sub = document.querySelector($(f.el) + " .mvk-lower-sub");
      if (sub) tl.fromTo(sub, { opacity: 0, y: 8 }, { opacity: 1, y: 0, duration: 0.22, ease: "power2.out", immediateRender: false }, at(f.at + 0.2));
      tl.to($(f.el), { opacity: 0, duration: 0.2, ease: "power1.in" }, at(f.at + f.dur - 0.2));
    },
    speedlines(tl, f) {
      // manga speed lines: snap in, flicker through their seeded drawings, snap out
      const box = document.querySelector($(f.el));
      if (!box) return;
      const gs = box.querySelectorAll(".mvk-sv"), step = 1 / (f.fps || 12);
      tl.fromTo(box, { opacity: 0 }, { opacity: f.opacity ?? 0.9, duration: 0.05, ease: "none", immediateRender: false }, at(f.at));
      if (gs.length > 1) {
        for (let t = f.at + step, k = 1; t < f.at + f.dur; t += step, k++) gs.forEach((g, i) => tl.set(g, { opacity: i === k % gs.length ? 1 : 0 }, at(t)));
      }
      tl.to(box, { opacity: 0, duration: 0.15, ease: "power1.in" }, at(f.at + f.dur - 0.15));
    },
    weather(tl, f) {
      // rain, snow, embers, petals: seeded particles falling (or rising) for the effect's span
      const box = document.querySelector($(f.el));
      if (!box) return;
      const r = rng(f.seed || 11), parts = box.querySelectorAll(".mvk-p"), H = f.H, W = f.W;
      const up = f.kind === "embers";
      tl.fromTo(box, { opacity: 0 }, { opacity: f.opacity ?? 0.8, duration: 0.5, ease: "none", immediateRender: false }, at(f.at));
      parts.forEach((p) => {
        const speed = (f.kind === "rain" ? 1900 : f.kind === "snow" ? 120 : f.kind === "embers" ? 160 : 180) * (0.6 + r() * 0.8);
        const x0 = r() * W, drift = (f.kind === "rain" ? -0.18 : (r() - 0.5) * 0.6) * speed;
        const y0 = r() * H, span = H + 200, fall = span / speed;
        // one pass per fall time, phase-shifted by y0; finite repeats only
        const reps = Math.ceil((f.dur + fall) / fall);
        tl.fromTo(p, { x: x0, y: up ? H - y0 : y0 - 100, rotation: r() * 360 },
          { x: x0 + drift * fall, y: up ? -100 - y0 : span + y0 - 100, rotation: "+=" + (f.kind === "petals" ? 220 : 0),
            duration: fall, ease: "none", repeat: reps, immediateRender: false }, at(f.at - (y0 / speed)));
      });
      tl.to(box, { opacity: 0, duration: 0.4, ease: "none" }, at(f.at + f.dur - 0.4));
    },
  };

  // ---------- 2.5D layers: a cut-out over the plate with its own depth and motion ----------
  // #<layer> takes the parallax camera, .mvk-lym the entrance and moves, .mvk-lyh the hits, .mvk-lyi the idle loop.
  const ENTER = {
    "slide-left": (W) => ({ x: 0.6 * W, y: 0 }), "slide-right": (W) => ({ x: -0.6 * W, y: 0 }),
    rise: (W, H) => ({ x: 0, y: 0.35 * H }), drop: (W, H) => ({ x: 0, y: -0.5 * H }),
  };
  const PARTS = { layer: ".mvk-ly", move: ".mvk-lym", hit: ".mvk-lyh", idle: ".mvk-lyi", img: ".mvk-lyi > .mvk-lyart", rig: ".mvk-rig" };
  const mv = (m) => ({ x: m.x, y: m.y, scale: m.s, rotation: m.rot });

  function layer(tl, s, L, W, H) {
    const q = (c) => $(L.id) + (c ? " " + c : "");
    const lead = s.lead || 0, t0 = s.start - lead, span = Math.max(0.05, s.end - t0);
    if (L.cam) {
      tl.fromTo(q(), cam(L.cam.from, W, H), { ...cam(L.cam.to, W, H), duration: span, ease: L.cam.ease || "none", immediateRender: false }, at(t0));
    }
    const e = L.enter;
    if (e) {
      if (e.type === "fade" || e.type === "pop") {
        tl.fromTo(q(".mvk-ly"), { opacity: 0 }, { opacity: 1, duration: e.type === "pop" ? e.dur * 0.5 : e.dur, ease: "power1.out", immediateRender: false }, at(e.at));
      }
      if (e.type === "pop") {
        tl.fromTo(q(".mvk-lym"), { scale: 0.6 }, { scale: 1, duration: e.dur, ease: "back.out(2.2)", immediateRender: false }, at(e.at));
      } else if (ENTER[e.type]) {
        tl.fromTo(q(".mvk-lym"), ENTER[e.type](W, H), { x: 0, y: 0, duration: e.dur, ease: e.type === "drop" ? "bounce.out" : "power3.out", immediateRender: false }, at(e.at));
      }
    }
    for (const m of L.moves || []) {
      if (m.antic) {  // lean back a little, then travel past and settle: the move reads as alive, its end time unchanged
        const pre = lerp(m.from, m.to, -m.antic), d1 = m.dur * 0.22;
        tl.fromTo(q(".mvk-lym"), mv(m.from), { ...mv(pre), duration: d1, ease: "power1.out", immediateRender: false }, at(m.at));
        tl.fromTo(q(".mvk-lym"), mv(pre), { ...mv(m.to), duration: m.dur - d1, ease: "back.out(1.4)", immediateRender: false }, at(m.at + d1));
      } else {
        tl.fromTo(q(".mvk-lym"), mv(m.from), { ...mv(m.to), duration: m.dur, ease: m.ease || "power2.inOut", immediateRender: false }, at(m.at));
      }
    }
    for (const h of L.hits || []) {
      tl.fromTo(q(".mvk-lyh"), { x: 0, y: 0, rotation: 0 }, { x: h.dx, y: h.dy, rotation: h.rot, duration: 0.07, ease: "power2.out", immediateRender: false }, at(h.at));
      tl.fromTo(q(".mvk-lyh"), { x: h.dx, y: h.dy, rotation: h.rot }, { x: 0, y: 0, rotation: 0, duration: 0.55, ease: "elastic.out(1, 0.45)", immediateRender: false }, at(h.at + 0.07));
    }
    for (const tw of L.tweens || []) {  // raw GSAP the agent wrote: a part, optional from, any vars
      const sel = q(tw.sel || PARTS[tw.part] || PARTS.move), vars = { ...tw.to, duration: tw.dur, ease: tw.ease, immediateRender: false };
      if (tw.from) tl.fromTo(sel, tw.from, vars, at(tw.at));
      else tl.to(sel, vars, at(tw.at));
    }
    if (L.swaps && L.swaps.length) {  // a pose or expression change: show one drawing of the stack
      const imgs = document.querySelectorAll(q(".mvk-lyart"));
      for (const w of L.swaps) imgs.forEach((im, k) => tl.set(im, { opacity: k === w.k ? 1 : 0 }, at(w.at)));
    }
    if (L.rig) rig(tl, s, L);
    boil(tl, s, L);
    ghosts(tl, s, L, W, H);
    const i = L.idle;
    if (i) {  // a finite yoyo over the shot's window (never repeat: -1): the frame stays a pure function of time
      const half = i.period / 2, reps = Math.max(0, Math.ceil(span / half) - 1);
      const [a, b] = i.type === "breathe" ? [{ scaleY: 1 }, { scaleY: 1 + 0.018 * i.amt }]
        : i.type === "bob" ? [{ y: 0 }, { y: -0.008 * H * i.amt }]
        : [{ rotation: -1.2 * i.amt }, { rotation: 1.2 * i.amt }];
      tl.fromTo(q(".mvk-lyi"), a, { ...b, duration: half, ease: "sine.inOut", yoyo: true, repeat: reps, immediateRender: false }, at(t0));
    }
  }

  // after-images: copy k (#<layer>-g<k>, written behind the layer) plays the layer's own timeline k * gap late,
  // shown only over each span, fainter the further behind
  function ghosts(tl, s, L, W, H) {
    const n = Math.max(0, ...(L.ghosts || []).map((g) => g.n));
    for (let k = 1; k <= n; k++) {
      const id = L.id + "-g" + k, sub = gsap.timeline({ paused: true });
      layer(sub, s, { ...L, id, ghosts: null }, W, H);
      sub.set({}, {}, s.end);
      for (const g of L.ghosts) {
        if (k > g.n) continue;
        const lag = k * g.gap, d = g.until - g.at, out = Math.min(0.15, d / 2);
        tl.fromTo(sub, { time: Math.max(0, g.at - lag) }, { time: Math.max(0, g.until - lag), duration: d, ease: "none", immediateRender: false }, at(g.at));
        tl.fromTo($(id), { opacity: 0 }, { opacity: g.op * (1 - (k - 1) / g.n), duration: 0.02, ease: "none", immediateRender: false }, at(g.at));
        tl.to($(id), { opacity: 0, duration: out, ease: "power1.in" }, at(g.until - out));
      }
    }
  }

  // ---------- stick rig: a figure drawn in code; each joint is a CSS variable on its <svg> ----------
  // A pose tween moves every joint at once (fromTo on --lean, --armR, … and --drop, the hips' sink).
  function rig(tl, s, L) {
    const R = L.rig, root = $(L.id) + " .mvk-rig";
    const vars = (p) => Object.fromEntries(Object.entries(p).map(([k, v]) => ["--" + k, v]));
    // steps: each pose tween holds on R.steps drawings a second, the way hand-drawn frames do
    const ease = (name, d) => {
      if (!(R.steps > 0)) return name;
      const e = gsap.parseEase(name), n = Math.max(1, Math.round(d * R.steps));
      return (p) => e(Math.floor(p * n + 1e-9) / n);
    };
    for (const p of R.tweens || []) {
      if (p.dur <= 0) {  // snap: the last pose holds until the key, then jumps
        tl.set(root, vars(p.to), at(p.at));
      } else if (p.antic) {
        const pre = lerp(p.from, p.to, -p.antic), d1 = p.dur * 0.25;
        tl.fromTo(root, vars(p.from), { ...vars(pre), duration: d1, ease: ease("power1.out", d1), immediateRender: false }, at(p.at));
        tl.fromTo(root, vars(pre), { ...vars(p.to), duration: p.dur - d1, ease: ease("back.out(1.3)", p.dur - d1), immediateRender: false }, at(p.at + d1));
      } else {
        tl.fromTo(root, vars(p.from), { ...vars(p.to), duration: p.dur, ease: ease(p.ease || "power2.inOut", p.dur), immediateRender: false }, at(p.at));
      }
    }
    const faces = document.querySelectorAll(root + " .rig-f");
    for (const f of R.facesets || []) faces.forEach((g) => tl.set(g, { opacity: g.classList.contains("rig-f-" + f.face) ? 1 : 0 }, at(f.at)));
    for (const [a, b] of R.flaps || []) {  // the mouth opens on each spoken word
      tl.set(root + " .rig-mo", { opacity: 1 }, at(a)); tl.set(root + " .rig-m", { opacity: 0 }, at(a));
      tl.set(root + " .rig-mo", { opacity: 0 }, at(b)); tl.set(root + " .rig-m", { opacity: 1 }, at(b));
    }
    for (const t of R.blinks || []) {  // a blink: the eyes (.rig-ey) shut and open again
      const ey = root + " .rig-ey", o = { transformOrigin: "50% 50%", ease: "none", immediateRender: false };
      tl.fromTo(ey, { scaleY: 1 }, { ...o, scaleY: 0.12, duration: 0.05 }, at(t));
      tl.fromTo(ey, { scaleY: 0.12 }, { ...o, scaleY: 1, duration: 0.08 }, at(t + 0.05));
    }
    (R.emotes || []).forEach((e, k) => {
      const el = root + " .rig-e" + k;
      tl.fromTo(el, { opacity: 0 }, { opacity: 1, duration: 0.08, ease: "none", immediateRender: false }, at(e.at));
      tl.fromTo(el + " .rig-ei", { scale: 0.35 }, { scale: 1, transformOrigin: "50% 50%", duration: 0.35, ease: "back.out(3)", immediateRender: false }, at(e.at));
      tl.to(el, { opacity: 0, duration: 0.15, ease: "none" }, at(e.at + Math.max(0.2, e.dur) - 0.15));
    });
  }

  // line boil (a rig or a drawn prop): the noise re-seeds on fixed steps (drawn "on twos"), a pure function of time
  function boil(tl, s, L) {
    const B = L.rig || L.vec, turb = B && B.boil > 0 && document.querySelector($(L.id) + " feTurbulence");
    if (!turb) return;
    const step = 1 / (B.boil_fps || 8), t0 = s.start - (s.lead || 0);
    for (let t = t0 + step, k = 2; t < s.end; t += step, k++) tl.set(turb, { attr: { seed: 1 + (k % 9) } }, at(t));
  }

  // hit-stop: real time r -> scene time. Each freeze holds the scene IMPACT seconds after its `at` (the frame where the
  // release pose and the recoil have already landed, not the one before contact) for its dur, then the scene plays
  // faster over CATCH seconds until it is back with the voice, so every later word-anchored moment stays on its word.
  const CATCH = 0.3, IMPACT = 2 / 30;
  const REALTIME = new Set(["flash", "speedlines"]);
  function remap(fz, dur) {
    return (p) => {
      const r = p * dur;
      let lag = 0;
      for (const f of fz) {
        const t = Math.min(f.at + IMPACT, dur), c = Math.max(0.05, Math.min(CATCH, dur - t - f.dur));
        if (r <= t) continue;
        lag += r < t + f.dur ? r - t : Math.max(0, f.dur * (1 - (r - t - f.dur) / c));
      }
      return Math.max(0, r - lag) / dur;
    };
  }

  function build(tl, sp) {
    const fz = (sp.fx || []).filter((f) => f.type === "freeze");
    if (!fz.length) return play(tl, sp);
    const inner = play(gsap.timeline({ paused: true }), sp);
    tl.fromTo(inner, { time: 0 }, { time: sp.dur, duration: sp.dur, ease: remap(fz, sp.dur), immediateRender: false }, 0);
    for (const f of fz) {  // the frame still shakes while the scene holds: the hit lands in real time
      if (f.amp) FX.shake(tl, { ...f, dur: Math.max(f.dur, 0.18) }, sp);
    }
    // the light and lines of the hit are not part of the held picture: held with the scene, a flash washes the hit
    // out and speedlines wait out the hold unseen, so both play in real time
    for (const f of sp.fx || []) if (REALTIME.has(f.type)) FX[f.type](tl, f, sp);
    tl.set({}, {}, sp.dur);
    return tl;
  }

  function play(tl, sp) {
    const W = sp.W, H = sp.H;
    sp.shots.forEach((s) => {
      (IN[s.in?.type] || IN.cut)(tl, s, sp);
      // drift over the whole window (an overlapping way in starts s.lead before s.start)
      const lead = s.lead || 0;
      if (s.cam) {
        tl.fromTo($(s.id) + " .mvk-kb", cam(s.cam.from, W, H),
          { ...cam(s.cam.to, W, H), duration: Math.max(0.05, s.end - s.start + lead), ease: s.cam.ease || "none", immediateRender: false }, at(s.start - lead));
      }
      let prev = { s: 1, x: 0, y: 0 };
      for (const k of s.keys || []) {
        tl.fromTo($(s.id) + " .mvk-kbo", cam(prev, W, H),
          { ...cam(k.to, W, H), duration: k.dur ?? 0.22, ease: k.ease || "power3.out", immediateRender: false }, at(k.at));
        prev = k.to;
      }
      for (const L of s.layers || []) layer(tl, s, L, W, H);
      if (s.out && OUT[s.out.type]) OUT[s.out.type](tl, s, sp);
    });
    for (const f of sp.fx || []) {
      if (f.shown) {  // on screen from its first frame (CSS .mvk-shown): only the exit animates
        tl.to($(f.el), { opacity: 0, duration: 0.18, ease: "power2.in" }, at(f.at + f.dur - 0.18));
        continue;
      }
      if (f.type === "freeze") continue;  // build() plays it on the outer timeline
      if (REALTIME.has(f.type) && (sp.fx || []).some((g) => g.type === "freeze")) continue;  // and the hit's light and lines
      if (FX[f.type]) FX[f.type](tl, f, sp);
      else console.warn("mvkit: unknown fx", f.type);
    }
    for (const tw of sp.tweens || []) {  // raw GSAP the agent wrote for the beat: any selector in the scene
      const sel = tw.sel.startsWith("#") ? tw.sel : "#" + sp.id + "-world " + tw.sel;
      const vars = { ...tw.to, duration: tw.dur, ease: tw.ease, immediateRender: false };
      if (tw.from) tl.fromTo(sel, tw.from, vars, at(tw.at));
      else tl.to(sel, vars, at(tw.at));
    }
    // the timeline must span the slot so the last frame holds what it should
    tl.set({}, {}, sp.dur);
    return tl;
  }

  // Overlay (grain + vignette + chip), built by compositions/overlay.html.
  function overlay(tl, o) {
    if (o.grain) {
      const g = document.querySelector("#overlay-grain");
      const r = rng(o.seed || 3), step = 1 / 12;
      for (let t = step; t < o.dur; t += step) {  // from one step in: a set at t=0 does not survive the seek order
        tl.set(g, { backgroundPosition: `${Math.floor(r() * 512)}px ${Math.floor(r() * 512)}px` }, t);
      }
    }
    // the chip is on screen from frame 0 (CSS opacity 1): a frame 0 without it reads as a blank start
    if (o.chip && o.chip.until != null) {
      tl.fromTo("#overlay-chip", { opacity: 1 }, { opacity: 0, duration: 0.3, ease: "power1.in", immediateRender: false }, at(o.chip.until));
    }
    tl.set({}, {}, o.dur);
    return tl;
  }

  // A hand-authored ("custom") scene may call these too: MVK.layer(tl, shot, layerSpec, W, H) plays one 2.5D layer.
  window.MVK = { build, overlay, rng, layer, rig, cam };
})();
