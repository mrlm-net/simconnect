/* The map: base tiles, the airport, aircraft, routes, the final, world view.
   Styled overlays are SVG with CSS classes bound to the tokens, so a theme
   switch restyles them without a redraw. The raw taxi network (thousands of
   paths and points) stays on one canvas and is redrawn on a theme switch. */
'use strict';

const map = L.map('map', { zoomControl: false, attributionControl: false, maxZoom: 21, zoomSnap: 0.5, zoomDelta: 0.5 }).setView([50.1, 14.26], 14);
// Aircraft and their data tags above the taxiway signs and other labels
// (tooltips, 650), below popups (700).
map.createPane('aircraft').style.zIndex = 660;
L.control.attribution({ position: 'bottomleft', prefix: false }).addTo(map);
const canvas = L.canvas({ padding: 0.5, tolerance: 4 });

/* ───────────── Base tiles ───────────── */
// Esri's grey canvas follows the theme and needs no key (CARTO's light_all /
// dark_all answered "API key required" here, even served over http).
const OSM_ATTR = '&copy; OpenStreetMap contributors';
const ESRI_ATTR = 'Tiles &copy; Esri &mdash; Esri, HERE, Garmin, &copy; OpenStreetMap contributors';
const tiles = {
  light: L.tileLayer('https://server.arcgisonline.com/ArcGIS/rest/services/Canvas/World_Light_Gray_Base/MapServer/tile/{z}/{y}/{x}', { maxZoom: 21, maxNativeZoom: 16, attribution: ESRI_ATTR }),
  dark: L.tileLayer('https://server.arcgisonline.com/ArcGIS/rest/services/Canvas/World_Dark_Gray_Base/MapServer/tile/{z}/{y}/{x}', { maxZoom: 21, maxNativeZoom: 16, attribution: ESRI_ATTR }),
  osm: L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', { maxZoom: 21, maxNativeZoom: 19, attribution: OSM_ATTR }),
  sat: L.tileLayer('https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}', { maxZoom: 21, maxNativeZoom: 19, attribution: 'Imagery &copy; Esri' }),
};
let baseMode = store.get('apm-base', 'map');
let currentTiles = null;
function setTiles() {
  const want = baseMode === 'sat' ? tiles.sat : baseMode === 'osm' ? tiles.osm : isDark() ? tiles.dark : tiles.light;
  map.getContainer().classList.toggle('tiles-dim', baseMode === 'osm' && isDark());
  if (want === currentTiles) return;
  if (currentTiles) map.removeLayer(currentTiles);
  want.addTo(map);
  currentTiles = want;
}
function setBase(mode) {
  baseMode = ['map', 'osm', 'sat'].includes(mode) ? mode : 'map';
  store.set('apm-base', baseMode);
  $$('#baseSeg input').forEach((i) => { i.checked = i.value === baseMode; });
  setTiles();
}

/* ───────────── Layers and their switches ───────────── */
const layers = {
  runways: L.layerGroup(), parking: L.layerGroup(), points: L.layerGroup(), holds: L.layerGroup(), labels: L.layerGroup(),
  overlaps: L.layerGroup(), occupied: L.layerGroup(), pads: L.layerGroup(), procs: L.layerGroup(), final: L.layerGroup(),
  routes: L.layerGroup(), choices: L.layerGroup(), preview: L.layerGroup(), via: L.layerGroup(),
  safe: L.layerGroup(), traffic: L.layerGroup(), world: L.layerGroup(), locate: L.layerGroup(), tugs: L.layerGroup(),
  circuits: L.layerGroup(), vfrpts: L.layerGroup(),
};
const PATH_TYPES = { 0: 'NONE', 1: 'TAXI', 2: 'RUNWAY', 3: 'PARKING', 4: 'PATH', 5: 'CLOSED', 6: 'VEHICLE', 7: 'ROAD', 8: 'PAINTEDLINE' };
// Drawing per path TYPE: colour token, weight, opacity, dashes.
const PATH_STYLE = {
  0: ['--text-3', 2, 0.8], 1: ['--twy', 3, 0.95], 2: ['--rwy-mark', 1.5, 0.7], 3: ['--twy', 1.5, 0.7], 4: ['--twy', 2, 0.6],
  5: ['--danger', 2, 0.7, '6 6'], 6: ['--ac-ovf', 1.5, 0.8], 7: ['--text-3', 1.5, 0.8], 8: ['--border-strong', 1, 0.9, '4 4'],
};
const POINT_TYPES = { 0: 'NONE', 1: 'NORMAL', 2: 'HOLD_SHORT', 3: '(3)', 4: 'ILS_HOLD_SHORT', 5: 'HOLD_SHORT_NO_DRAW', 6: 'ILS_HOLD_SHORT_NO_DRAW' };
const HOLD_SHORT = new Set([2, 4, 5, 6]);
const pathGroups = {};
for (const t of Object.keys(PATH_TYPES)) pathGroups[t] = L.layerGroup();

