// The browser client.
//
// Two views over one URL scheme: no ?patch= shows the gallery, a ?patch= shows
// the editor. Every link shared before this page existed therefore still opens
// the right drawing.
//
// Nothing about any patch is hard-coded. Controls are generated from the
// ParamSpec list the API reports and tiles from the gallery config, so a newly
// registered patch appears here with working sliders without this file
// changing.

const el = {
  header:   document.getElementById('header'),
  back:     document.getElementById('back'),
  crumb:    document.getElementById('crumb'),
  crumbCat: document.getElementById('crumb-category'),
  crumbPatch: document.getElementById('crumb-patch'),
  gallery:  document.getElementById('gallery'),
  categories: document.getElementById('categories'),
  editor:   document.getElementById('editor'),
  desc:     document.getElementById('patch-desc'),
  pagePreset: document.getElementById('page-preset'),
  pageW:    document.getElementById('page-w'),
  pageH:    document.getElementById('page-h'),
  pageSwap: document.getElementById('page-swap'),
  seed:     document.getElementById('seed'),
  reseed:   document.getElementById('reseed'),
  params:   document.getElementById('params'),
  reset:    document.getElementById('reset'),
  print:    document.getElementById('print'),
  svg:      document.getElementById('svg'),
  gcode:    document.getElementById('gcode'),
  status:   document.getElementById('status'),
  canvas:   document.getElementById('canvas'),
};

const ctx = el.canvas.getContext('2d');

let patches = [];          // full ParamSpec data, from /api/patches
let categories = [];       // gallery layout, from /api/gallery
let current = null;        // name of the patch being edited, or null
let lastFrame = null;      // most recent Frame, kept for printing
let inFlight = null;
let pending = false;

// ------------------------------------------------------------------ drawing

// Draw a Frame into a canvas, fitted to the given pixel box. Shared by the
// editor's big canvas and every gallery thumbnail, so a tile cannot drift from
// what the editor shows.
function paint(canvas, c, frame, boxW, boxH, lineWidth) {
  const scale = Math.min(boxW / frame.width, boxH / frame.height);
  const dpr = window.devicePixelRatio || 1;

  canvas.width = Math.round(frame.width * scale * dpr);
  canvas.height = Math.round(frame.height * scale * dpr);

  c.setTransform(1, 0, 0, 1, 0, 0);
  c.fillStyle = '#fff';
  c.fillRect(0, 0, canvas.width, canvas.height);

  c.setTransform(scale * dpr, 0, 0, scale * dpr,
                 -frame.minX * scale * dpr, -frame.minY * scale * dpr);
  c.strokeStyle = '#111';
  c.lineWidth = lineWidth;
  c.lineJoin = 'round';
  c.lineCap = 'round';

  // One Path2D for every polyline, stroked once: an individual stroke() per
  // path costs far more when there are thousands of them.
  const all = new Path2D();
  frame.paths.forEach((flat, i) => {
    if (flat.length < 4) return;
    all.moveTo(flat[0], flat[1]);
    for (let j = 2; j < flat.length; j += 2) all.lineTo(flat[j], flat[j + 1]);
    if (frame.closed[i]) all.closePath();
  });
  c.stroke(all);
}

// ------------------------------------------------------------------ gallery

function showGallery() {
  current = null;
  el.gallery.hidden = false;
  el.editor.hidden = true;
  el.back.hidden = true;
  el.crumb.hidden = true;
  document.title = 'adraw';
}

