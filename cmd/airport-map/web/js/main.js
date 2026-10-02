/* Tabs, the bottom sheet, popovers, keys, loading an airport, the review
   overlay, polling and boot. */
'use strict';

/* ───────────── Tabs ───────────── */
const TABS = ['traffic', 'sequence', 'schedule', 'radio', 'airport', 'map'];
// The previous page's tab names, so a remembered tab carries over.
const OLD_TABS = { approach: 'sequence', charts: 'airport', layers: 'map', help: 'traffic' };
let currentTab = store.get('apm-tab') || OLD_TABS[store.get('airportMapTab')] || store.get('airportMapTab') || 'traffic';
function showTab(name, fromUser) {
  if (!TABS.includes(name)) name = 'traffic';
  currentTab = name;
  $$('.tabs [role="tab"]').forEach((b) => {
    const on = b.dataset.tab === name;
    b.setAttribute('aria-selected', String(on));
    b.tabIndex = on ? 0 : -1;
  });
  $$('.tabpanel').forEach((p) => { p.hidden = p.dataset.panel !== name; });
  store.set('apm-tab', name);
  if (fromUser && isPhone()) {
    if ($('panel').dataset.sheet === 'peek') setSheet('half');
    if (ctlSelected) select(0);
  }
  if (fromUser && document.body.classList.contains('panel-hidden')) togglePanel(true);
  refreshSection();
}
// refreshSection fills a section that just became visible.
function refreshSection() {
  if (currentTab === 'radio') { renderRadio(); radioPoll.now(); }
  if (currentTab === 'sequence') approachPoll.now();
  if (currentTab === 'schedule') { renderBoard(); schedulePoll.now(); }
  if (currentTab === 'airport') procPoll.now();
  if (currentTab === 'map') { renderOthers(lastTraffic); trafficPoll.now(); }
}

/* ───────────── Bottom sheet (phone) and the panel ───────────── */
function setSheet(state) {
  const p = $('panel');
  p.dataset.sheet = state;
  p.style.height = '';
  $('sheetHandle').setAttribute('aria-expanded', String(state !== 'peek'));
  if (state !== 'peek') refreshSection();
}
function initSheet() {
  const h = $('sheetHandle'), p = $('panel');
  const cycle = { peek: 'half', half: 'full', full: 'peek' };
  let y0 = 0, h0 = 0, dragging = false, moved = false;
  h.addEventListener('pointerdown', (e) => {
    if (!isPhone()) return;
    dragging = true; moved = false; y0 = e.clientY; h0 = p.getBoundingClientRect().height;
    h.setPointerCapture(e.pointerId);
    p.classList.add('is-dragging');
  });
  h.addEventListener('pointermove', (e) => {
    if (!dragging) return;
    const dy = y0 - e.clientY;
    if (Math.abs(dy) > 6) moved = true;
    p.style.height = Math.max(110, Math.min(window.innerHeight - 56, h0 + dy)) + 'px';
  });
  const end = () => {
    if (!dragging) return;
    dragging = false;
    p.classList.remove('is-dragging');
    if (!moved) { setSheet(cycle[p.dataset.sheet]); return; }
    const hh = p.getBoundingClientRect().height, vh = window.innerHeight;
    const snaps = { peek: 112, half: vh * 0.46 + 60, full: vh - 68 };
    setSheet(Object.entries(snaps).sort((a, b) => Math.abs(a[1] - hh) - Math.abs(b[1] - hh))[0][0]);
  };
  h.addEventListener('pointerup', end);
  h.addEventListener('pointercancel', end);
  h.addEventListener('keydown', (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setSheet(cycle[p.dataset.sheet]); } });
}
function updateSheetSummary() {
  const nWait = ctlViews.filter((v) => !isDone(v) && waits(v)).length;
  const ours = ctlViews.filter((v) => !isDone(v)).length;
  const rwy = $('rwyChipVal').textContent;
  $('sheetSummary').textContent = `${nWait} waiting · ${ours} ours${rwy && rwy !== '—' ? ' · RWY ' + rwy : ''}${lastOwn && lastOwn.paused ? ' · paused' : ''}${simLive ? '' : ' · no simulator'}`;
}
function togglePanel(show) {
  const hide = show === undefined ? !document.body.classList.contains('panel-hidden') : !show;
  document.body.classList.toggle('panel-hidden', hide);
  const b = $('panelBtn');
  b.setAttribute('aria-pressed', String(!hide));
  b.setAttribute('aria-label', hide ? 'Show the panel' : 'Hide the panel');
  b.title = hide ? 'Show the panel' : 'Hide the panel';
  setTimeout(() => map.invalidateSize(), 50);
  if (!hide) refreshSection();
}