const LAYER_DEFAULTS = { ours: true, others: false, routes: true, final: true, labels: true, tags: true, safe: false, overlaps: true, occupied: true, runways: true, parking: true, holds: true, points: false, follow: false };
const layerOn = { ...LAYER_DEFAULTS, ...store.json('apm-layers', {}), follow: false };
const pathOn = { 1: true, 2: true, 3: true, 4: true, 5: true, ...store.json('apm-paths', {}) };
const LAYER_GROUPS = { runways: 'runways', parking: 'parking', holds: 'holds', points: 'points', labels: 'labels', overlaps: 'overlaps', occupied: 'occupied', final: 'final', safe: 'safe' };

function setLayer(name, on) {
  layerOn[name] = on;
  const saved = { ...layerOn };
  delete saved.follow;
  store.set('apm-layers', JSON.stringify(saved));
  const g = LAYER_GROUPS[name];
  if (g) { if (on) layers[g].addTo(map); else layers[g].remove(); }
  const c = map.getContainer().classList;
  if (name === 'tags') c.toggle('no-tags', !on);
  if (name === 'labels') c.toggle('no-labels', !on);
  $$(`[data-layer="${name}"]`).forEach((i) => { i.checked = on; });
}
function setPathType(t, on) {
  pathOn[t] = on;
  store.set('apm-paths', JSON.stringify(pathOn));
  if (on) pathGroups[t].addTo(map); else pathGroups[t].remove();
}
function initLayers() {
  // choices, preview and via come with the New flight form.
  for (const g of ['pads', 'procs', 'routes', 'traffic', 'locate', 'tugs', 'circuits', 'vfrpts']) layers[g].addTo(map);
  for (const name of Object.keys(LAYER_DEFAULTS)) setLayer(name, layerOn[name]);
  for (const t of Object.keys(PATH_TYPES)) if (pathOn[t]) pathGroups[t].addTo(map);
}

// Zoom classes: taxiway signs below 15, stand labels below 16, data tags below 13.
function zoomClasses() {
  const z = map.getZoom(), c = map.getContainer().classList;
  c.toggle('z-lt13', z < 13);
  c.toggle('z-lt15', z < 15);
  c.toggle('z-lt16', z < 16);
}
map.on('zoomend', zoomClasses);

/* ───────────── The airport ───────────── */
const pointLL = (i) => { const p = data.taxiPoints[i]; return p ? [p.lat, p.lon] : null; };
const parkingLL = (i) => { const p = data.parking[i]; return p ? [p.lat, p.lon] : null; };
// A path's START is a taxi point; its END a taxi point, or a parking spot
// for TYPE=3 (PARKING) paths (verified on LKPR, #247).
const endpoints = (path) => [pointLL(path.start), path.type === 3 ? parkingLL(path.end) : pointLL(path.end)];

let parkingCircles = new Map(); // parking index → circle
let unresolvedPaths = 0;
let overlapPairs = [];