function buildGallery() {
  el.categories.innerHTML = '';

  for (const cat of categories) {
    const section = document.createElement('section');
    section.className = 'category';

    const h = document.createElement('h2');
    h.textContent = cat.name;
    section.append(h);

    if (cat.description) {
      const p = document.createElement('p');
      p.textContent = cat.description;
      section.append(p);
    }

    const grid = document.createElement('div');
    grid.className = 'tiles';

    for (const tile of cat.patches) {
      const button = document.createElement('button');
      button.className = 'tile';
      button.addEventListener('click', () => openPatch(tile.name, cat.name));

      const canvas = document.createElement('canvas');

      const body = document.createElement('div');
      body.className = 'tile-body';

      const name = document.createElement('div');
      name.className = 'tile-name';
      name.textContent = tile.name;

      const desc = document.createElement('div');
      desc.className = 'tile-desc';
      desc.textContent = tile.description;

      body.append(name, desc);
      button.append(canvas, body);
      grid.append(button);

      drawThumbnail(canvas, tile);
    }

    section.append(grid);
    el.categories.append(section);
  }
}

async function drawThumbnail(canvas, tile) {
  const q = new URLSearchParams({ patch: tile.name, seed: tile.previewSeed });
  for (const [k, v] of Object.entries(tile.preview || {})) q.set(k, v);

  try {
    const res = await fetch('/api/render?' + q);
    const frame = await res.json();
    if (!res.ok) throw new Error(frame.error || res.statusText);

    // Thumbnails are drawn at a fixed internal size rather than the element's
    // measured width: the grid may still be laying out when this resolves.
    paint(canvas, canvas.getContext('2d'), frame, 560, 396, 0.45);
  } catch (err) {
    const c = canvas.getContext('2d');
    canvas.width = 560;
    canvas.height = 396;
    c.fillStyle = '#fff';
    c.fillRect(0, 0, 560, 396);
    c.fillStyle = '#b00';
    c.font = '16px sans-serif';
    c.fillText(err.message, 14, 30);
  }
}

// --------------------------------------------------------------------- page
//
// Page size is a property of the paper, not of the pattern, so it sits outside
// the generated parameter list and survives switching patches. The server takes
// it as its own request field for the same reason: the same drawing makes sense
// on A4 or on a square, and nobody wants a MIDI knob mapped onto paper size.

const PAGE_PRESETS = [
  { label: 'A4 landscape',  w: 297, h: 210 },
  { label: 'A4 portrait',   w: 210, h: 297 },
  { label: 'A3 landscape',  w: 420, h: 297 },
  { label: 'A3 portrait',   w: 297, h: 420 },
  { label: 'A5 landscape',  w: 210, h: 148 },
  { label: 'Square',        w: 210, h: 210 },
  { label: 'Letter',        w: 279, h: 216 },
];

const DEFAULT_PAGE = PAGE_PRESETS[0];

function buildPagePresets() {
  for (const [i, p] of PAGE_PRESETS.entries()) {
    const opt = document.createElement('option');
    opt.value = String(i);
    opt.textContent = `${p.label} — ${p.w}×${p.h}`;
    el.pagePreset.append(opt);
  }
  const custom = document.createElement('option');
  custom.value = 'custom';
  custom.textContent = 'Custom';
  el.pagePreset.append(custom);
}

function pageSize() {
  const w = parseFloat(el.pageW.value);
  const h = parseFloat(el.pageH.value);
  if (!Number.isFinite(w) || !Number.isFinite(h)) return { ...DEFAULT_PAGE };
  return { w: clampPage(w), h: clampPage(h) };
}

function clampPage(v) {
  return Math.min(2000, Math.max(10, Math.round(v)));
}

// Keep the dropdown showing whichever preset matches the current numbers, so
// typing 210x297 by hand selects "A4 portrait" rather than leaving it on stale.
function syncPagePreset() {
  const { w, h } = pageSize();
  const i = PAGE_PRESETS.findIndex(p => p.w === w && p.h === h);
  el.pagePreset.value = i >= 0 ? String(i) : 'custom';
}

function setPage(w, h) {
  el.pageW.value = clampPage(w);
  el.pageH.value = clampPage(h);
  syncPagePreset();
}

// ------------------------------------------------------------------- editor

function currentPatch() {
  return patches.find(p => p.name === current);
}

function categoryOf(name) {
  for (const cat of categories) {
    if (cat.patches.some(p => p.name === name)) return cat.name;
  }
  return '';
}