// setDock docks the panel left, right or at the bottom (Map → Panel), kept
// on this device. A phone always has the bottom sheet.
function setDock(d) {
  if (!['left', 'right', 'bottom'].includes(d)) d = 'right';
  document.body.dataset.dock = d;
  store.set('airportMapDock', d);
  $$('#dockSeg input').forEach((i) => { i.checked = i.value === d; });
  setTimeout(() => map.invalidateSize(), 50);
}

/* ───────────── Popovers, help ───────────── */
function closePopovers(except) {
  $$('.popover').forEach((p) => { if (p !== except) p.hidden = true; });
  $$('[aria-haspopup="dialog"][aria-controls]').forEach((b) => { if (!except || b.getAttribute('aria-controls') !== except.id) b.setAttribute('aria-expanded', 'false'); });
}
function openHelp() {
  closePopovers();
  const d = $('help');
  if (d.showModal) d.showModal(); else d.setAttribute('open', '');
}

/* ───────────── Loading an airport ───────────── */
// normalize flattens a pkg/airport Layout into the shape the drawing code
// uses: lat/lon on every item and the runway end fields.
function normalize(d) {
  // A small airport may have no parking (or runways) at all: null, not [].
  d.parking = d.parking || [];
  d.taxiPoints = d.taxiPoints || [];
  d.runways = d.runways || [];
  for (const p of [...d.parking, ...d.taxiPoints]) { p.lat = p.position.lat; p.lon = p.position.lon; }
  for (const r of d.runways) {
    r.lat = r.center.lat; r.lon = r.center.lon;
    r.primaryNumber = r.primary.number; r.primaryDesignator = r.primary.designator;
    r.secondaryNumber = r.secondary.number; r.secondaryDesignator = r.secondary.designator;
  }
  d.taxiPaths = d.taxiPaths || [];
  d.taxiNames = d.taxiNames || [];
  return d;
}
// fitAirport shows the whole airport: its taxiways and stands, as on load.
function fitAirport() {
  if (!data) return;
  const pts = [...data.taxiPoints, ...data.parking].map((p) => [p.lat, p.lon]);
  if (pts.length) map.fitBounds(L.latLngBounds(pts).pad(0.05));
  else map.setView([data.lat, data.lon], 14);
}
async function load(icao, refresh) {
  icao = icao.trim().toUpperCase();
  if (!icao) return;
  const st = $('aptStatus');
  st.className = 'small';
  st.textContent = `Loading ${icao}…`;
  $('aptCode').textContent = icao;
  const r = await api(`/api/airport?icao=${encodeURIComponent(icao)}${refresh ? '&refresh=1' : ''}`);
  if (!r.ok) {
    st.className = 'small err-text';
    st.textContent = r.error;
    $('aptName').textContent = data ? data.name : 'not loaded';
    if (data) $('aptCode').textContent = data.icao;
    toast(`${icao}: ${r.error}`, 'err');
    return;
  }
  const prev = data && data.icao;
  data = normalize(r.data);
  if (prev !== data.icao) { activeUse = null; routeFrom = null; viaPoints = []; drawVia(); $('rFrom').textContent = '—'; $('rFromHint').textContent = 'click a stand on the map'; }
  $('aptCode').textContent = data.icao;
  $('aptName').textContent = data.name;
  $('icao').value = data.icao;
  drawAirport();
  loadTower();
  renderTypeTables();
  $('info').innerHTML = `${data.runways.length} runways · ${data.parking.length} parking · ${data.taxiPoints.length} points · ${data.taxiPaths.length} paths · ${data.taxiNames.length} names` +
    (unresolvedPaths ? `<br><span class="err-text">${unresolvedPaths} paths with an out-of-range endpoint (red rings)</span>` : '') +
    `<br>Fetched ${new Date(data.fetchedAt).toLocaleString()}`;
  st.textContent = `Loaded ${data.icao}: ${data.runways.length} runways, ${data.parking.length} stands, ${data.taxiPoints.length} taxi points`;
  fitAirport();
  history.replaceState(null, '', `?icao=${data.icao}`);
  fillRunwayEnds();
  resetProcedures();
  loadChoices();
  renderRadio();
  airportInfoPoll.now();
  standsPoll.now();
  approachPoll.now();
  procPoll.now();
}