function drawAirport() {
  for (const g of ['runways', 'parking', 'overlaps', 'occupied', 'final', 'routes']) layers[g].clearLayers();
  parkingCircles = new Map();
  routeLayers.clear();
  finalSig = '';
  occupiedSig = '';
  if (!data) return;
  for (const r of data.runways) {
    const h = r.heading * Math.PI / 180, hl = r.length / 2, hw = r.width / 2;
    const along = [Math.sin(h), Math.cos(h)], across = [Math.cos(h), -Math.sin(h)];
    const at = (s, t) => offset(r.lat, r.lon, along[0] * s + across[0] * t, along[1] * s + across[1] * t);
    const name = `${rwyEnd(r.primaryNumber, r.primaryDesignator)}/${rwyEnd(r.secondaryNumber, r.secondaryDesignator)}`;
    L.polygon([at(-hl, -hw), at(hl, -hw), at(hl, hw), at(-hl, hw)], { className: 'm-rwy' }).bindPopup(`<b>Runway ${esc(name)}</b><br>` + kv({
      index: r.index, heading: r.heading.toFixed(1) + '°', length: r.length.toFixed(0) + ' m', width: r.width.toFixed(0) + ' m',
      primary: `${r.primaryNumber}/${r.primaryDesignator}`, secondary: `${r.secondaryNumber}/${r.secondaryDesignator}`,
    })).addTo(layers.runways);
    L.polyline([at(-hl + 40, 0), at(hl - 40, 0)], { className: 'm-rwy-cl', interactive: false }).addTo(layers.runways);
    for (const [s, label] of [[-1, rwyEnd(r.primaryNumber, r.primaryDesignator)], [1, rwyEnd(r.secondaryNumber, r.secondaryDesignator)]]) {
      L.marker(at(s * (hl + 70), 0), { interactive: false, keyboard: false, icon: L.divIcon({ className: 'rwy-tag', html: `<span>${esc(label)}</span>`, iconSize: [34, 20], iconAnchor: [17, 10] }) }).addTo(layers.runways);
    }
  }
  overlapPairs = overlappingStands();
  const overlapsOf = new Map();
  for (const [a, b] of overlapPairs) {
    for (const [p, q] of [[a, b], [b, a]]) {
      if (!overlapsOf.has(p.index)) overlapsOf.set(p.index, []);
      overlapsOf.get(p.index).push(parkingLabel(q));
    }
  }
  for (const p of data.parking) {
    const label = parkingLabel(p);
    const c = L.circle([p.lat, p.lon], { radius: Math.max(p.radius, 3), className: 'm-park', bubblingMouseEvents: false })
      .bindTooltip(esc(label), { permanent: true, direction: 'center', className: 'stand-lbl' })
      .bindPopup(`<b>Parking ${esc(label)}</b><br>` + kv({
        index: p.index, name: p.name, number: p.number, type: p.type, heading: p.heading.toFixed(0) + '°',
        radius: p.radius.toFixed(1) + ' m', biasX: p.biasX.toFixed(1), biasZ: p.biasZ.toFixed(1),
      }) + (overlapsOf.has(p.index) ? `<br>overlaps ${esc(overlapsOf.get(p.index).join(', '))}` : ''), { autoPan: false })
      .on('click', () => pickStand(p))
      .addTo(layers.parking);
    parkingCircles.set(p.index, c);
  }
  for (const idx of overlapsOf.keys()) {
    const p = data.parking[idx];
    L.circle([p.lat, p.lon], { radius: Math.max(p.radius, 3), className: 'm-overlap', interactive: false }).addTo(layers.overlaps);
  }
  const n = new Set(overlapPairs.flat().map((p) => p.index)).size;
  $('overlapInfo').textContent = overlapPairs.length ? `· ${overlapPairs.length} pairs, ${n} stands` : '· none';
  drawNetwork();
  markPicked();
}

// Two stands overlap when their RADIUS circles intersect: both cannot hold
// a large aircraft at the same time.
function overlappingStands() {
  const pairs = [];
  const ps = data.parking.filter((p) => p.radius > 0);
  for (let i = 0; i < ps.length; i++) {
    for (let j = i + 1; j < ps.length; j++) {
      const a = ps[i], b = ps[j];
      if (dist([a.lat, a.lon], [b.lat, b.lon]) < a.radius + b.radius) pairs.push([a, b]);
    }
  }
  return pairs;
}

// drawNetwork draws the canvas part: taxi paths by TYPE, taxi points,
// hold-short points, and the taxiway signs. Again on a theme switch.
function drawNetwork() {
  for (const g of Object.values(pathGroups)) g.clearLayers();
  for (const g of ['labels', 'points', 'holds']) layers[g].clearLayers();
  if (!data) return;
  const colour = {};
  const col = (token) => colour[token] || (colour[token] = cssVar(token));
  const labelled = new Set();
  unresolvedPaths = 0;
  for (const p of data.taxiPaths) {
    const [a, b] = endpoints(p);
    const tname = PATH_TYPES[p.type] || `(${p.type})`;
    const name = data.taxiNames[p.nameIndex] || '';
    const pop = () => `<b>Taxi path #${p.index}</b><br>` + kv({
      type: `${p.type} ${tname}`, start: p.start, end: p.end, width: p.width.toFixed(1) + ' m',
      nameIndex: `${p.nameIndex} "${name}"`, runway: p.type === 2 ? rwyEnd(p.runwayNumber, p.runwayDesignator) : '—',
      length: a && b ? dist(a, b).toFixed(0) + ' m' : 'unresolved',
    });
    const group = pathGroups[p.type] || pathGroups[0];
    if (!a || !b) {
      unresolvedPaths++;
      const at = a || b;
      if (at) L.circleMarker(at, { renderer: canvas, radius: 5, color: col('--danger'), weight: 2, fill: false }).bindPopup(pop).addTo(group);
      continue;
    }
    const [tok, weight, opacity, dash] = PATH_STYLE[p.type] || PATH_STYLE[0];
    L.polyline([a, b], { renderer: canvas, color: col(tok), weight, opacity, dashArray: dash || null, lineCap: 'round' }).bindPopup(pop).addTo(group);
    const key = `${name}:${Math.round(a[0] * 400)}:${Math.round(a[1] * 400)}`;
    if (name && (p.type === 1 || p.type === 4) && !labelled.has(key)) {
      labelled.add(key);
      L.tooltip({ permanent: true, direction: 'center', className: 'twy-sign' })
        .setLatLng([(a[0] + b[0]) / 2, (a[1] + b[1]) / 2]).setContent(esc(name)).addTo(layers.labels);
    }
  }
  for (const p of data.taxiPoints) {
    const hold = HOLD_SHORT.has(p.type);
    L.circleMarker([p.lat, p.lon], {
      renderer: canvas, radius: hold ? 4.5 : 2.2, color: hold ? col('--route-casing') : col('--text-3'), weight: hold ? 1.5 : 0,
      fillColor: hold ? col('--holdbar') : col('--text-3'), fillOpacity: hold ? 1 : 0.7,
    }).bindPopup(() => `<b>Taxi point #${p.index}</b><br>` + kv({
      type: `${p.type} ${POINT_TYPES[p.type] || '(' + p.type + ')'}`, orientation: p.orientation, biasX: p.biasX.toFixed(1), biasZ: p.biasZ.toFixed(1),
    })).addTo(hold ? layers.holds : layers.points);
  }
}