// The step a continuous parameter's slider should use.
//
// A range slider only produces min + k*step, so an arbitrary step makes round
// values unreachable: (60-5)/200 is 0.275, and no number of those from 5 lands
// on 30 — the address bar ends up reading cell=30.025. Rounding the step up to
// the nearest 1, 2 or 5 times a power of ten keeps roughly two hundred
// positions while making every value a human would type reachable.
function sliderStep(spec) {
  if (spec.step) return spec.step;
  const raw = (spec.max - spec.min) / 200;
  const mag = Math.pow(10, Math.floor(Math.log10(raw)));
  for (const m of [1, 2, 5]) {
    if (m * mag >= raw) return m * mag;
  }
  return 10 * mag;
}

function quantize(spec, v) {
  const step = sliderStep(spec);
  const snapped = Math.round(v / step) * step;
  const places = Math.max(0, -Math.floor(Math.log10(step)));
  return parseFloat(snapped.toFixed(places));
}

function decimals(spec) {
  if (spec.step >= 1) return 0;
  return (spec.max - spec.min) <= 2 ? 2 : 1;
}

function buildParams() {
  const patch = currentPatch();
  el.desc.textContent = patch.description;
  el.params.innerHTML = '';

  for (const spec of patch.params) {
    const wrap = document.createElement('div');
    wrap.className = 'param';

    const head = document.createElement('div');
    head.className = 'param-head';

    const name = document.createElement('span');
    name.className = 'param-name';
    name.textContent = spec.label;

    const value = document.createElement('span');
    value.className = 'param-value';

    head.append(name, value);

    const slider = document.createElement('input');
    slider.type = 'range';
    slider.min = spec.min;
    slider.max = spec.max;
    slider.step = sliderStep(spec);
    slider.value = spec.default;
    slider.dataset.key = spec.key;

    const show = () => {
      const v = quantize(spec, parseFloat(slider.value));
      value.textContent = v.toFixed(decimals(spec)) + (spec.unit ? ' ' + spec.unit : '');
    };
    show();
    slider.addEventListener('input', e => {
      show();
      // applyURL dispatches a non-bubbling event purely to refresh the
      // readout; it issues its own single request afterwards.
      if (e.bubbles !== false || e.isTrusted) request();
    });

    const desc = document.createElement('p');
    desc.className = 'param-desc';
    desc.textContent = spec.description;

    wrap.append(head, slider, desc);
    el.params.append(wrap);
  }
}

function values() {
  const out = {};
  for (const slider of el.params.querySelectorAll('input[type="range"]')) {
    const spec = currentPatch().params.find(p => p.key === slider.dataset.key);
    out[slider.dataset.key] = quantize(spec, parseFloat(slider.value));
  }
  return out;
}

function query() {
  const { w, h } = pageSize();
  const q = new URLSearchParams({
    patch: current,
    seed: el.seed.value || '1',
    page: `${w}x${h}`,
  });
  for (const [k, v] of Object.entries(values())) q.set(k, v);
  return q;
}

function openPatch(name, categoryName) {
  current = name;
  el.gallery.hidden = true;
  el.editor.hidden = false;
  el.back.hidden = false;
  el.crumb.hidden = false;
  el.crumbCat.textContent = categoryName || categoryOf(name);
  el.crumbPatch.textContent = name;
  document.title = 'adraw / ' + name;

  buildParams();
  request();
}

// ----------------------------------------------------------------- url state

// The address bar mirrors the current settings, so any drawing you like is a
// shareable, bookmarkable link — and the same query string works verbatim
// against /api/render and /api/export.
function syncURL() {
  history.replaceState(null, '', '?' + query());
}