/* ───────────── The review overlay ───────────── */
// ?overlay=LKPR-A1-24-after: a review overlay (GET /api/overlay) — the push
// in red, the tow, the start of the taxi in blue, the target pose — fitted
// on the map with its legend (playwright/shots2.mjs screenshots it).
async function showOverlay(name) {
  if (!name) return;
  const r = await api(`/api/overlay?name=${encodeURIComponent(name)}`);
  if (!r.ok || !r.data) { toast(`Overlay ${name}: ${r.error || 'empty'}`, 'err'); return; }
  const geo = typeof r.data === 'string' ? JSON.parse(r.data) : r.data;
  const style = (f) => ({ push: { color: '#ff3030', weight: 5 }, tow: { color: '#ff9d1e', weight: 5, dashArray: '8 6' }, target: { color: '#39ff6a', weight: 3 } })[f.properties.kind] || { color: '#2f8cff', weight: 5 };
  const layer = L.geoJSON(geo, {
    style,
    pointToLayer: (f, ll) => f.properties.kind === 'pose'
      ? L.marker(ll, { zIndexOffset: 900, icon: L.divIcon({ className: '', iconSize: [30, 30], iconAnchor: [15, 15],
        html: `<svg viewBox="0 0 24 24" width="30" height="30" style="transform:rotate(${f.properties.heading}deg)"><path fill="#39ff6a" stroke="#000" stroke-width=".8" d="M12 1.8c.85 0 1.35.95 1.35 2.1v5.3l7.85 4.55v2.05l-7.85-2.35v4.7l2.25 1.75v1.35L12 20.4l-3.6.85V19.9l2.25-1.75v-4.7L2.8 15.8v-2.05l7.85-4.55V3.9c0-1.15.5-2.1 1.35-2.1z"/></svg>` }) })
        .bindTooltip(`target ${Math.round(f.properties.heading)}°`, { direction: 'left' })
      : L.circleMarker(ll, { radius: 7, color: '#fff', fillColor: '#ffcc00', fillOpacity: 1 })
        .bindTooltip(`${esc(f.properties.label)} (${Math.round(f.properties.heading)}°)`, { permanent: true, direction: 'right' }),
  }).addTo(map);
  if (layer.getBounds().isValid()) map.fitBounds(layer.getBounds(), { padding: [60, 60], maxZoom: 18 });
  const tag = $('overlayTag');
  tag.innerHTML = `${esc(name)} — <span style="color:#ff5050">push</span> · <span style="color:#ff9d1e">tow</span> · <span style="color:#4a9cff">taxi start</span> · <span style="color:#39ff6a">target pose (to scale)</span>`;
  tag.hidden = false;
}