// The stand picked for New flight.
function markPicked() {
  for (const [idx, c] of parkingCircles) {
    const el = c.getElement && c.getElement();
    if (el) el.classList.toggle('is-picked', !!routeFrom && routeFrom.index === idx);
  }
}

// Stands held by our traffic, or found occupied by a scan (GET /api/stands).
let occupiedSig = '';
function drawOccupied(list) {
  const sig = JSON.stringify(list.map((s) => [s.index, s.detected && !s.owner]));
  if (sig === occupiedSig) return;
  occupiedSig = sig;
  layers.occupied.clearLayers();
  for (const s of list) {
    const p = data && data.parking[s.index];
    if (!p) continue;
    L.circle([p.lat, p.lon], { radius: Math.max(p.radius, 3), className: s.detected && !s.owner ? 'm-occ m-occ--det' : 'm-occ', interactive: false }).addTo(layers.occupied);
  }
}

/* ───────────── Aircraft ───────────── */
const acMarkers = new Map(); // object ID → marker
let userMarker = null;
let popupFor = 0; // the object whose details popup is open

function acIcon(cat) {
  return L.divIcon({
    className: 'acm', iconSize: [30, 30], iconAnchor: [15, 15],
    html: `<div class="acm__wrap"><div class="ac ac--${cat}">${icon('i-jet', '')}</div><div class="dtag dtag--${cat}"></div></div>`,
  });
}
// updateMarker restyles a marker in place (no new icon each second).
function updateMarker(m, cat, hdg, tag, sel, wait) {
  const el = m.getElement();
  if (!el) return;
  const ac = el.querySelector('.ac'), dt = el.querySelector('.dtag');
  if (m._cat !== cat) {
    m._cat = cat;
    ac.className = `ac ac--${cat}`;
  }
  // The heading unwrapped, so 359° → 1° turns 2°, not back round.
  const prev = m._hdg === undefined ? hdg : m._hdg;
  m._hdg = prev + ((((hdg - prev) % 360) + 540) % 360 - 180);
  ac.style.setProperty('--hdg', m._hdg.toFixed(1) + 'deg');
  dt.className = `dtag dtag--${cat}${wait ? ' dtag--wait' : ''}`;
  setHTML(dt, tag);
  el.classList.toggle('is-sel', sel);
  m.setZIndexOffset(sel ? 1000 : cat === 'other' ? 0 : 500);
}

