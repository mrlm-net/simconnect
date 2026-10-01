/* Airport map: shared helpers. Plain scripts sharing the page's global scope,
   loaded in order: core, map, traffic, sections, main. */
'use strict';

const $ = (id) => document.getElementById(id);
const $$ = (sel, root = document) => [...root.querySelectorAll(sel)];
const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
const icon = (id, cls = 'ic') => `<svg class="${cls}" aria-hidden="true"><use href="#${id}"/></svg>`;
const kv = (obj) => Object.entries(obj).map(([k, v]) => `${k}: ${esc(v)}`).join('<br>');
const mmss = (s) => `${Math.floor(s / 60)}:${String(Math.floor(s % 60)).padStart(2, '0')}`;
const isPhone = () => window.matchMedia('(max-width: 699px)').matches;
const reduceMotion = () => window.matchMedia('(prefers-reduced-motion: reduce)').matches;

// localStorage, never throwing (private windows, blocked storage).
const store = {
  get(k, d = null) { try { const v = localStorage.getItem(k); return v === null ? d : v; } catch { return d; } },
  set(k, v) { try { localStorage.setItem(k, v); } catch { /* storage blocked */ } },
  json(k, d) { try { const v = JSON.parse(localStorage.getItem(k)); return v && typeof v === 'object' ? v : d; } catch { return d; } },
};

// api fetches a URL; it never throws. ok is false on an HTTP error or no
// server, with error the server's text.
// atcPosition is the position this device works in network play (#511):
// "" or "all" every position; the server refuses clearances for aircraft on
// other frequencies.
let atcPosition = '';
try { atcPosition = localStorage.getItem('apm-as') || ''; } catch { /* private window */ }
// The map server: when it last answered (any status) and last failed to.
let apiOkAt = Date.now(), apiFailAt = 0;
// serverDown: the map server itself not answering for a few seconds (not
// the simulator: the server answers without it).
function serverDown() { return apiFailAt > apiOkAt && Date.now() - apiOkAt > 5000; }
async function api(url, opts) {
  try {
    if (atcPosition && atcPosition !== 'all') {
      opts = { ...(opts || {}), headers: { ...((opts && opts.headers) || {}), 'X-ATC-Position': atcPosition } };
    }
    const res = await fetch(url, opts);
    apiOkAt = Date.now();
    if (res.status === 204) return { ok: true, status: 204, data: null };
    if (!res.ok) return { ok: false, status: res.status, error: (await res.text()).trim() || res.statusText };
    const ct = res.headers.get('Content-Type') || '';
    return { ok: true, status: res.status, data: ct.includes('json') ? await res.json() : await res.text() };
  } catch {
    apiFailAt = Date.now();
    return { ok: false, status: 0, error: 'the map server is not reachable' };
  }
}
const send = (url, body, method = 'POST') => api(url, body === undefined ? { method } : { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });

// Polling: one chain of timeouts per poller, never two requests at once.
// every() gives the interval now, or 0 to skip (its section is hidden).
// A failed poll (fn returns false) backs off up to 15 s; a hidden browser
// tab polls nothing.
const pollers = [];
function poller(name, fn, every) {
  const p = { name, failed: 0, timer: 0, running: false, again: false };
  p.run = async () => {
    clearTimeout(p.timer);
    let ms = every();
    if (ms && !document.hidden) {
      p.running = true;
      let ok = true;
      try { ok = (await fn()) !== false; } catch (e) { console.error(name, e); }
      p.running = false;
      p.failed = ok ? 0 : p.failed + 1;
      ms = every();
      if (p.again) { p.again = false; ms = 50; }
    }
    const wait = !ms ? 1000 : p.failed ? Math.max(ms, Math.min(15000, 1000 * 2 ** p.failed)) : ms;
    p.timer = setTimeout(p.run, wait);
  };
  p.now = () => { if (p.running) p.again = true; else p.run(); };
  pollers.push(p);
  return p;
}
document.addEventListener('visibilitychange', () => { if (!document.hidden) pollers.forEach((p) => p.now()); });

// A list is not re-rendered under the user: while a select in it has the
// focus, or a pointer is pressed in it (a replaced button loses its click).
// A skipped render is done once the press or the select is over.
function interacting(el) {
  if (!el) return false;
  const a = document.activeElement;
  const busy = !!el.dataset.pressed || !!(a && a.tagName === 'SELECT' && el.contains(a));
  if (busy) el._stale = true;
  return busy;
}
function guardPress(el, rerender) {
  const again = () => { if (el._stale && !interacting(el)) { el._stale = false; rerender(); } };
  el.addEventListener('pointerdown', () => { el.dataset.pressed = '1'; });
  const up = () => setTimeout(() => { delete el.dataset.pressed; again(); }, 250);
  el.addEventListener('pointerup', up);
  el.addEventListener('pointercancel', up);
  el.addEventListener('pointerleave', () => { if (el.dataset.pressed) up(); });
  el.addEventListener('focusout', () => setTimeout(again, 0));
}
// setHTML replaces an element's content only when it changed.
function setHTML(el, html) {
  if (el && el._html !== html) { el.innerHTML = html; el._html = html; }
}