/* ───────────── Polling ───────────── */
const aircraftPoll = poller('aircraft', pollAircraft, () => 1000);
const trafficPoll = poller('traffic', pollTraffic, () => (layerOn.ours || layerOn.others || layerOn.safe || tabVisible('map') ? 1000 : 0));
// Push (server-sent events, GET /api/events): the server says what changed
// and it is fetched at once. The control poll stays at a second (it brings
// our aircraft's positions, tugs and routes); the radio polls, only a
// fallback while the stream is up.
let pushUp = false;
const controlPoll = poller('control', pollControl, () => 1000);
// These answer 503 without the simulator: polled only while it is there
// (my aircraft reported, or one of them answered lately).
const standsPoll = poller('stands', pollStands, () => (simLive && data && layerOn.occupied ? 3000 : 0));
// Faster while the tower's angle is drawn: the cone follows the turning.
const cameraPoll = poller('camera', pollCamera, () => (!simLive ? 0 : showAngle && camView && camView.mode === 'tower' ? 500 : 2000));
const gamePoll = poller('game', pollGame, () => (simLive ? 2000 : 0));
const schedulePoll = poller('schedule', pollSchedule, () => (!simLive ? 0 : tabVisible('schedule') ? 3000 : 10000));
const worldPoll = poller('world', pollWorld, () => (!simLive ? 0 : worldOn || tabVisible('map') ? 5000 : 15000));
const procPoll = poller('procedures', pollProcedures, () => (simLive && data && !procs && procTries < 20 && tabVisible('airport') ? 3000 : 0));
const airportInfoPoll = poller('airportinfo', pollAirportInfo, () => (data ? 10000 : 0));
const approachPoll = poller('approach', pollApproach, () => (data && seqWanted() ? (tabVisible('sequence') ? 2000 : 4000) : 0));
// Play on this device (#511): the calls on the frequency followed, as the
// server's voice says them, through this browser.
// Every call seen is marked, on whatever frequency: switching frequency
// plays what is said from then on, not what was said there before. Each
// clip is fetched (and synthesised by the server) as soon as it is queued,
// tried again once, and played in order; a call older than HERE_STALE_MS
// behind the newest (a sleeping tablet waking up) is skipped.
const HERE_STALE_MS = 20000;
const hereQueue = []; // {freq, clip: Promise<object URL or null>}
const herePlayed = new Set();
let hereAudio = null, herePlaying = false, hereWatch = 0, hereBlockedSaid = false;
async function hereFetch(url) {
  for (let i = 0; i < 2; i++) {
    try {
      const res = await fetch(url, { headers: atcPosition && atcPosition !== 'all' ? { 'X-ATC-Position': atcPosition } : {} });
      if (res.ok) return URL.createObjectURL(await res.blob());
      if (res.status === 404) return null; // gone from the server's recent calls
    } catch { /* the network: again */ }
    await new Promise((ok) => setTimeout(ok, 700));
  }
  return null;
}
function hereDone() {
  clearTimeout(hereWatch);
  if (hereAudio && hereAudio.src.startsWith('blob:')) URL.revokeObjectURL(hereAudio.src);
  herePlaying = false;
  hereNext();
}
async function hereNext() {
  if (herePlaying || !hereQueue.length || !hereAudio) return;
  herePlaying = true;
  const item = hereQueue.shift();
  const src = await item.clip;
  // Retuned meanwhile, or no clip: on to the next.
  if (!src || item.freq !== rdFreq || !$('rdHere').checked) { if (src) URL.revokeObjectURL(src); herePlaying = false; hereNext(); return; }
  hereAudio.src = src;
  // A clip that never ends (a stalled element) does not stop the rest.
  hereWatch = setTimeout(hereDone, 30000);
  hereAudio.play().catch((err) => {
    if (err && err.name === 'NotAllowedError' && !hereBlockedSaid) {
      hereBlockedSaid = true;
      toast('The browser blocked the sound: tap Play on this device again', 'err');
    }
    hereDone();
  });
}
async function pollHere() {
  if (!data) return true;
  const r = await api(`/api/radio?icao=${encodeURIComponent(data.icao)}&n=20`);
  if (!r.ok) return false;
  const list = r.data || [];
  const newest = Math.max(0, ...list.map((t) => Date.parse(t.at) || 0));
  for (const t of list) {
    const key = `${t.at} ${t.callsign} ${t.intent}`;
    if (herePlayed.has(key)) continue;
    herePlayed.add(key);
    if (t.frequency !== rdFreq || t.intent === 'atis' || newest - Date.parse(t.at) > HERE_STALE_MS) continue;
    const url = `/api/voice/clip?icao=${encodeURIComponent(data.icao)}&at=${encodeURIComponent(t.at)}&cs=${encodeURIComponent(t.callsign)}&intent=${encodeURIComponent(t.intent)}`;
    hereQueue.push({ freq: t.frequency, clip: hereFetch(url) });
  }
  // Bounded: the oldest marks go (a Set keeps the order they came in).
  for (const k of herePlayed) { if (herePlayed.size <= 500) break; herePlayed.delete(k); }
  hereNext();
  return true;
}
const herePoll = poller('here', pollHere, () => (!$('rdHere').checked || !simLive ? 0 : pushUp ? 5000 : 1000));
$('rdHere').addEventListener('change', (e) => {
  if (voice) showVoice(voice); // the output picker: not for this device
  if (e.target.checked && isLocalHost && rdSoundOn) {
    setVoice(false); // the map's own voice would say it twice here
    toast('Playing in this browser: the map voice on this computer is off (it would echo)');
  }
  if (e.target.checked) {
    // Made in the click, so the browser lets it play.
    hereAudio = hereAudio || new Audio();
    hereAudio.onended = hereAudio.onerror = hereDone;
    hereBlockedSaid = false;
    // What was said before: not played. (The server's times, not this
    // device's clock, which can be off.)
    api(`/api/radio?icao=${encodeURIComponent(data ? data.icao : '')}&n=20`).then((r) => {
      for (const t of (r.ok && r.data) || []) herePlayed.add(`${t.at} ${t.callsign} ${t.intent}`);
      herePoll.now();
    });
  } else {
    hereQueue.length = 0;
    if (hereAudio) hereAudio.pause();
    clearTimeout(hereWatch);
    herePlaying = false;
  }
});
// The position this device works (#511).
// Two pickers: in the strip, and in the More menu on narrower screens.
$('asSel').value = $('asSelMore').value = atcPosition;
['asSel', 'asSelMore'].forEach((id) => $(id).addEventListener('change', (e) => {
  atcPosition = e.target.value;
  renderStrips(); renderCtx(); // other frequencies greyed out
  $('asSel').value = $('asSelMore').value = atcPosition;
  try { localStorage.setItem('apm-as', atcPosition); } catch { /* private window */ }
  toast(atcPosition ? `Working as ${e.target.selectedOptions[0].textContent}: other frequencies are read-only` : 'Working all positions');
}));
// The simulator connection, also without an aircraft (the main menu).
// isLocalHost: this browser runs on the map's own computer.
const isLocalHost = ['127.0.0.1', 'localhost', '::1', '[::1]'].includes(location.hostname);
let netShown = '';
const statusPoll = poller('status', async () => {
  const r = await api('/api/status');
  if (r.ok && r.data && r.data.connected) markLive();
  if (r.ok && r.data) {
    // Where other devices open the map (network play, #511).
    const urls = r.data.network || [];
    // Spectating (a view token): watch only, the controls hidden (the
    // server refuses them anyway).
    const spect = r.data.role === 'spectator';
    document.body.classList.toggle('spectator', spect);
    $('spectChip').hidden = !spect;
    // With tokens, the host sees the links to give out (each carries its token).
    const links = r.data.links || {};
    const linkList = (role, label) => (links[role] || []).map((u) => `${label}: <a href="${esc(u)}" target="_blank" rel="noopener">${esc(u)}</a>`).join('<br>');
    const html = urls.length
      ? (r.data.tokens && isLocalHost
        ? `On the network, give out a link: ${[linkList('control', 'control'), linkList('spectator', 'watch only')].filter(Boolean).join('<br>')}`
        : `On the network: ${urls.map((u) => `<a href="${esc(u)}/?icao=${data ? esc(data.icao) : ''}" target="_blank" rel="noopener">${esc(u)}</a>`).join(', ')}: open it on a tablet, laptop or phone and pick <b>As</b>.`)
      : 'Only this computer can open the map. To play over the network, start it with <code>-addr :8080</code> and open the address shown here on the other devices.';
    if (html !== netShown) {
      netShown = html;
      $('netInfo').innerHTML = html;
      $('moreNet').innerHTML = urls.length ? urls.map(esc).join('<br>') : 'this computer only';
      $('connChip').dataset.net = urls.join(', ');
    }
  }
  return r.ok;
}, () => 3000);
const radioPoll = poller('radio', pollRadio, () => (data && radioWanted() ? (pushUp ? 10000 : tabVisible('radio') ? 1000 : 2000) : 0));
(function push() {
  if (!window.EventSource) return;
  const es = new EventSource('/api/events');
  es.onopen = () => { pushUp = true; };
  es.onerror = () => { pushUp = false; }; // the browser reconnects by itself (retry: 3 s)
  es.addEventListener('control', () => { controlPoll.now(); if (seqWanted()) approachPoll.now(); });
  es.addEventListener('radio', () => { if (radioWanted()) radioPoll.now(); if ($('rdHere').checked) herePoll.now(); });
})();
const voicePoll = poller('voice', pollVoice, () => (rdSync ? 1000 : 10000));