// vsArrow: climbing, descending or level (within ±300 fpm, so wobble in
// cruise does not flicker the arrow).
const vsArrow = (t) => t.onGround ? '' : t.vs > 300 ? '<span class="vs-up">↑</span>' : t.vs < -300 ? '<span class="vs-dn">↓</span>' : '<span class="vs-lv">→</span>';
function tagFor(t) {
  const cs = esc(t.tail || t.objectId);
  const f = t.ours && !t.onGround ? schedFlight(t.tail) : null;
  let line;
  if (f) line = `${flightLevel(t.alt || 0)} <i>·</i> ${f.kind === 'overflight' ? esc(f.origin) + '→' : '→'}${esc(f.destination)}`; // en route: level, destination (#369)
  else if (t.onGround) line = `${t.groundKts.toFixed(0)} kt`;
  else line = `${t.groundKts.toFixed(0)} kt <i>·</i> ${t.agl.toFixed(0)} ft`;
  const v = t.ours ? ctlViews.find((x) => x.tail === t.tail && !x.done) : null;
  return `<b>${vsArrow(t)}${cs}</b>${v && v.rules === 'VFR' ? ' <span class="rules rules--v">VFR</span>' : ''}<br>${line}`;
}
const shortState = (s) => (s || '').replace(/^STATE_/, '').toLowerCase().replace(/_/g, ' ');
const KIND_NOTE = { enroute: 'arrival en route (MSFS AI on its plan, handed over at the STAR entry)', overflight: 'overflight (MSFS AI on its plan, crossing the area)', departed: 'departed (MSFS AI on its plan after the SID)', controlled: 'under our control', other: 'other traffic (not ours)' };
// The kind of an aircraft, as its popup's chip shows it.
const KIND_CHIP = { enroute: 'Arrival', overflight: 'Overflight', departed: 'Departure', controlled: 'Ours', other: 'Other traffic' };
// Lights by their letters (Traffic.Lights): the ones that are on.
const LIGHT_NAMES = { N: 'nav', B: 'beacon', S: 'strobe', T: 'taxi', L: 'landing', O: 'logo', W: 'wing' };
const lightsSaid = (s) => [...(s || '')].map((c) => LIGHT_NAMES[c]).filter(Boolean).join(', ') || 'off';
// trafficDetail is an aircraft's popup: who it is, where it goes, how it
// flies, and who flies it.
function trafficDetail(t) {
  const kind = trafficKind(t);
  const f = t.ours ? schedFlight(t.tail) : null;
  const vs = Math.abs(t.vs) < 300 ? 'level' : `${t.vs > 0 ? '↑' : '↓'} ${Math.round(Math.abs(t.vs) / 100) * 100} fpm`;
  const rows = [
    ['Altitude', t.onGround ? 'on the ground' : `${flightLevel(t.alt || 0)}${t.agl < 5000 ? ` <span class="muted">(${Math.round(t.agl / 10) * 10} ft AGL)</span>` : ''}`],
    ['Speed', `${t.groundKts.toFixed(0)} kt`],
    ['Vertical', t.onGround ? '—' : vs],
    ['Heading', `${String(Math.round(t.heading || 0) % 360).padStart(3, '0')}°`],
    ['Phase', esc(shortState(t.state) || '—')],
    ['Lights', esc(lightsSaid(t.lights))],
  ];
  // The gear only where it matters: on the ground or low.
  if (t.onGround || t.agl < 5000) rows.push(['Gear', t.gear >= 0.99 ? 'down' : t.gear <= 0.01 ? 'up' : 'moving']);
  rows.push(['Span', `${t.span.toFixed(0)} m`]);
  const route = f ? `<div class="tdet__route">${esc(f.origin || 'local')} → ${esc(f.destination || 'local')}${f.std ? ` <span class="muted">STD ${hhmm(f.std)} · STA ${hhmm(f.sta)}</span>` : ''}</div>` : '';
  return `<div class="tdet">
    <div class="tdet__head"><span class="tdet__cs">${esc(t.tail || String(t.objectId))}</span><span class="tdet__chip tdet__chip--${kind}">${KIND_CHIP[kind] || ''}</span></div>
    <div class="tdet__type">${t.type ? `<b>${esc(t.type)}</b> · ` : ''}<span class="muted">${esc(t.title)}</span></div>
    ${route}
    <dl class="tdet__kv">${rows.map(([k, v]) => `<dt>${k}</dt><dd>${v}</dd>`).join('')}</dl>
    <div class="tdet__note">${esc(KIND_NOTE[kind] || '')}</div>
  </div>`;
}
const CAT_OF = { controlled: 'ours', enroute: 'arr', overflight: 'ovf', departed: 'dep', other: 'other' };

let lastTraffic = [];
function drawTraffic(list) {
  lastTraffic = list;
  const seen = new Set();
  const selTail = selectedTail();
  for (const t of list) {
    if (t.user || (t.ours ? !layerOn.ours : !layerOn.others)) continue;
    seen.add(t.objectId);
    const kind = trafficKind(t), cat = CAT_OF[kind] || 'other';
    let m = acMarkers.get(t.objectId);
    if (!m) {
      m = L.marker([t.lat, t.lon], { icon: acIcon(cat), keyboard: false, riseOnHover: true, pane: 'aircraft' }).addTo(layers.traffic);
      const id = t.objectId;
      m.on('click', () => aircraftClicked(id));
      acMarkers.set(t.objectId, m);
    }
    m.setLatLng([t.lat, t.lon]);
    const v = kind === 'controlled' ? ctlViews.find((x) => x.tail === t.tail && !x.done) : null;
    updateMarker(m, cat, t.heading, tagFor(t),!!selTail && t.tail === selTail, !!v && waits(v));
    // Not moved since the last poll (the sim's AI frozen, or the data
    // stale): no dead reckoning, or it glides ahead and snaps back each
    // second (live, 2026-10-02: "moving slightly forward and backward").
    const still = m._dr && m._dr.lat === t.lat && m._dr.lon === t.lon;
    m._dr = { lat: t.lat, lon: t.lon, hdg: t.heading, kts: still ? 0 : t.groundKts, at: performance.now() };
    if (popupFor === t.objectId && map._popup && map.hasLayer(map._popup)) map._popup.setLatLng([t.lat, t.lon]).setContent(trafficDetail(t));
  }
  for (const [id, m] of acMarkers) if (!seen.has(id)) { m.remove(); acMarkers.delete(id); }
}
function aircraftClicked(objectId) {
  const t = lastTraffic.find((x) => x.objectId === objectId);
  if (!t) return;
  const v = t.ours && ctlViews.find((x) => x.tail === t.tail && !x.done);
  if (v) { select(v.id); return; }
  popupFor = objectId;
  L.popup({ offset: [0, -8] }).setLatLng([t.lat, t.lon]).setContent(trafficDetail(t)).openOn(map);
}
map.on('popupclose', () => { popupFor = 0; });
// Highlight the selected aircraft at once, without waiting for a poll.
function refreshSelection() {
  const selTail = selectedTail();
  for (const [id, m] of acMarkers) {
    const t = lastTraffic.find((x) => x.objectId === id);
    const el = m.getElement();
    if (t && el) { const sel = !!selTail && t.tail === selTail; el.classList.toggle('is-sel', sel); m.setZIndexOffset(sel ? 1000 : 500); }
  }
}