/* ───────────── Theme ───────────── */
const mqDark = window.matchMedia('(prefers-color-scheme: dark)');
let themePref = store.get('apm-theme', 'system');
const themeListeners = [];
function isDark() {
  const t = document.documentElement.dataset.theme;
  return t === 'dark' || (!t && mqDark.matches);
}
function applyTheme(pref) {
  themePref = pref;
  if (pref === 'light' || pref === 'dark') document.documentElement.dataset.theme = pref;
  else delete document.documentElement.dataset.theme;
  store.set('apm-theme', pref);
  $$('[data-theme-set]').forEach((b) => b.setAttribute('aria-checked', String(b.dataset.themeSet === pref)));
  themeListeners.forEach((f) => f());
}
mqDark.addEventListener('change', () => { if (themePref === 'system') themeListeners.forEach((f) => f()); });
// cssVar reads a colour token, for what CSS classes cannot style (canvas).
const cssVar = (name) => getComputedStyle(document.documentElement).getPropertyValue(name).trim() || '#888';

/* ───────────── Toast ───────────── */
let toastTimer = 0;
function toast(msg, kind) {
  const t = $('toast');
  t.textContent = msg;
  t.classList.toggle('toast--err', kind === 'err');
  t.hidden = false;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { t.hidden = true; }, kind === 'err' ? 5000 : 2800);
}

// speakEnglish says text in an English voice: ATC's language whatever the
// browser's locale (a Czech system's default voice reads it as Czech).
function speakEnglish(text) {
  try {
    speechSynthesis.cancel();
    const u = new SpeechSynthesisUtterance(text);
    u.lang = 'en-GB';
    const voices = speechSynthesis.getVoices();
    const v = voices.find((x) => x.lang === 'en-GB') || voices.find((x) => x.lang === 'en-US') || voices.find((x) => /^en/i.test(x.lang));
    if (v) u.voice = v;
    u.rate = 0.95;
    speechSynthesis.speak(u);
  } catch { /* no speech */ }
}

/* ───────────── Geometry ───────────── */
// offset moves a lat/lon by meters east and north (local flat earth).
function offset(lat, lon, east, north) {
  const R = 6378137;
  return [lat + (north / R) * 180 / Math.PI, lon + (east / (R * Math.cos(lat * Math.PI / 180))) * 180 / Math.PI];
}
function dist(a, b) {
  const R = 6378137, toR = Math.PI / 180;
  const x = (b[1] - a[1]) * toR * Math.cos((a[0] + b[0]) / 2 * toR), y = (b[0] - a[0]) * toR;
  return Math.hypot(x, y) * R;
}
// smoothLine rounds a polyline's corners for drawing (Chaikin: each pass
// cuts every corner at a quarter of its legs); the ends stay.
function smoothLine(pts, passes = 3) {
  for (let n = 0; n < passes && pts.length > 2; n++) {
    const out = [pts[0]];
    for (let i = 0; i + 1 < pts.length; i++) {
      const [a, b] = [pts[i], pts[i + 1]];
      if (i > 0) out.push([0.75 * a[0] + 0.25 * b[0], 0.75 * a[1] + 0.25 * b[1]]);
      if (i + 2 < pts.length) out.push([0.25 * a[0] + 0.75 * b[0], 0.25 * a[1] + 0.75 * b[1]]);
    }
    out.push(pts[pts.length - 1]);
    pts = out;
  }
  return pts;
}
const flightLevel = (altFt) => altFt >= 5500 ? 'FL' + String(Math.round(altFt / 100)).padStart(3, '0') : `${Math.round(altFt / 100) * 100} ft`;
const hhmm = (t) => t ? new Date(t).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : '';

/* ───────────── Airport data helpers ───────────── */
const RWY_DESIG = ['', 'L', 'R', 'C', 'W', 'A', 'B'];
const RWY_COMPASS = { 37: 'N', 38: 'NE', 39: 'E', 40: 'SE', 41: 'S', 42: 'SW', 43: 'W', 44: 'NW' };
// TAXI_PARKING NAME enum as in the BGL spec — a best guess, raw value alongside.
const PARKING_NAMES = ['', 'P', 'N', 'NE', 'E', 'SE', 'S', 'SW', 'W', 'NW', 'GATE', 'DOCK'];
function rwyEnd(num, des) {
  const n = RWY_COMPASS[num] || (num > 0 ? String(num).padStart(2, '0') : '?');
  return n + (RWY_DESIG[des] || '');
}
function parkingLabel(p) {
  let prefix = PARKING_NAMES[p.name];
  if (prefix === undefined) prefix = p.name >= 12 && p.name <= 37 ? String.fromCharCode(65 + p.name - 12) : `?${p.name}`;
  if (prefix === 'GATE' || prefix === 'P') prefix = '';
  return prefix + p.number + (p.suffix >= 12 && p.suffix <= 37 ? String.fromCharCode(65 + p.suffix - 12) : '');
}
// The runway ends of the loaded airport, as names ("06", "24L").
const runwayEnds = () => (data ? data.runways.flatMap((r) => [rwyEnd(r.primaryNumber, r.primaryDesignator), rwyEnd(r.secondaryNumber, r.secondaryDesignator)]) : []);

// The loaded airport (GET /api/airport, normalised), null before.
let data = null;