// Read the view and settings out of the address bar. Unknown keys and
// out-of-range values are ignored rather than rejected, so a hand-edited or
// truncated link still opens something rather than an error.
function applyURL() {
  const q = new URLSearchParams(location.search);
  const name = q.get('patch');

  if (!name || !patches.some(p => p.name === name)) {
    showGallery();
    return;
  }

  current = name;
  el.gallery.hidden = true;
  el.editor.hidden = false;
  el.back.hidden = false;
  el.crumb.hidden = false;
  el.crumbCat.textContent = categoryOf(name);
  el.crumbPatch.textContent = name;
  document.title = 'adraw / ' + name;

  buildParams();

  const seed = q.get('seed');
  if (seed && Number.isFinite(+seed)) el.seed.value = seed;

  const page = (q.get('page') || '').split('x');
  if (page.length === 2 && Number.isFinite(+page[0]) && Number.isFinite(+page[1])) {
    setPage(+page[0], +page[1]);
  } else {
    setPage(DEFAULT_PAGE.w, DEFAULT_PAGE.h);
  }

  for (const slider of el.params.querySelectorAll('input[type="range"]')) {
    const raw = q.get(slider.dataset.key);
    if (raw === null || !Number.isFinite(+raw)) continue;
    const spec = currentPatch().params.find(p => p.key === slider.dataset.key);
    slider.value = Math.min(spec.max, Math.max(spec.min, +raw));
    slider.dispatchEvent(new Event('input', { bubbles: false }));
  }
  request();
}

// ------------------------------------------------------------------ requests

async function request() {
  if (!current) return;

  // Coalesce: while a request is open, remember that another is wanted and
  // fire exactly one when it lands. Dragging a slider then costs one request
  // per round trip instead of one per pixel.
  if (inFlight !== null) { pending = true; return; }

  const id = (inFlight = Date.now());
  const started = performance.now();

  try {
    const res = await fetch('/api/render?' + query());
    const body = await res.json();
    if (!res.ok) throw new Error(body.error || res.statusText);
    if (inFlight !== id) return;

    lastFrame = body;

    const stage = el.canvas.parentElement.getBoundingClientRect();
    paint(el.canvas, ctx, body, stage.width - 56, stage.height - 56, 0.3);
    el.canvas.style.width = (el.canvas.width / (window.devicePixelRatio || 1)) + 'px';
    el.canvas.style.height = (el.canvas.height / (window.devicePixelRatio || 1)) + 'px';

    syncURL();
    el.status.classList.remove('error');
    el.status.textContent =
      `${body.paths.length} paths · ${Math.round(performance.now() - started)} ms`;
  } catch (err) {
    el.status.classList.add('error');
    el.status.textContent = err.message;
  } finally {
    inFlight = null;
    if (pending) { pending = false; request(); }
  }
}

// Fetched and saved as a blob rather than navigated to, so it also works in the
// static WebAssembly build, where /api/ exists only inside fetch.
async function download(format) {
  const q = query();
  q.set('format', format);
  try {
    const res = await fetch('/api/export?' + q);
    if (!res.ok) throw new Error((await res.json()).error || res.statusText);

    const name = /filename=([^;]+)/.exec(res.headers.get('Content-Disposition') || '');
    const a = document.createElement('a');
    a.href = URL.createObjectURL(await res.blob());
    a.download = name ? name[1] : 'drawing.' + format;
    a.click();
    // Revoking in the same tick can cancel the download in some browsers.
    setTimeout(() => URL.revokeObjectURL(a.href), 1000);
  } catch (err) {
    el.status.classList.add('error');
    el.status.textContent = err.message;
  }
}

// -------------------------------------------------------------------- print
//
// Printing emits real vector SVG rather than the canvas.
//
// A canvas prints as raster at roughly screen resolution, which for a tool
// whose output is millimetre-precise line work would mean a soft, wrongly
// scaled page. The SVG carries physical mm dimensions, so a 297x210 drawing
// prints at exactly A4 and every line stays crisp at any printer resolution.
//
// It is built on beforeprint rather than kept up to date, and that timing is
// the whole trick: injecting several thousand <path> elements costs 50-100ms of
// DOM work, which on every slider tick would undo precisely what drawing to a
// canvas bought. beforeprint is synchronous, but the frame is already in
// memory, so no fetch is needed and none of that cost is paid while working.