function drawUser(a) {
  if (!a) { if (userMarker) { userMarker.remove(); userMarker = null; } return; }
  if (!userMarker) userMarker = L.marker([a.lat, a.lon], { icon: acIcon('user'), keyboard: false, zIndexOffset: 1100, pane: 'aircraft' }).addTo(map);
  userMarker.setLatLng([a.lat, a.lon]);
  updateMarker(userMarker, 'user', a.heading, `<b>You</b><br>${a.groundKts.toFixed(0)} kt`, false, false);
  userMarker.setZIndexOffset(1100);
}

// Between the polls (1 s) the markers are dead-reckoned on their heading
// and speed at the simulation rate, so movement looks continuous (at 180 kt
// a step was 90 m); not at all while the sim is paused.
function glideTraffic() {
  const now = performance.now();
  const rate = lastOwn ? (lastOwn.paused ? 0 : lastOwn.simRate || 1) : 1;
  if (rate) {
    for (const m of acMarkers.values()) {
      const d = m._dr;
      if (!d || d.kts < 1) continue;
      const meters = Math.min(1.5, (now - d.at) / 1000) * rate * d.kts * 0.514444;
      const h = d.hdg * Math.PI / 180;
      m.setLatLng(offset(d.lat, d.lon, Math.sin(h) * meters, Math.cos(h) * meters));
    }
  }
  requestAnimationFrame(glideTraffic);
}

// Safe zones: half the wing span plus 3 m around every aircraft on the
// ground; red where two overlap — wingtips that could touch.
const SAFE_MARGIN = 3;
function drawSafeZones(list) {
  layers.safe.clearLayers();
  if (!layerOn.safe) return;
  const zones = list.filter((t) => t.onGround && t.span > 0).map((t) => ({ at: L.latLng(t.lat, t.lon), r: t.span / 2 + SAFE_MARGIN, clash: false }));
  for (let i = 0; i < zones.length; i++) {
    for (let j = i + 1; j < zones.length; j++) {
      if (zones[i].at.distanceTo(zones[j].at) < zones[i].r + zones[j].r) zones[i].clash = zones[j].clash = true;
    }
  }
  for (const z of zones) L.circle(z.at, { radius: z.r, className: z.clash ? 'm-safe m-safe--clash' : 'm-safe', interactive: false }).addTo(layers.safe);
}

// locate centres the map on a point and rings it briefly.
function locate(lat, lon, label) {
  map.setView([lat, lon], Math.max(map.getZoom(), 15));
  const ring = L.circleMarker([lat, lon], { radius: 26, className: 'm-locate', interactive: false }).addTo(layers.locate);
  if (label) ring.bindTooltip(esc(label), { permanent: true, direction: 'top', className: 'map-lbl' });
  setTimeout(() => ring.remove(), 2500);
}