/* ───────────── Boot ───────────── */
function boot() {
  applyTheme(themePref);
  $$('[data-theme-set]').forEach((b) => b.addEventListener('click', () => applyTheme(b.dataset.themeSet)));
  setBase(baseMode);
  initLayers();
  zoomClasses();
  initDirector();
  initTower();
  initTowerMouse();
  initTraffic();
  initSections();
  initSheet();
  fillRunwayEnds();
  fillProcedurePick();
  updateSpawn();
  updateConn();
  showSim();
  renderStrips();

  $$('.tabs [role="tab"]').forEach((b) => b.addEventListener('click', () => showTab(b.dataset.tab, true)));
  document.querySelector('.tabs').addEventListener('keydown', (e) => {
    const tabs = $$('.tabs [role="tab"]');
    const i = tabs.findIndex((t) => t.dataset.tab === currentTab);
    let j = -1;
    if (e.key === 'ArrowRight') j = (i + 1) % tabs.length;
    if (e.key === 'ArrowLeft') j = (i - 1 + tabs.length) % tabs.length;
    if (j >= 0) { e.preventDefault(); showTab(tabs[j].dataset.tab, true); tabs[j].focus(); }
  });
  document.addEventListener('click', (e) => {
    const g = e.target.closest('[data-goto]');
    if (g) { closePopovers(); showTab(g.dataset.goto, true); }
    if (e.target.closest('[data-open-help]')) openHelp();
    if (e.target.closest('[data-close-help]')) $('help').close();
  });
  $$('[aria-haspopup="dialog"][aria-controls]').forEach((btn) => btn.addEventListener('click', (e) => {
    e.stopPropagation();
    const pop = $(btn.getAttribute('aria-controls'));
    const open = pop.hidden;
    closePopovers(pop);
    pop.hidden = !open;
    btn.setAttribute('aria-expanded', String(open));
    if (open && !isPhone()) { const first = pop.querySelector('input, button, select'); if (first) first.focus(); }
  }));
  document.addEventListener('click', (e) => {
    if (!document.contains(e.target)) return; // re-rendered inside a popover
    if (!e.target.closest('.popover') && !e.target.closest('[aria-haspopup]')) closePopovers();
  });
  $('aptForm').addEventListener('submit', (e) => { e.preventDefault(); load($('icao').value); });
  $('aptInRange').addEventListener('click', (e) => {
    const b = e.target.closest('[data-apt]');
    if (b) { $('icao').value = b.dataset.apt; load(b.dataset.apt); }
  });
  $('aptRefresh').addEventListener('click', () => load($('icao').value || (data ? data.icao : ''), true));

  $('zoomIn').addEventListener('click', () => map.zoomIn());
  $('zoomOut').addEventListener('click', () => map.zoomOut());
  $('fitAirport').addEventListener('click', fitAirport);
  $('lostDismiss').addEventListener('click', () => { lostDismissed = true; updateLost(); });
  $('locateMe').addEventListener('click', () => {
    if (lastOwn) locate(lastOwn.lat, lastOwn.lon, 'You');
    else toast('Your aircraft is not reported (simulator not connected?)');
  });
  $('worldBtn').addEventListener('click', toggleWorld);
  $('panelBtn').addEventListener('click', () => togglePanel());
  setDock(store.get('airportMapDock') || 'right');
  $('dockSeg').addEventListener('change', (e) => setDock(e.target.value));
  // Full screen (browser), with the panel: play without the browser around it.
  $('fsBtn').addEventListener('click', () => {
    try {
      if (document.fullscreenElement) document.exitFullscreen();
      else if (document.documentElement.requestFullscreen) document.documentElement.requestFullscreen().catch(() => toast('Full screen is not available here'));
    } catch { /* not allowed here */ }
  });
  document.addEventListener('fullscreenchange', () => {
    const on = !!document.fullscreenElement, b = $('fsBtn');
    b.innerHTML = icon(on ? 'i-shrink' : 'i-expand');
    b.title = on ? 'Leave full screen (Esc)' : 'Full screen, panel included (Esc leaves)';
    b.setAttribute('aria-label', on ? 'Leave full screen' : 'Full screen');
    setTimeout(() => map.invalidateSize(), 100);
  });

  document.addEventListener('keydown', (e) => {
    const typing = e.target.closest('input, select, textarea');
    if (e.key === 'Escape') {
      if ($$('.popover').some((p) => !p.hidden)) { closePopovers(); return; }
      if ($('help').open) return; // the dialog closes itself
      if (typing) return;
      // Stop following, deselect, and hide the planned route.
      if (ctlSelected || ctlFollow) { ctlFollow = 0; select(0); }
      if (routeFrom) hideRoute();
      if (isPhone() && $('panel').dataset.sheet !== 'peek') setSheet('peek');
      return;
    }
    if (typing || e.ctrlKey || e.metaKey || e.altKey || $('help').open) return;
    if (/^[1-6]$/.test(e.key)) showTab(TABS[Number(e.key) - 1], true);
  });
  window.addEventListener('resize', () => { if (!isPhone()) $('panel').style.height = ''; });
  $('ctlLogBox').addEventListener('toggle', () => { if ($('ctlLogBox').open) controlPoll.now(); });

  setInterval(tickWaits, 1000);
  requestAnimationFrame(glideTraffic);
  if (isPhone()) setSheet('peek');
  showTab(currentTab);

  const params = new URLSearchParams(location.search);
  const initial = params.get('icao') || 'LKPR';
  // Read before load: loading an airport rewrites the URL.
  const overlay = params.get('overlay');
  $('icao').value = initial;
  load(initial).then(() => showOverlay(overlay));
  pollers.forEach((p) => p.run());
}
boot();