function frameToSVG(frame, strokeWidth) {
  const parts = [];
  frame.paths.forEach((flat, i) => {
    if (flat.length < 4) return;
    let d = 'M' + flat[0] + ' ' + flat[1];
    for (let j = 2; j < flat.length; j += 2) d += 'L' + flat[j] + ' ' + flat[j + 1];
    if (frame.closed[i]) d += 'Z';
    parts.push('<path d="' + d + '"/>');
  });
  return '<svg xmlns="http://www.w3.org/2000/svg" ' +
    'width="' + frame.width + 'mm" height="' + frame.height + 'mm" ' +
    'viewBox="' + frame.minX + ' ' + frame.minY + ' ' +
                  frame.width + ' ' + frame.height + '">' +
    '<g fill="none" stroke="#000" stroke-width="' + strokeWidth + '" ' +
    'stroke-linejoin="round" stroke-linecap="round">' +
    parts.join('') + '</g></svg>';
}

const printArea = document.getElementById('print-area');
const printPage = document.getElementById('print-page');

function preparePrint() {
  if (!current || !lastFrame) return;
  // Match the sheet to the drawing so it prints at true size with no margin
  // and no scale-to-fit, rather than being shrunk into a default A4 portrait.
  printPage.textContent =
    '@page { size: ' + lastFrame.width + 'mm ' + lastFrame.height + 'mm; margin: 0; }';
  printArea.innerHTML = frameToSVG(lastFrame, 0.3);
}

function clearPrint() {
  printArea.innerHTML = '';
  printPage.textContent = '';
}

window.addEventListener('beforeprint', preparePrint);
window.addEventListener('afterprint', clearPrint);

// Safari has historically not fired beforeprint; the media query listener
// covers it and is harmless where the events do work.
if (window.matchMedia) {
  const mq = window.matchMedia('print');
  const onChange = e => (e.matches ? preparePrint() : clearPrint());
  if (mq.addEventListener) mq.addEventListener('change', onChange);
  else if (mq.addListener) mq.addListener(onChange);
}

// --------------------------------------------------------------------- init

el.back.addEventListener('click', () => {
  history.replaceState(null, '', location.pathname);
  showGallery();
});
el.pagePreset.addEventListener('change', () => {
  const p = PAGE_PRESETS[+el.pagePreset.value];
  if (p) { setPage(p.w, p.h); request(); }
});
for (const input of [el.pageW, el.pageH]) {
  input.addEventListener('input', () => { syncPagePreset(); request(); });
}
el.pageSwap.addEventListener('click', () => {
  const { w, h } = pageSize();
  setPage(h, w);
  request();
});
el.seed.addEventListener('input', request);
el.reseed.addEventListener('click', () => {
  el.seed.value = Math.floor(Math.random() * 1e6);
  request();
});
el.reset.addEventListener('click', () => { buildParams(); request(); });
el.print.addEventListener('click', () => window.print());
el.svg.addEventListener('click', () => download('svg'));
el.gcode.addEventListener('click', () => download('gcode'));

window.addEventListener('popstate', applyURL);

let resizeTimer;
window.addEventListener('resize', () => {
  clearTimeout(resizeTimer);
  resizeTimer = setTimeout(request, 120);
});

(async function init() {
  try {
    const [pRes, gRes] = await Promise.all([
      fetch('/api/patches'),
      fetch('/api/gallery'),
    ]);
    patches = await pRes.json();
    categories = await gRes.json();
    if (!patches.length) throw new Error('no patches registered');

    buildPagePresets();
    setPage(DEFAULT_PAGE.w, DEFAULT_PAGE.h);
    buildGallery();
    applyURL();
  } catch (err) {
    el.gallery.hidden = false;
    el.categories.innerHTML =
      '<p class="lede" style="color:#ff7b72">could not load patches: ' +
      err.message + '</p>';
  }
})();