/* ───────────── Routes of our aircraft ───────────── */
// One layer group per aircraft, rebuilt only when its route, clearance
// limit or selection changed; the air route's first point follows the
// aircraft in place.
const routeLayers = new Map(); // control ID → {sig, group, air}
function drawRoutes() {
  const keep = new Set();
  for (const v of ctlViews) {
    const sel = v.id === ctlSelected;
    if (!sel && (!layerOn.routes || !layerOn.ours || v.done)) continue;
    const ground = v.route && v.route.length > 1 && v.state !== 'complete' && !v.done;
    const air = v.airRoute && v.airRoute.length;
    if (!ground && !air && !v.hold) continue;
    keep.add(v.id);
    const canUpTo = !!(v.actions && v.actions.includes('upto')) && onMyFrequency(v);
    const sig = JSON.stringify([sel, canUpTo, ground ? v.route : 0, v.limitNode, v.atLimit, air ? v.airRoute : 0, v.airFixes || 0, v.hold || 0]);
    let r = routeLayers.get(v.id);
    if (!r || r.sig !== sig) {
      if (r) r.group.remove();
      r = buildRoute(v, sel, ground, air, canUpTo);
      r.sig = sig;
      r.group.addTo(layers.routes);
      routeLayers.set(v.id, r);
    }
    if (r.air && v.position) r.air.forEach((l) => l.setLatLngs(smoothLine([[v.position.lat, v.position.lon], ...v.airRoute.map((p) => [p.lat, p.lon])])));
  }
  for (const [id, r] of routeLayers) if (!keep.has(id)) { r.group.remove(); routeLayers.delete(id); }
}
function buildRoute(v, sel, ground, air, canUpTo) {
  const g = L.layerGroup();
  const out = { group: g, air: null };
  const faint = sel ? '' : ' m-route--faint';
  if (ground) {
    const pts = v.route.map((p) => [p.lat, p.lon]);
    let li = v.limitNode >= 0 && v.nodes ? v.nodes.indexOf(v.limitNode) : -1;
    if (li < 0) li = pts.length - 1;
    const cleared = pts.slice(0, li + 1), planned = pts.slice(li);
    if (cleared.length > 1) {
      L.polyline(cleared, { className: sel ? 'm-route-casing' : 'm-route-casing m-route-casing--faint', interactive: false }).addTo(g);
      L.polyline(cleared, { className: 'm-route' + faint, interactive: false }).addTo(g);
    }
    if (planned.length > 1) L.polyline(planned, { className: 'm-route m-route--planned', interactive: false }).addTo(g);
    if (sel && v.nodes) {
      v.nodes.forEach((n, i) => {
        if (i === 0 || !pts[i]) return;
        const limit = n === v.limitNode;
        L.circleMarker(pts[i], { radius: limit ? 8 : 4.5, className: limit ? 'm-limit' : 'm-node' + (canUpTo ? '' : ' is-off'), bubblingMouseEvents: false })
          .bindTooltip(limit ? `Cleared up to here (${v.atLimit ? 'holding' : 'taxiing'})` : canUpTo ? 'Clear up to here' : 'Route point', { className: 'node-tip', direction: 'top' })
          .on('click', () => { if (canUpTo) ctlAct(v.id, 'upto', n); })
          .addTo(g);
      });
    }
  }
  if (air) {
    const line = smoothLine([[v.position.lat, v.position.lon], ...v.airRoute.map((p) => [p.lat, p.lon])]);
    out.air = [L.polyline(line, { className: 'm-air' + (sel ? '' : ' m-air--faint'), interactive: false }).addTo(g)];
    if (sel) {
      // Dots at the procedure's fixes only, not at the points of its rounded turns.
      for (const f of v.airFixes || []) {
        L.circleMarker([f.lat, f.lon], { radius: 3.5, className: 'm-fix', interactive: false })
          .bindTooltip(esc(f.ident), { permanent: true, direction: 'right', offset: [6, 0], className: 'map-lbl' }).addTo(g);
      }
    }
  }
  if (v.hold && v.hold.racetrack && v.hold.racetrack.length) {
    const rt = v.hold.racetrack.map((p) => [p.lat, p.lon]);
    L.polygon(rt, { className: 'm-holdpat', fill: false, interactive: false })
      .bindTooltip(`hold ${esc(v.hold.ident)} ${Math.round(v.hold.altFt)} ft`, { permanent: sel, direction: 'top', className: 'map-lbl' }).addTo(g);
  }
  return out;
}

/* ───────────── The final ───────────── */
// The extended centreline to 15 NM with a tick every mile, and each
// arrival within 20 NM at its distance to go: green spacing kept, red short.
let finalSig = '';
function runwayEndNamed(name) {
  for (const r of (data && data.runways) || []) for (const e of [r.primary, r.secondary]) if (e && e.name === name) return e;
  return null;
}
function drawFinal(seqs) {
  const sig = JSON.stringify(seqs.map((r) => [r.runway, r.sequence.map((e) => [e.callsign, e.distanceToGoNM.toFixed(1), e.spacingNM])]));
  if (sig === finalSig) return;
  finalSig = sig;
  layers.final.clearLayers();
  for (const r of seqs) {
    const end = runwayEndNamed(r.runway);
    if (!end || !r.sequence.length) continue;
    const back = (end.heading + 180) * Math.PI / 180;
    const at = (nm) => offset(end.threshold.lat, end.threshold.lon, Math.sin(back) * nm * 1852, Math.cos(back) * nm * 1852);
    L.polyline([at(0), at(15)], { className: 'm-final', interactive: false }).addTo(layers.final);
    for (let nm = 1; nm <= 15; nm++) L.circleMarker(at(nm), { radius: nm % 5 ? 2 : 4, className: 'm-final-tick', interactive: false }).addTo(layers.final);
    r.sequence.forEach((e, i) => {
      if (e.distanceToGoNM > 20) return;
      const lead = i > 0 ? r.sequence[i - 1] : null;
      const gap = lead ? e.distanceToGoNM - lead.distanceToGoNM : 0;
      const short = lead && gap < (e.spacingNM || 0) - 0.3;
      L.circleMarker(at(e.distanceToGoNM), { radius: 6, className: short ? 'm-final-ac--short' : 'm-final-ac--ok', interactive: false })
        .addTo(layers.final);
    });
  }
}