// Keep the screen on (a tablet working a position): on by default on touch
// devices, remembered; taken again when the page comes back into view.
let wakeLock = null;
async function keepAwake() {
  const want = $('wakeLock').checked && document.visibilityState === 'visible';
  if (!('wakeLock' in navigator)) { $('wakeLock').disabled = true; return; }
  try {
    if (want && !wakeLock) {
      wakeLock = await navigator.wakeLock.request('screen');
      wakeLock.addEventListener('release', () => { wakeLock = null; });
    } else if (!want && wakeLock) {
      await wakeLock.release();
      wakeLock = null;
    }
  } catch { /* not allowed now (no user gesture, low battery): tried again later */ }
}
{
  const saved = store.get('airportMapWakeLock');
  $('wakeLock').checked = saved === null ? matchMedia('(pointer: coarse)').matches : saved === '1';
  $('wakeLock').addEventListener('change', () => { store.set('airportMapWakeLock', $('wakeLock').checked ? '1' : '0'); keepAwake(); });
  document.addEventListener('visibilitychange', keepAwake);
  document.addEventListener('pointerdown', () => { if (!wakeLock) keepAwake(); }, { passive: true });
  keepAwake();
}

// Installable as an app: a service worker, where the browser allows one
// (https or this computer).
if ('serviceWorker' in navigator && window.isSecureContext) {
  navigator.serviceWorker.register('sw.js').catch(() => { /* still works as a page */ });
}