/* ───────────── World view ───────────── */
// The traffic picture's circle, the airports in range and every aircraft
// with call sign, level and phase; other traffic only with its layer on.
let worldOn = false, worldBack = null, lastWorld = null;
function drawWorld() {
  layers.world.clearLayers();
  const w = lastWorld;
  if (!w || !w.hasCentre) return;
  const c = [w.centre.lat, w.centre.lon];
  L.circle(c, { radius: w.radiusNM * 1852, className: 'm-world', interactive: false }).addTo(layers.world);
  L.circleMarker(c, { radius: 3, className: 'm-wcentre', interactive: false }).addTo(layers.world);
  for (const a of w.airports.filter((x) => /^[A-Z]{4}$/.test(x.icao))) {
    L.circleMarker([a.position.lat, a.position.lon], { radius: 3, className: 'm-wapt' })
      .bindTooltip(esc(a.icao), { permanent: true, direction: 'top', className: 'map-lbl map-lbl--sm' }).addTo(layers.world);
  }
  for (const t of w.aircraft) {
    if (t.user || (!t.ours && !layerOn.others)) continue;
    const lvl = t.onGround ? '' : ' ' + flightLevel(t.altFt || 0);
    const f = t.ours ? schedFlight(t.tail) : null;
    const lbl = `${esc(t.tail || t.objectId)}${lvl} ${esc(t.phase)}${f ? ' →' + esc(f.destination) : ''}`;
    L.circleMarker([t.position.lat, t.position.lon], { radius: t.ours ? 5 : 4, className: `m-wac m-wac--${esc(t.phase)}${t.ours ? ' is-ours' : ''}` })
      .bindTooltip(lbl, { permanent: true, direction: 'right', offset: [6, 0], className: 'map-lbl map-lbl--sm' })
      .on('click', () => locate(t.position.lat, t.position.lon, t.tail || String(t.objectId)))
      .addTo(layers.world);
  }
}
function toggleWorld() {
  worldOn = !worldOn;
  const b = $('worldBtn');
  b.setAttribute('aria-pressed', String(worldOn));
  b.title = worldOn ? 'Back to the airport' : 'World view: the traffic picture, airports in range';
  if (worldOn) {
    worldBack = { center: map.getCenter(), zoom: map.getZoom() };
    layers.world.addTo(map);
    drawWorld();
    const w = lastWorld;
    if (w && w.hasCentre) map.fitBounds(L.latLng(w.centre.lat, w.centre.lon).toBounds(w.radiusNM * 1852 * 2));
    else toast('No traffic picture yet (simulator not connected?)');
    worldPoll.now();
  } else {
    layers.world.remove();
    if (worldBack) map.setView(worldBack.center, worldBack.zoom);
  }
}

themeListeners.push(() => { setTiles(); drawNetwork(); });

// drawTugs shows each departure's pushback tug (T) and fuel truck (F,
// #582) while they drive from their depot to the aircraft or home again,
// and the fuel truck while it refuels: a small marker and its way ahead,
// dashed.
const tugMarks = new Map(); // "id:tug" or "id:fuel" -> {marker, line}
const VEHICLES = [['tug', 'T', 'tug'], ['fuel', 'F', 'fuel truck']];
function drawTugs() {
  const keep = new Set();
  for (const v of ctlViews) {
    if (v.done) continue;
    for (const [key, letter, name] of VEHICLES) {
      const t = v[key];
      if (!t) continue;
      const k = `${v.id}:${key}`;
      keep.add(k);
      let m = tugMarks.get(k);
      if (!m) {
        const icon = L.divIcon({ className: `m-tug m-tug--${key}`, html: `<span>${letter}</span>`, iconSize: [16, 16], iconAnchor: [8, 8] });
        m = { marker: L.marker([t.position.lat, t.position.lon], { icon, interactive: false, keyboard: false }).addTo(layers.tugs),
          line: L.polyline([], { className: 'm-tug-route', interactive: false }).addTo(layers.tugs) };
        m.marker.bindTooltip(`${esc(v.tail)} ${name}`, { direction: 'top', className: 'map-lbl' });
        tugMarks.set(k, m);
      }
      m.marker.setLatLng([t.position.lat, t.position.lon]);
      m.line.setLatLngs((t.route || []).map((p) => [p.lat, p.lon]));
    }
  }
  for (const [k, m] of tugMarks) if (!keep.has(k)) { m.marker.remove(); m.line.remove(); tugMarks.delete(k); }
}
