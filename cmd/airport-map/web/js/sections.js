/* The sections and the status strip: simulation, live traffic, camera,
   game, schedule and boards, world, airport info, procedures, the approach
   sequence, radio and voice, map layers. */
'use strict';

const tabVisible = (name) => currentTab === name && !(isPhone() && $('panel').dataset.sheet === 'peek') && !document.body.classList.contains('panel-hidden');

/* ───────────── Simulation, my aircraft, the connection ───────────── */
let lastOwn = null; // GET /api/aircraft: my aircraft, the pause and the rate
let clockShown = false; // the clock popup filled once
let liveAt = 0;     // when the simulator last answered
let simLive = false;
function markLive() { liveAt = Date.now(); updateConn(); }
// needSim stops an action that needs the simulator, saying why. (The
// endpoints behind it answer 503 without it: they are not polled either.)
function needSim(what) {
  if (simLive) return true;
  toast(`${what}: the simulator is not connected`, 'err');
  return false;
}
function updateConn() {
  const was = simLive;
  simLive = Date.now() - liveAt < 6000;
  if (was !== simLive || !updateConn.done) offlineStates();
  updateConn.done = true;
  $('connDot').className = 'dot ' + (simLive ? 'dot--ok' : 'dot--off');
  $('connLbl').textContent = simLive ? 'Connected' : 'Not connected';
  $('connChip').title = simLive ? 'Simulator connected' : 'Simulator not connected: the airport comes from file or cache, traffic and voice need the simulator';
  $('connChip').classList.toggle('chip--off', !simLive);
  $('moreConn').innerHTML = `<span class="dot ${simLive ? 'dot--ok' : 'dot--off'}"></span> ${simLive ? 'connected' : 'not connected'}`;
  if (was !== simLive) { renderStrips(); updateDirector(); pollers.forEach((p) => p.now()); }
}

// The simulator gone: the map blurs behind a dialog with a plane in the
// hold and something to read, until it is back (or dismissed).
const LOST_QUIPS = [
  'Holding at LOST, expect further clearance… eventually.',
  'The tower is calling. Nobody is answering. Classic.',
  'Your aircraft went for a coffee. We will wait.',
  'Squawking 7600 with the simulator. Radio failure, both ways.',
  'Even the ATIS stopped talking. Information Silence.',
  'Did someone trip over the cable? Asking for a friend.',
  'Approach is vectoring the simulator back. Expect a long downwind.',
];
let lostSince = 0, lostDismissed = false, lostQuip = 0;
function updateLost() {
  const el = $('lostOverlay');
  if (simLive) { lostSince = 0; lostDismissed = false; el.hidden = true; return; }
  // The map server gone, or only the simulator: said apart.
  const down = serverDown();
  if (el.dataset.down !== String(down)) {
    el.dataset.down = String(down);
    $('lostTitle').textContent = down ? 'The map server is not answering' : 'Houston, we\x27ve lost the simulator';
    if (down) $('lostQuip').textContent = `Is the airport map still running on ${location.host}? Restarted, this page picks it up by itself.`;
  }
  if (!lostSince) lostSince = Date.now();
  // A moment first: on load the simulator answers within a second or two.
  const was = el.hidden;
  el.hidden = lostDismissed || Date.now() - lostSince < 3000;
  if (was && !el.hidden) $('lostDismiss').focus(); // a modal: the focus goes in
  const s = Math.floor((Date.now() - lostSince) / 1000);
  $('lostFor').textContent = `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}
setInterval(() => {
  updateLost();
  // Another line every 7 s, faded.
  if (!$('lostOverlay').hidden && !serverDown() && Math.floor(Date.now() / 1000) % 7 === 0) {
    const q = $('lostQuip');
    q.classList.add('is-out');
    setTimeout(() => { lostQuip = (lostQuip + 1) % LOST_QUIPS.length; q.textContent = LOST_QUIPS[lostQuip]; q.classList.remove('is-out'); }, 250);
  }
}, 1000);
// offlineStates says "not connected" where the simulator is missing, and
// forgets what came from it.
function offlineStates() {
  if (simLive) return;
  camView = null;
  game = null;
  $('gameChip').hidden = true;
  $('gamePill').textContent = 'needs the simulator';
  $('gamePill').className = 'pill';
  $('schedPill').textContent = 'needs the simulator';
  $('schedPill').className = 'pill';
  $('sInfo').textContent = 'The simulator is not connected: the schedule starts once it is.';
  $('wInfo').textContent = 'The simulator is not connected: no traffic picture.';
  $('pInfo').textContent = procs ? $('pInfo').textContent : 'Procedures come from the simulator (not connected).';
  drawOccupied([]);
  renderBoard();
}
const rateLabel = (r) => ({ 0.25: '¼×', 0.5: '½×' }[r] || `${+r.toFixed(2)}×`);
function showSim() {
  const a = lastOwn, paused = !!(a && a.paused);
  const btn = $('simPause');
  btn.setAttribute('aria-pressed', String(paused));
  btn.innerHTML = `${icon(paused ? 'i-play' : 'i-pause')}<span class="lbl">${paused ? 'Resume' : 'Pause'}</span>`;
  btn.setAttribute('aria-label', paused ? 'Resume the simulation' : 'Pause the simulation');
  $('pausedBanner').hidden = !paused;
  document.querySelector('.strip').classList.toggle('is-paused', paused);
  const lbl = a ? (paused ? 'paused' : rateLabel(a.simRate || 1)) : '—';
  $$('[data-rate-out]').forEach((o) => { o.textContent = lbl; });
  updateSheetSummary();
}
async function simAct(action) {
  if (!needSim('Simulation')) return;
  const r = await send(`/api/sim?action=${action}`);
  if (!r.ok) toast(r.error, 'err');
  setTimeout(() => aircraftPoll.now(), 300);
}
async function pollAircraft() {
  const r = await api('/api/aircraft');
  if (!r.ok) { lastOwn = null; drawUser(null); showSim(); updateConn(); return false; }
  lastOwn = r.data;
  if (lastOwn) markLive(); else updateConn();
  drawUser(lastOwn);
  showSim();
  const a = lastOwn;
  // The simulator's time: UTC, and local at the aircraft.
  const hhmm = (sec) => { const m = Math.floor(((sec % 86400) + 86400) % 86400 / 60); return `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`; };
  $('clockChip').hidden = !a || !a.zuluSec;
  if (a && a.zuluSec) {
    $('clockZ').textContent = `${hhmm(a.zuluSec)}Z`;
    $('clockL').textContent = `${hhmm(a.localSec)} LT`;
    // The popup: the date, both times, the offset, the part of the day.
    let off = Math.round((a.localSec - a.zuluSec) / 900) * 15; // minutes, to the quarter hour
    if (off > 720) off -= 1440; else if (off < -720) off += 1440;
    const offTxt = `UTC${off >= 0 ? '+' : '−'}${Math.floor(Math.abs(off) / 60)}${Math.abs(off) % 60 ? ':' + String(Math.abs(off) % 60).padStart(2, '0') : ''}`;
    const date = a.zuluYear ? new Date(Date.UTC(a.zuluYear, a.zuluMonth - 1, a.zuluDay)) : null;
    const dateTxt = date ? date.toLocaleDateString(undefined, { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric', timeZone: 'UTC' }) : '—';
    const part = ['dawn', 'day', 'dusk', 'night'][a.dayPart] || '—';
    if (!$('clockPop').hidden || !clockShown) setHTML($('clockPopBody'), `<dl class="kv">
      <dt>Date</dt><dd>${esc(dateTxt)} <span class="muted">(UTC)</span></dd>
      <dt>UTC</dt><dd class="mono">${hhmm(a.zuluSec)}:${String(Math.floor(a.zuluSec % 60)).padStart(2, '0')}Z</dd>
      <dt>Local</dt><dd class="mono">${hhmm(a.localSec)} <span class="muted">${offTxt}</span></dd>
      <dt>Daylight</dt><dd>${part}</dd>
      <dt>Simulation</dt><dd>${a.paused ? 'paused' : `running${a.simRate && a.simRate !== 1 ? `, ${a.simRate}×` : ''}`}</dd>
    </dl>`);
    clockShown = true;
  }
  // Following one of ours (its Follow) wins: the two would pull the map apart.
  if (a && layerOn.follow && !ctlFollow) map.panTo([a.lat, a.lon], { animate: false });
  $('acInfo').textContent = a
    ? `My aircraft: ${a.lat.toFixed(6)}, ${a.lon.toFixed(6)} · ${a.heading.toFixed(0)}° · ${a.groundKts.toFixed(0)} kt${a.onGround ? ' · on ground' : ''}${a.paused ? ' · sim paused' : a.simRate && a.simRate !== 1 ? ` · sim ${a.simRate}×` : ''}`
    : 'My aircraft: not reported (simulator not connected?)';
}

/* ───────────── Live traffic ───────────── */
async function pollTraffic() {
  const r = await api('/api/traffic');
  if (!r.ok) return false;
  const list = r.data || [];
  drawTraffic(list);
  drawSafeZones(list);
  if (tabVisible('map')) renderOthers(list);
}
function renderOthers(list) {
  const el = $('trafficTable');
  if (interacting(el)) return;
  const rows = list.filter((t) => !t.user && !t.ours).sort((a, b) => a.agl - b.agl);
  $('othersN').textContent = `${rows.length} aircraft`;
  setHTML(el, rows.length
    ? '<thead><tr><th>Call sign</th><th class="n">ft</th><th class="n">kt</th><th class="n">fpm</th><th class="n">gear</th><th></th></tr></thead><tbody>' +
      rows.map((t) => `<tr class="loc" data-lat="${t.lat}" data-lon="${t.lon}" data-name="${esc(t.tail || t.objectId)}" title="${esc(t.title)} — click to show"><td class="mono strong">${esc(t.tail || t.objectId)}<br><span class="muted small">${esc(shortState(t.state))}</span></td>` +
        `<td class="n mono">${t.agl.toFixed(0)}</td><td class="n mono">${t.groundKts.toFixed(0)}</td><td class="n mono">${t.vs.toFixed(0)}</td><td class="n mono">${(t.gear * 100).toFixed(0)}%</td>` +
        `<td><button type="button" class="btn btn--icon btn--ghost rmOther" data-oid="${t.objectId}" aria-label="Remove ${esc(t.tail || t.objectId)} from the simulator" title="Remove from the simulator">${icon('i-x', 'ic ic--sm')}</button></td></tr>`).join('') + '</tbody>'
    : `<tbody><tr><td class="muted">${simLive ? 'No other aircraft.' : 'No traffic: the simulator is not connected.'}</td></tr></tbody>`);
}

/* ───────────── Our controlled aircraft ───────────── */
async function pollControl() {
  const r = await api('/api/control');
  if (!r.ok) return false;
  controlUpdated(r.data || []);
  if ($('ctlLogBox').open) {
    const lg = await api('/api/control/log');
    const el = $('ctlLog'), lines = (lg.ok && lg.data) || [];
    const sig = lines.length + '|' + (lines[lines.length - 1] || '');
    if (el.dataset.sig !== sig) {
      el.dataset.sig = sig;
      // One row per entry, newest first: time, call sign, kind, message.
      el.innerHTML = lines.length ? lines.slice().reverse().map(tlogRow).join('') : '<li class="tlog__empty">Nothing yet.</li>';
    }
  }
}
// tlogRow is a traffic log line ("00:03:15.609  WZZ1979 ATC: Wizzair 1979, …")
// as a row: the time to the second, the call sign, the kind, the message.
function tlogRow(line) {
  const m = /^(\d\d:\d\d:\d\d)(?:\.\d+)?\s+(.*)$/.exec(line);
  if (!m) return `<li class="tlog__row"><span class="tlog__msg">${esc(line)}</span></li>`;
  let rest = m[2].trim(), cs = '';
  const c = /^([A-Z0-9]{2,8})\s+(.*)$/.exec(rest);
  if (c) { cs = c[1]; rest = c[2]; }
  let kind = '';
  const k = /^(ATC|pilot|departure|arrival|sequence|tower|schedule|conflict|separation|runway in use|scene)\b([^:]*):\s*(.*)$/.exec(rest);
  // "sequence LKPR 24: number 2": what is between the kind and the colon
  // stays with the message.
  if (k) { kind = k[1]; rest = (k[2].trim() ? k[2].trim() + ': ' : '') + k[3]; }
  const cls = kind ? 'tlog--' + kind.split(' ')[0].toLowerCase() : '';
  return `<li class="tlog__row ${cls}"><span class="tlog__t">${m[1]}</span><span class="tlog__cs">${esc(cs)}</span><span class="tlog__kind">${esc(kind)}</span><span class="tlog__msg">${esc(rest)}</span></li>`;
}
async function pollStands() {
  if (!data) return;
  const r = await api(`/api/stands?icao=${encodeURIComponent(data.icao)}`);
  if (!r.ok) { drawOccupied([]); return false; }
  markLive();
  drawOccupied(r.data || []);
}

/* ───────────── Camera: the director ───────────── */
let camView = null; // GET /api/camera; null: not connected
let scenes = [];
const DIRECTOR_HTML = `<div class="field"><span class="field__lbl">Camera</span>
    <div class="seg" role="radiogroup" aria-label="Camera mode" title="The simulator's camera on our traffic">
      <button type="button" role="radio" data-cam="off">Off</button><button type="button" role="radio" data-cam="auto" title="Cuts to the aircraft on the radio as you hear it">Auto director</button><button type="button" role="radio" data-cam="follow" title="Stays on the selected aircraft">Follow selected</button><button type="button" role="radio" data-cam="tower" title="From the airport's tower: the selected aircraft, or with none selected a slow look round the airfield">Tower</button>
    </div></div>
  <div class="field" data-look-row hidden><span class="field__lbl">Tower look</span>
    <div class="director__row look" title="Turn the tower camera; hold a button to keep turning. Keys: arrows turn, + and − zoom (Shift: faster). Mouse: hold the middle button and drag to turn, roll the wheel while holding it to zoom. Double-click the middle button to lock the view to the mouse (no button held); middle click or Esc unlocks.">
      <button type="button" class="btn btn--sm" data-look="-4,0,0" aria-label="Turn left">⟲</button><button type="button" class="btn btn--sm" data-look="4,0,0" aria-label="Turn right">⟳</button><button type="button" class="btn btn--sm" data-look="0,1.5,0" aria-label="Look up">▲</button><button type="button" class="btn btn--sm" data-look="0,-1.5,0" aria-label="Look down">▼</button><button type="button" class="btn btn--sm" data-look="0,0,-3" aria-label="Zoom in">+</button><button type="button" class="btn btn--sm" data-look="0,0,3" aria-label="Zoom out">−</button>
      <button type="button" class="btn btn--sm btn--toggle" data-swing aria-pressed="false" title="Swing slowly from side to side by itself">Swing</button>
      <label class="switch switch--sm" title="Show on the map where the tower camera looks: its bearing, tilt and field of view"><input type="checkbox" data-angle><span class="switch__ui" aria-hidden="true"></span>Angle</label>
    </div></div>
  <div class="field"><span class="field__lbl">Simulator camera</span>
    <div class="director__row" title="The simulator's own cameras, on your aircraft; ◀ ▶ the camera's views">
      <button type="button" class="btn btn--sm" data-sim-step="-1" aria-label="Previous view">◀</button>
      <div class="seg" role="radiogroup" aria-label="Simulator camera">
        <button type="button" role="radio" data-sim="cockpit">Cockpit</button><button type="button" role="radio" data-sim="chase">Chase</button><button type="button" role="radio" data-sim="drone">Drone</button><button type="button" role="radio" data-sim="fixed" title="Fixed on the plane">Fixed</button><button type="button" role="radio" data-sim="environment" title="A free camera">Free</button>
      </div>
      <button type="button" class="btn btn--sm" data-sim-step="1" aria-label="Next view">▶</button>
    </div></div>
  <div class="field"><span class="field__lbl">Scene</span>
    <div class="director__row">
      <select class="input" data-scene aria-label="Scene" title="Scripted films: the scene spawns its aircraft and films them beat by beat with the radio (scenes/*.json, read on every play)"></select>
      <button type="button" class="btn btn--primary" data-scene-play>${icon('i-play', 'ic ic--sm')}<span>Play</span></button>
    </div></div>
  <div class="director__info" data-cam-info></div>`;
function initDirector() {
  $$('[data-director]').forEach((d) => { d.innerHTML = DIRECTOR_HTML; });
  // Tower look: a step per press, repeated while held.
  let lookTimer = 0;
  // Through queueLook: steps go out together, never piling up on a slow LAN.
  const lookStep = (b) => { const [yaw, tilt, fov] = b.dataset.look.split(',').map(Number); queueLook(yaw, tilt, fov); };
  // From the keyboard (Enter, Space): a click without a pointer.
  document.addEventListener('click', (e) => { const b = e.target.closest('[data-look]'); if (b && e.detail === 0) lookStep(b); });
  const lookStop = () => { clearInterval(lookTimer); lookTimer = 0; };
  document.addEventListener('pointerdown', (e) => {
    const b = e.target.closest('[data-look]');
    if (!b) return;
    e.preventDefault();
    lookStep(b);
    lookStop();
    lookTimer = setInterval(() => lookStep(b), 120);
  });
  ['pointerup', 'pointercancel', 'pointerleave'].forEach((t) => document.addEventListener(t, lookStop, true));
  document.addEventListener('click', (e) => {
    const c = e.target.closest('[data-cam]');
    if (c && c.dataset.cam === 'tower') { camViewOf('tower', ctlSelected || -1); return; }
    if (c) { camPost('/api/camera', { mode: c.dataset.cam, id: ctlSelected || 0 }); return; }
    // The simulator's own camera, or a step through its views.
    const sw = e.target.closest('[data-swing]');
    if (sw) { camPost('/api/camera', { mode: 'look', swing: sw.getAttribute('aria-pressed') !== 'true' }); return; }
    const sb = e.target.closest('[data-sim]');
    if (sb) { camPost('/api/camera', { mode: 'sim', sim: sb.dataset.sim }); return; }
    const st = e.target.closest('[data-sim-step]');
    if (st) { camPost('/api/camera', { mode: 'sim', step: Number(st.dataset.simStep) }); return; }
    if (e.target.closest('[data-scene-play]')) {
      if (camView && camView.mode === 'scene') { camPost('/api/camera', { mode: 'off' }); return; }
      if (!data) { toast('Load an airport first', 'err'); return; }
      const name = document.querySelector('[data-scene]').value;
      camPost(`/api/camera/scene?name=${encodeURIComponent(name)}&icao=${encodeURIComponent(data.icao)}`);
    }
  });
  document.addEventListener('change', (e) => {
    if (e.target.matches('[data-scene]')) $$('[data-scene]').forEach((s) => { s.value = e.target.value; });
  });
  api('/api/camera/scenes').then((r) => {
    scenes = (r.ok && r.data) || [];
    const opts = scenes.map((s) => `<option value="${esc(s.key)}" title="${esc(s.description)}">${esc(s.name)}</option>`).join('') || '<option value="">no scenes</option>';
    $$('[data-scene]').forEach((s) => { s.innerHTML = opts; });
  });
  updateDirector();
}
// camViewOf puts the camera on a fixed view of aircraft id (-1: the user's).
function camViewOf(view, id) { camPost('/api/camera', { mode: 'view', view, id, icao: data ? data.icao : '' }); }
async function camPost(path, body) {
  if (!needSim('Camera')) return;
  const r = await send(path, body || {});
  if (!r.ok) { toast(`Camera: ${r.error}`, 'err'); $$('[data-cam-info]').forEach((i) => { i.textContent = r.error; }); return; }
  camView = r.data;
  updateDirector();
  renderCtx();
}
async function pollCamera() {
  const r = await api('/api/camera');
  if (!r.ok) { if (camView) { camView = null; updateDirector(); renderCtx(); } return false; }
  markLive();
  const changed = JSON.stringify(camView) !== JSON.stringify(r.data);
  camView = r.data;
  if (changed) { updateDirector(); renderCtx(); }
}
function updateDirector() {
  const v = camView, mode = v ? v.mode : 'off', playing = mode === 'scene';
  // The tower: its own mode with none selected, a held view with one.
  const tower = mode === 'tower' || mode === 'view' && !!v && v.shot === 'tower';
  $$('[data-cam]').forEach((b) => b.setAttribute('aria-checked', String(b.dataset.cam === 'tower' ? tower : b.dataset.cam === mode && !(mode === 'off' && v && v.sim))));
  $$('[data-sim]').forEach((b) => b.setAttribute('aria-checked', String(mode === 'off' && !!v && b.dataset.sim === v.sim)));
  // The look buttons while the tower looks round (no aircraft selected).
  $$('[data-look-row]').forEach((r) => { r.hidden = mode !== 'tower'; });
  $$('[data-swing]').forEach((b) => b.setAttribute('aria-pressed', String(!!(v && v.swing))));
  $$('[data-angle]').forEach((c) => { c.checked = showAngle; });
  drawAngle();
  $$('[data-scene-play]').forEach((b) => {
    b.className = `btn ${playing ? 'btn--danger' : 'btn--primary'}`;
    b.innerHTML = `${icon(playing ? 'i-stop' : 'i-play', 'ic ic--sm')}<span>${playing ? 'Stop' : 'Play'}</span>`;
  });
  const info = !v ? (simLive ? 'Camera not available.' : 'The camera needs the simulator (not connected).')
    : v.mode === 'off' ? (v.sim ? `Simulator camera: ${v.sim}${v.simView ? ', view ' + (v.simView + 1) : ''}` : 'The simulator camera is yours.')
    : [playing ? 'Playing' : v.mode === 'auto' ? 'Auto: cuts to the aircraft heard on the radio' : v.mode === 'view' ? 'View' : v.mode === 'tower' ? (v.swing ? 'Tower: swinging round the airfield' : 'Tower: turn it with the look buttons') : 'Follow', v.subject, v.shot, v.acquired ? '' : 'waiting for the camera', v.error].filter(Boolean).join(' · ');
  $$('[data-cam-info]').forEach((i) => { i.textContent = info; });
  const chip = $('camBtn');
  chip.classList.toggle('chip--onair', playing);
  const scene = playing && scenes.find((s) => s.key === document.querySelector('[data-scene]').value);
  $('camChipLbl').textContent = playing ? `On air${scene ? ' · ' + scene.name : ''}` : (mode === 'off' && v && v.sim ? 'Sim' : tower ? 'Tower' : { off: 'Off', auto: 'Auto', follow: 'Follow', view: 'View', tower: 'Tower' }[mode] || mode);
}

/* ───────────── ATC game ───────────── */
let game = null;
async function pollGame() {
  const r = await api('/api/game');
  if (!r.ok) {
    game = null;
    $('gameChip').hidden = true;
    $('gamePill').textContent = 'not available';
    $('gamePill').className = 'pill';
    return false;
  }
  markLive();
  const g = game = r.data;
  $('gStart').innerHTML = `${icon(g.on ? 'i-stop' : 'i-play')}<span>${g.on ? 'Stop game' : 'Start game'}</span>`;
  $('gStart').className = `btn btn--grow ${g.on ? '' : 'btn--primary'}`;
  $('gameChip').hidden = !g.on;
  const next = Math.max(0, g.nextInSec || 0);
  $('gameChipScore').textContent = g.score;
  $('gameChipScore').classList.toggle('neg', g.score < 0);
  $('gameChipSub').textContent = `${g.handled}/${g.spawned} · next ${mmss(next)}`;
  $('moreGame').textContent = g.on ? `Game · ${g.score} pts · ${g.handled}/${g.spawned}` : 'ATC game';
  $('gamePill').textContent = g.on ? `${g.score} pts` : 'not playing';
  $('gamePill').className = `pill ${g.on ? 'pill--accent' : ''}`;
  const mins = g.started && g.on ? Math.floor((Date.now() - Date.parse(g.started)) / 60000) : 0;
  $('gScore').textContent = g.on ? `Playing for ${mins} min at ${g.icao}, runway ${g.runway} · ${g.handled} of ${g.spawned} flights handled · next in ${mmss(next)}.`
    : g.icao ? `Last game: ${g.score} points, ${g.handled} of ${g.spawned} flights handled.` : 'You are ground and tower on the New flight runway. Arrivals come in on STARs, departures wait at their stands; every aircraft holds for your clearances.';
  const el = $('gLog'), text = (g.events || []).join('\n');
  if (el.textContent !== text) { el.textContent = text; el.scrollTop = el.scrollHeight; }
}
async function toggleGame() {
  if (!data) { toast('Load an airport first', 'err'); return; }
  if (!needSim('Game')) return;
  // "active": each new flight gets the runway in use when it appears.
  const body = { on: !(game && game.on), icao: data.icao, runway: $('rTo').value || 'active', intervalSec: Number($('gEvery').value) * 60 || 180 };
  const r = await send('/api/game', body);
  if (!r.ok) toast(`Game: ${r.error}`, 'err');
  gamePoll.now();
}

/* ───────────── Scheduled traffic and boards ───────────── */
let sched = null;
let schedSkew = 0; // traffic time minus the browser's clock (ms)
let board = 'dep';
async function pollSchedule() {
  const r = await api('/api/schedule');
  if (!r.ok) {
    $('sInfo').textContent = r.error;
    $('schedPill').textContent = 'not available';
    $('schedPill').className = 'pill';
    return false;
  }
  markLive();
  sched = r.data;
  if (sched.now) schedSkew = Date.parse(sched.now) - Date.now();
  const at = (sched.airports || []).join(', ');
  const fs = sched.flights || [];
  $('sStart').innerHTML = `${icon(sched.enabled ? 'i-stop' : 'i-play')}<span>${sched.enabled ? 'Stop schedule' : 'Start schedule'}</span>`;
  $('sStart').className = `btn ${sched.enabled ? '' : 'btn--primary'}`;
  $('schedPill').innerHTML = sched.enabled ? '<span class="dot dot--ok"></span>running' : fs.length ? 'stopped' : 'off';
  $('schedPill').className = `pill ${sched.enabled ? 'pill--ok' : ''}`;
  $('sInfo').textContent = sched.enabled || fs.length ? `${sched.enabled ? 'Running' : 'Stopped'}${at ? ' at ' + at : ''}: ${sched.active} aircraft, ${fs.filter((f) => f.status === 'scheduled').length} scheduled.` : 'Off.';
  $$('#sDensity [data-density]').forEach((b) => b.setAttribute('aria-checked', String(Number(b.dataset.density) === sched.density)));
  if (document.activeElement !== $('sMax')) $('sMax').value = String(sched.maxAircraft);
  if (typeof sched.ifr === 'boolean') $('sIFR').checked = sched.ifr;
  if (typeof sched.vfr === 'boolean') $('sVFR').checked = sched.vfr;
  if (document.activeElement !== $('othersMode') && sched.others) $('othersMode').value = sched.others;
  if (document.activeElement !== $('sAirports') && sched.airports && sched.airports.length && sched.enabled) $('sAirports').value = sched.airports.join(', ');
  const ap = sched.airports || [];
  const bs = $('sBoard');
  if (document.activeElement !== bs && (bs.options.length !== ap.length || [...bs.options].some((o, i) => o.value !== ap[i]))) {
    const keep = bs.value;
    bs.innerHTML = ap.map((a) => `<option>${esc(a)}</option>`).join('');
    if (ap.includes(keep)) bs.value = keep;
    else if (data && ap.includes(data.icao)) bs.value = data.icao;
  }
  bs.hidden = !ap.length;
  renderBoard();
}
function statusChip(f) {
  const gone = f.status === 'done' || f.status === 'cancelled';
  const st = f.status + (f.held ? ' (held)' : '') + (f.note ? ' — ' + f.note : '') + (f.error && f.status === 'cancelled' ? ' — ' + f.error : '');
  const k = f.held ? 'held' : /wait|flow/.test(st) ? 'wait' : gone || f.status === 'scheduled' ? '' : /airborne|enroute|en route|departed|cruise/.test(f.status) ? 'air' : 'move';
  return `<span class="stat${k ? ' stat--' + k : ''}" title="${esc(f.model || '')}">${esc(st)}</span>`;
}
function renderBoard() {
  const el = $('boardTbl');
  const fs = (sched && sched.flights) || [];
  const icao = $('sBoard').value || (data ? data.icao : '');
  const deps = fs.filter((f) => f.kind === 'departure' && f.airport === icao), arrs = fs.filter((f) => f.kind === 'arrival' && f.airport === icao);
  const ovrs = fs.filter((f) => f.kind === 'overflight');
  $('bDepN').textContent = deps.length;
  $('bArrN').textContent = arrs.length;
  $('bOvfN').textContent = ovrs.length;
  if (!tabVisible('schedule')) return;
  const gone = (f) => f.status === 'done' || f.status === 'cancelled';
  const empty = (n) => `<tbody><tr><td colspan="${n}" class="muted">${sched ? 'None.' : 'No schedule (simulator not connected).'}</td></tr></tbody>`;
  let html;
  if (board === 'ovf') {
    html = ovrs.length ? '<thead><tr><th>Enter</th><th>Flight</th><th>Route</th><th>Type</th><th>Status</th></tr></thead><tbody>' + ovrs.map((f) =>
      `<tr class="${gone(f) ? 'is-gone' : ''}"><td class="t">${hhmm(f.enter)}</td><td class="strong">${esc(f.callsign)}</td><td>${esc(f.origin)}→${esc(f.destination)}</td><td>${esc(f.type)}</td><td class="st">${statusChip(f)}</td></tr>`).join('') + '</tbody>' : empty(5);
  } else {
    const dep = board === 'dep', list = dep ? deps : arrs;
    html = list.length ? `<thead><tr><th>${dep ? 'STD' : 'STA'}</th><th>Flight</th><th>${dep ? 'To' : 'From'}</th><th>Type</th><th>Stand</th><th>Status</th></tr></thead><tbody>` + list.map((f) => {
      const t = dep ? f.std : f.sta;
      const est = f.estimated && !f.estimated.startsWith('0001') ? hhmm(f.estimated) : '';
      const turn = dep && f.turnFrom ? ' <span title="turnaround: the arrival\'s aircraft departs again">⟲</span>' : '';
      return `<tr class="${est ? 'is-late' : ''} ${gone(f) ? 'is-gone' : ''}"><td class="t">${hhmm(t)}${est ? `<br><span class="small">→ ${est}</span>` : ''}</td><td class="strong">${esc(f.callsign)}${turn}</td><td>${esc(dep ? f.destination : f.origin)}</td><td>${esc(f.type)}</td><td>${esc(f.stand || '')}</td><td class="st">${statusChip(f)}</td></tr>`;
    }).join('') + '</tbody>' : empty(6);
  }
  setHTML(el, html);
}
async function setSchedule(body) {
  if (!needSim('Schedule')) return;
  const r = await send('/api/schedule', body);
  if (!r.ok) { $('sInfo').textContent = r.error; toast(`Schedule: ${r.error}`, 'err'); }
  schedulePoll.now();
}

/* ───────────── World: the traffic picture ───────────── */
async function pollWorld() {
  const r = await api('/api/world');
  if (!r.ok) { $('wInfo').textContent = r.error; return false; }
  markLive();
  const w = r.data;
  const apts = w.airports.filter((a) => /^[A-Z]{4}$/.test(a.icao));
  // Airports with a proper ICAO code (not heliports and strips), nearest first.
  setHTML($('airportList'), apts.map((a) => `<option value="${esc(a.icao)}">${a.distanceNM.toFixed(0)} NM</option>`).join(''));
  // Every airport in range, nearest first: a click loads it.
  if (!interacting($('aptInRange'))) setHTML($('aptInRange'), apts.length ? `<span class="apt-range__lbl">In range</span>` + apts.map((a) => `<button type="button" class="apt-range__btn" data-apt="${esc(a.icao)}" title="Load ${esc(a.icao)}"><b class="mono">${esc(a.icao)}</b> ${a.distanceNM.toFixed(0)} NM</button>`).join('') : '');
  const phases = {};
  for (const a of w.aircraft) phases[a.phase] = (phases[a.phase] || 0) + 1;
  const where = w.follow ? 'following your aircraft' : `around ${esc(w.icao || 'a position')}`;
  $('wInfo').innerHTML = `${where}, ${w.radiusNM.toFixed(0)} NM: <b>${w.airports.length}</b> airports, <b>${w.aircraft.length}</b> aircraft` +
    (w.aircraft.length ? '<br>' + Object.entries(phases).map(([k, n]) => `${n} ${esc(k)}`).join(' · ') : '') +
    (w.load && w.load.driven ? `<br><span title="Injected aircraft we drive: far away or standing still they get fewer updates (level of detail)">driving ${w.load.driven}: ${w.load.perSecond.toFixed(0)} updates/s, ${w.load.everyFrame} every frame</span>` : '');
  if (document.activeElement !== $('wCentre')) $('wCentre').value = w.follow ? 'follow' : w.icao ? 'airport' : 'position';
  if (document.activeElement !== $('wRadius')) $('wRadius').value = String(w.radiusNM);
  lastWorld = w;
  if (worldOn) drawWorld();
}
async function setWorld(body) {
  if (!needSim('Traffic picture')) return;
  const r = await send('/api/world', body);
  if (!r.ok) toast(`Traffic picture: ${r.error}`, 'err');
  worldPoll.now();
}

/* ───────────── Airport info: runway in use, weather, ATIS, pads ───────────── */
let airportInfo = null;
let deicePads = [];
let padsSig = '';
async function pollAirportInfo() {
  if (!data) return;
  const r = await api(`/api/airportinfo?icao=${encodeURIComponent(data.icao)}`);
  if (!r.ok) return false;
  const a = airportInfo = r.data;
  const lim = a.limits || {};
  // De-icing pads (#323) on the map and in the list.
  deicePads = lim.DeicingPads || [];
  renderPads();
  const mv = a.magVar ? `${Math.abs(a.magVar).toFixed(0)}° ${a.magVar > 0 ? 'E' : 'W'}` : '—';
  const ilsOn = (rwy) => { const i = (a.ils || []).find((x) => x.runway === rwy); return i ? ` <span class="muted">· ILS ${esc(i.ident)} ${i.mhz.toFixed(2)}</span>` : ''; };
  $('apInfo').innerHTML = `<dl class="kv">
    <dt>Name</dt><dd><b>${esc(a.icao)}</b> ${esc(a.name)}</dd>
    <dt>Elevation</dt><dd class="mono">${a.elevationFt.toFixed(0)} ft · var ${mv}</dd>
    ${(a.runways || []).map((x) => `<dt>RWY ${esc(x.name)}</dt><dd class="mono">${x.lengthM.toFixed(0)} × ${x.widthM.toFixed(0)} m${x.approach ? ` <span class="muted">· ${esc(x.approach)}</span>` : ''}${ilsOn(x.name)}</dd>`).join('')}
    <dt>Transition</dt><dd>altitude ${(lim.TransitionAltitudeFt || 0).toFixed(0)} ft${lim.PreferredRunways && lim.PreferredRunways.length ? ' · preferred ' + esc(lim.PreferredRunways.join(', ')) : ''}</dd>
  </dl>`;
  setActiveUse(a.use || null); // none here (no weather): not the last airport's
  const u = a.use, w = a.weather;
  // Strip: runway in use, ATIS, wind, QNH.
  // Parallels used together: "26L+26R"; segregated: departures / arrivals.
  const dl = u ? (u.departures && u.departures.length ? u.departures : [u.departure]).join('+') : '';
  const al = u ? (u.arrivals && u.arrivals.length ? u.arrivals : [u.arrival]).join('+') : '';
  $('rwyChipVal').textContent = u ? (al !== dl ? `${dl}/${al}` : dl) : '—';
  $('rwyChipSub').textContent = u ? (al !== dl ? 'dep / arr' : 'dep · arr') : '';
  $('rwyChip').title = u ? `Runway in use (from the wind): departures ${dl}, arrivals ${al}${u.parallel ? ' (' + u.parallel + ')' : ''}` : 'Runway in use: no weather yet';
  $('atisChip').hidden = !a.atis;
  if (a.atis) {
    $('atisChipLetter').textContent = a.atis.letter[0];
    $('atisChipSr').textContent = `information ${a.atis.letter}`;
    $('atisChip').title = `ATIS information ${a.atis.letter}`;
  }
  const wind = w ? (w.WindKts < 1 ? 'calm' : `${String(Math.round(((w.WindDirTrue - a.magVar) % 360 + 360) % 360)).padStart(3, '0')}°/${w.WindKts.toFixed(0)}${w.GustKts > w.WindKts ? 'G' + w.GustKts.toFixed(0) : ''}`) : '';
  $('windChip').hidden = !w;
  $('qnhChip').hidden = !w;
  $('windChipVal').textContent = wind;
  $('qnhChipVal').textContent = w ? w.QNHhPa.toFixed(0) : '';
  $('moreWind').textContent = wind || '—';
  $('moreQnh').textContent = w ? w.QNHhPa.toFixed(0) : '—';
  // Airport section: runway in use.
  $('useBig').textContent = u ? dl : '—';
  $('useSub').textContent = u ? (al !== dl ? `departures · arrivals ${al}` : 'departures and arrivals') + (u.parallel ? ` · ${u.parallel}` : '') : 'no weather yet (simulator not connected?)';
  // On the runway: headwind or tailwind, and crosswind, whole knots without
  // a sign (from 146° at 6 kt on 24: tailwind 0, crosswind 6 — not "-1").
  const along = u ? (u.headwindKts >= 0 ? `headwind ${Math.round(u.headwindKts)}` : `tailwind ${Math.round(-u.headwindKts)}`) : '';
  const across = u ? `crosswind ${Math.round(Math.abs(u.crosswindKts))} kt` : '';
  // The ILS of the runways landed on, frequencies from the simulator.
  const ilsRows = (a, u) => {
    const arr = (u.arrivals && u.arrivals.length ? u.arrivals : [u.arrival]);
    const rows = (a.ils || []).filter((i) => arr.includes(i.runway));
    return rows.length ? `<dt>ILS</dt><dd class="mono">${rows.map((i) => `${esc(i.runway)} ${esc(i.ident)} ${i.mhz.toFixed(2)}`).join('<br>')}</dd>` : '';
  };
  $('useKv').innerHTML = u ? `<dt>On the runway</dt><dd class="mono">${along}, ${across}</dd>
    <dt>Approach</dt><dd>${esc(u.approach || '—')}</dd>
    ${ilsRows(a, u)}
    <dt>Limits</dt><dd><span class="pill ${u.withinLimits ? 'pill--ok' : 'pill--warn'}">${u.withinLimits ? 'within' : 'outside limits'}</span></dd>` : '';
  // Weather.
  if (w) {
    $('apWx').className = 'small';
    $('apWx').innerHTML = wxHTML();
    // The same in the wind chip's popup, with the runway in use and the ATIS.
    setHTML($('wxPopBody'), wxHTML() + `<dl class="kv">
      <dt>Runway</dt><dd class="mono">${u ? esc(dl === al ? dl : dl + ' / ' + al) : '—'}</dd>
      <dt>ATIS</dt><dd>${a.atis ? 'information ' + esc(a.atis.letter) : '—'}</dd>
    </dl>`);
  }
  function wxHTML() {
    const windTxt = w.WindKts < 1 ? 'calm' : `${String(Math.round(((w.WindDirTrue - a.magVar) % 360 + 360) % 360)).padStart(3, '0')}° ${w.WindKts.toFixed(0)} kt${w.GustKts > w.WindKts ? ' G' + w.GustKts.toFixed(0) : ''}`;
    const vis = w.VisibilityM >= 10000 ? '10 km or more' : `${(w.VisibilityM / 1000).toFixed(1)} km`;
    return `<dl class="kv">
      <dt>Wind</dt><dd class="mono">${windTxt}${u ? ` <span class="muted">· ${along}, ${across}</span>` : ''}</dd>
      <dt>Visibility</dt><dd>${vis}${w.Precip && w.Precip !== 'none' ? ' · ' + esc(w.Precip) : ''}${w.InCloud ? ' · in cloud' : ''}</dd>
      <dt>Temperature</dt><dd class="mono">${w.TempC.toFixed(0)} °C</dd>
      ${w.CeilingFt > 0 ? `<dt>Ceiling</dt><dd class="mono">${Math.round(w.CeilingFt / 100) * 100} ft</dd>` : ''}
      <dt>QNH</dt><dd class="mono">${w.QNHhPa.toFixed(0)} hPa</dd>
    </dl>` + (w.Icing ? `<p class="warn-text">${icon('i-snow', 'ic ic--sm')}Icing: de-icing required.</p>` : `<p class="small muted">${icon('i-snow', 'ic ic--xs')} No icing.</p>`) +
      (w.distanceNM > 20 ? `<p class="warn-text">${icon('i-warn', 'ic ic--sm')}Measured at your aircraft, ${w.distanceNM.toFixed(0)} NM from ${esc(a.icao)}.</p>` : '');
  }
  if (!w) {
    $('apWx').className = 'small muted';
    $('apWx').textContent = 'No weather yet (simulator not connected?). SimConnect reports the weather where your aircraft is, not per airport.';
  }
  // ATIS.
  $('apSpeak').disabled = !a.atis;
  $('apAtisLetter').textContent = a.atis ? a.atis.letter[0] : '–';
  $('apAtisTitle').textContent = a.atis ? `Information ${a.atis.letter}` : 'No ATIS yet';
  $('apAtisText').textContent = a.atis ? a.atis.text : 'The ATIS needs the weather (simulator not connected?).';
}
function renderPads() {
  $('dePads').innerHTML = deicePads.length
    ? deicePads.map((p, i) => `<span class="tag">${icon('i-snow', 'ic ic--xs')}${esc(p.name)} <button type="button" data-pad="${i}" aria-label="Remove ${esc(p.name)}" title="Remove">${icon('i-x', 'ic ic--xs')}</button></span>`).join('')
    : '<span class="muted small">None yet — tick “Pick on the map” and click a taxiway.</span>';
  const sig = JSON.stringify(deicePads);
  if (sig === padsSig) return;
  padsSig = sig;
  layers.pads.clearLayers();
  for (const pad of deicePads) {
    L.marker([pad.position.lat, pad.position.lon], { zIndexOffset: 800, keyboard: false, icon: L.divIcon({ className: 'pad-m', html: icon('i-snow', 'ic ic--sm'), iconSize: [24, 24], iconAnchor: [12, 12] }) })
      .bindTooltip(esc(pad.name), { permanent: true, direction: 'right', offset: [12, 0], className: 'map-lbl' }).addTo(layers.pads);
  }
}
async function savePads(pads) {
  const r = await send(`/api/deicing?icao=${encodeURIComponent(data.icao)}`, pads, 'PUT');
  if (!r.ok) { toast(`De-icing pads: ${r.error}`, 'err'); return; }
  deicePads = r.data || [];
  renderPads();
  airportInfoPoll.now();
}
async function addPadAt(latlng) {
  const r = await api(`/api/node?${new URLSearchParams({ icao: data.icao, lat: latlng.lat, lon: latlng.lng })}`);
  if (!r.ok) { toast(r.error, 'err'); return; }
  let k = deicePads.length + 1;
  while (deicePads.some((p) => p.name === `DI ${k}`)) k++;
  savePads([...deicePads, { name: `DI ${k}`, position: r.data.position }]);
}
async function listenAtis() {
  if (!data || !airportInfo || !airportInfo.atis) return;
  // Through voice-goio (#419), its ATIS voice and radio; the browser's
  // English voice only when the map has no voice.
  const r = await send(`/api/voice/atis?icao=${encodeURIComponent(data.icao)}`);
  if (!r.ok) speakEnglish(airportInfo.atis.spoken);
}

/* ───────────── VFR reporting points (#566) ───────────── */
let vfrPts = null; // GET /api/vfrpoints
function resetVfrPoints() {
  vfrPts = null;
  layers.vfrpts.clearLayers();
  $('vpList').innerHTML = '';
}
async function pollVfrPoints() {
  if (!data || vfrPts) return;
  const r = await api(`/api/vfrpoints?icao=${encodeURIComponent(data.icao)}`);
  if (!r.ok) return;
  vfrPts = r.data || [];
  renderVfrPoints();
}
function renderVfrPoints() {
  $('vpList').innerHTML = vfrPts.length
    ? vfrPts.map((p, i) => `<span class="tag">${esc(p.name)} <button type="button" data-vp="${i}" aria-label="Remove ${esc(p.name)}" title="Remove">${icon('i-x', 'ic ic--xs')}</button></span>`).join('')
    : '<span class="small muted">None yet — tick “Add on the map” and click.</span>';
  layers.vfrpts.clearLayers();
  for (const p of vfrPts) {
    L.marker([p.position.lat, p.position.lon], { icon: L.divIcon({ className: 'm-vfrpt', html: '<span>▲</span>', iconSize: [14, 14], iconAnchor: [7, 7] }), interactive: false, keyboard: false })
      .bindTooltip(esc(p.name), { permanent: true, direction: 'right', offset: [6, 0], className: 'map-lbl map-lbl--sm' })
      .addTo(layers.vfrpts);
  }
}
async function saveVfrPoints(list) {
  const r = await send(`/api/vfrpoints?icao=${encodeURIComponent(data.icao)}`, list);
  if (!r.ok) { toast(r.error, 'err'); return; }
  vfrPts = list.map((p) => ({ ...p, name: p.name.trim().toUpperCase() }));
  renderVfrPoints();
}
function addVfrPointAt(latlng) {
  const name = (prompt('Name of the reporting point (e.g. NOVEMBER):') || '').trim();
  if (!name) return;
  saveVfrPoints([...(vfrPts || []), { name, position: { lat: latlng.lat, lon: latlng.lng } }]);
}

/* ───────────── VFR circuits (#567) ───────────── */
let circuits = null; // GET /api/circuits: by runway end, its config and the C172's circuit
function resetCircuits() {
  circuits = null;
  layers.circuits.clearLayers();
  $('ciInfo').textContent = 'Loading when the Airport section is open…';
}
async function pollCircuits() {
  if (!data || circuits) return;
  const r = await api(`/api/circuits?icao=${encodeURIComponent(data.icao)}`);
  if (!r.ok) { $('ciInfo').textContent = r.error; return; }
  circuits = r.data || {};
  const ends = Object.keys(circuits).sort();
  const was = $('ciRwy').value;
  $('ciRwy').innerHTML = ends.map((e) => `<option>${esc(e)}</option>`).join('');
  if (ends.includes(was)) $('ciRwy').value = was;
  fillCircuitForm();
  drawCircuits();
}
function fillCircuitForm() {
  const e = circuits && circuits[$('ciRwy').value];
  if (!e) return;
  const c = e.config || {};
  $('ciSide').value = c.side || 'left';
  $('ciHeight').value = c.heightFt || '';
  $('ciDownwind').value = c.downwindNM || '';
  $('ciUpwind').value = c.upwindNM || '';
  $('ciBase').value = c.baseNM || '';
  $('ciOverhead').checked = !!c.overheadJoin;
  const ci = e.circuit;
  const set = Object.keys(c).length ? 'set for this airport' : 'the defaults';
  $('ciInfo').textContent = `${ci.side}-hand at ${Math.round(ci.heightFt)} ft MSL, downwind ${ci.downwindNM.toFixed(1)} NM out — ${set}.`;
}
// drawCircuits draws each runway end's circuit, dashed, its legs named; the
// selected runway's bolder.
function drawCircuits() {
  layers.circuits.clearLayers();
  if (!circuits) return;
  const sel = $('ciRwy').value;
  for (const [end, e] of Object.entries(circuits)) {
    const pts = (e.circuit.points || []).map((p) => [p.position.lat, p.position.lon]);
    if (pts.length < 2) continue;
    const on = end === sel;
    L.polyline(pts, { className: `m-circuit${on ? ' is-sel' : ''}`, interactive: false }).addTo(layers.circuits);
    if (!on) continue;
    for (const p of e.circuit.points) {
      if (p.leg === 'runway') continue;
      L.circleMarker([p.position.lat, p.position.lon], { radius: 3, className: 'm-circuit-pt', interactive: false })
        .bindTooltip(`${p.leg} ${Math.round(p.altFt)} ft`, { permanent: true, direction: 'right', offset: [6, 0], className: 'map-lbl map-lbl--sm' })
        .addTo(layers.circuits);
    }
  }
}
async function saveCircuit(reset) {
  if (!data) return;
  const num = (id) => { const v = Number($(id).value); return $(id).value.trim() === '' || !isFinite(v) ? undefined : v; };
  const cfg = reset ? {} : { side: $('ciSide').value === 'right' ? 'right' : undefined, heightFt: num('ciHeight'), downwindNM: num('ciDownwind'), upwindNM: num('ciUpwind'), baseNM: num('ciBase'), overheadJoin: $('ciOverhead').checked || undefined };
  const r = await send(`/api/circuits?icao=${encodeURIComponent(data.icao)}&runway=${encodeURIComponent($('ciRwy').value)}`, cfg);
  if (!r.ok) { toast(r.error, 'err'); return; }
  toast(reset ? 'Circuit back to the defaults' : 'Circuit saved');
  circuits = null;
  pollCircuits();
}

/* ───────────── Procedures (#314): SIDs, STARs, approaches ───────────── */
let procs = null;
let procTries = 0;
function resetProcedures() {
  procs = null;
  procTries = 0;
  layers.procs.clearLayers();
  $('pRunway').innerHTML = '<option value="">All</option>' + runwayEnds().map((e) => `<option>${esc(e)}</option>`).join('');
  $('pInfo').textContent = 'Loading when the Airport section is open…';
  fillProcedurePick();
}
async function pollProcedures() {
  if (!data || procs) return;
  procTries++;
  const r = await api(`/api/procedures?icao=${encodeURIComponent(data.icao)}`);
  if (!r.ok) {
    $('pInfo').textContent = procTries >= 20 ? 'Procedures not available (offline or not loaded).' : 'Loading the procedures…';
    return procTries >= 20 ? true : false;
  }
  procs = r.data;
  // Lists the server leaves null (a procedure without runway transitions).
  for (const p of [...(procs.sids || []), ...(procs.stars || []), ...(procs.approaches || [])]) {
    p.runways = p.runways || [];
    p.paths = (p.paths || []).map((x) => ({ ...x, points: x.points || [] }));
  }
  procs.fixes = procs.fixes || [];
  const navaids = procs.fixes.filter((f) => f.kind === 'V' || f.kind === 'N').length;
  $('pInfo').textContent = `${procs.sids.length} SIDs, ${procs.stars.length} STARs, ${procs.approaches.filter((x) => x.from === 'vectors').length} approaches (${procs.approaches.length} entries) · ${navaids} VOR/NDB shown; ${procs.fixes.length - navaids} waypoints appear with their procedure`;
  fillProcedurePick();
  drawProcedures();
}
// procLabel says what a procedure is: where a SID leads, where a STAR
// starts, an approach's transitions.
function procLabel(kind, p) {
  const rw = p.runways.length ? `RWY ${p.runways.join('/')}` : 'all runways';
  if (kind === 'SID') return `SID ${p.name} · ${rw} → ${p.to || 'runway heading, radar vectors'}`;
  if (kind === 'STAR') return `STAR ${p.name} · ${p.from || 'radar vectors'} → ${p.to ? p.to + ' → ' : ''}${rw}`;
  return p.from === 'vectors' ? `${p.name} · direct, vectors to ${p.to || 'the final'}` : p.name;
}
// One procedure at a time, as a chart shows it, grouped by kind.
function fillProcedurePick() {
  const sel = $('pFilter'), keep = sel.value, rwy = $('pRunway').value;
  const anyKind = $('pSID').checked || $('pSTAR').checked || $('pAPP').checked;
  if (!procs) { sel.innerHTML = `<option value="">${anyKind ? '— not loaded —' : '— tick SIDs, STARs or approaches —'}</option>`; return; }
  const opts = [];
  for (const [kind, title, list, on] of [['SID', 'Departures (SID)', procs.sids, $('pSID').checked], ['STAR', 'Arrivals (STAR)', procs.stars, $('pSTAR').checked], ['APP', 'Approaches', procs.approaches, $('pAPP').checked]]) {
    if (!on) continue;
    const group = [];
    for (const p of list) {
      p.key = kind + ':' + p.name;
      p.kind = kind;
      if (rwy && p.runways.length && !p.runways.includes(rwy)) continue;
      group.push(`<option value="${esc(p.key)}">${esc(procLabel(kind, p))}</option>`);
    }
    if (group.length) opts.push(`<optgroup label="${title}">${group.join('')}</optgroup>`);
  }
  sel.innerHTML = opts.length ? opts.join('') : `<option value="">${anyKind ? '— none for this runway —' : '— tick SIDs, STARs or approaches —'}</option>`;
  if ([...sel.options].some((o) => o.value === keep && keep)) sel.value = keep;
  else sel.selectedIndex = 0;
}
function drawProcedures() {
  layers.procs.clearLayers();
  if (!procs) return;
  const pick = $('pFilter').value, rwy = $('pRunway').value;
  const kinds = { SID: [procs.sids, $('pSID').checked, 'sid'], STAR: [procs.stars, $('pSTAR').checked, 'star'], APP: [procs.approaches, $('pAPP').checked, 'app'] };
  const arrow = (a, b, cls) => {
    const brg = Math.atan2((b.lon - a.lon) * Math.cos(a.lat * Math.PI / 180), b.lat - a.lat) * 180 / Math.PI;
    return L.marker([(a.lat + b.lat) / 2, (a.lon + b.lon) / 2], { interactive: false, keyboard: false, icon: L.divIcon({ className: `proc-ic proc--${cls}`, iconSize: [14, 14], iconAnchor: [7, 7],
      html: `<svg width="14" height="14" viewBox="-7 -7 14 14" style="transform:rotate(${brg}deg)"><path d="M0,-6 L5,5 L0,2 L-5,5 Z" fill="currentColor"/></svg>` }) });
  };
  const symbol = (m, cls) => {
    let svg;
    if (m.kind === 'V') svg = '<polygon points="0,-7 6,-3.5 6,3.5 0,7 -6,3.5 -6,-3.5" fill="none" stroke="currentColor" stroke-width="2"/><circle r="1.5" fill="currentColor"/>';
    else if (m.kind === 'N') svg = '<circle r="6" fill="none" stroke="currentColor" stroke-width="2" stroke-dasharray="2 2"/><circle r="1.5" fill="currentColor"/>';
    else svg = `<polygon points="0,-6 5.5,4 -5.5,4" fill="${m.flyOver ? 'none' : 'currentColor'}" stroke="currentColor" stroke-width="1.5"/>` + (m.flyOver ? '<circle r="7.5" fill="none" stroke="currentColor" stroke-width="1"/>' : '');
    const label = `<b>${esc(m.ident)}</b>${m.constraint ? '<br>' + esc(m.constraint) : ''}`;
    const k = m.kind === 'V' ? 'vor' : m.kind === 'N' ? 'ndb' : cls;
    return L.marker([m.position.lat, m.position.lon], { keyboard: false, icon: L.divIcon({ className: `proc-ic proc--${k}`, iconSize: [18, 18], iconAnchor: [9, 9], html: `<svg width="18" height="18" viewBox="-9 -9 18 18">${svg}</svg>` }) })
      .bindTooltip(label, { permanent: true, direction: 'right', offset: [8, 0], className: 'map-lbl' });
  };
  for (const [list, on, cls] of Object.values(kinds)) {
    if (!on) continue;
    for (const p of list) {
      if (p.key !== pick || (rwy && p.runways.length && !p.runways.includes(rwy))) continue;
      const paths = [...p.paths.map((x) => [x, false]), ...(p.missed || []).map((x) => [x, true])];
      for (const [path, missed] of paths) {
        if (path.points.length < 2) continue;
        if (rwy && path.runway && path.runway !== rwy) continue; // another runway's transition
        L.polyline(path.points.map((q) => [q.lat, q.lon]), { className: `m-proc m-proc--${cls}${missed ? ' m-proc--missed' : ''}` })
          .bindTooltip(esc(path.label), { sticky: true, className: 'node-tip' }).addTo(layers.procs);
        // Direction arrows about every 4 km, the fixes with constraints.
        let run = 0;
        for (let i = 1; i < path.points.length; i++) {
          const a = path.points[i - 1], b = path.points[i];
          run += map.distance([a.lat, a.lon], [b.lat, b.lon]);
          if (run > 4000) { arrow(a, b, cls).addTo(layers.procs); run = 0; }
        }
        for (const m of path.marks || []) symbol(m, cls).addTo(layers.procs);
        // Track and distance along each leg, as on charts.
        for (const t of path.tracks || []) {
          const rot = ((t.bearing + 270) % 180) - 90; // readable: never upside down
          L.marker([t.position.lat, t.position.lon], { interactive: false, keyboard: false, icon: L.divIcon({ className: '', iconSize: [0, 0],
            html: `<div class="trk" style="transform:translate(-50%,-50%) rotate(${rot}deg)">${esc(t.text)}</div>` }) }).addTo(layers.procs);
        }
        // A STAR ending in radar vectors: dashed, a row of arrowheads.
        if (path.vectors && path.vectors.length === 2) {
          const [a, b] = path.vectors;
          L.polyline([[a.lat, a.lon], [b.lat, b.lon]], { className: `m-proc m-proc--${cls} m-proc--vectors` }).bindTooltip(esc(path.vectorsLabel), { sticky: true, className: 'node-tip' }).addTo(layers.procs);
          for (const f of [0.35, 0.55, 0.75, 0.95]) {
            const q = { lat: a.lat + (b.lat - a.lat) * f, lon: a.lon + (b.lon - a.lon) * f };
            arrow(q, { lat: q.lat + (b.lat - a.lat) * 0.01, lon: q.lon + (b.lon - a.lon) * 0.01 }, cls).addTo(layers.procs);
          }
          L.marker([b.lat, b.lon], { interactive: false, keyboard: false, icon: L.divIcon({ className: '', iconSize: [0, 0], html: `<div class="trk">${esc(path.vectorsLabel)}</div>` }) }).addTo(layers.procs);
        }
      }
    }
  }
  if ($('pFix').checked) {
    // Navaids (VOR, NDB) with chart symbols; waypoints come with the procedures.
    for (const f of procs.fixes.filter((x) => x.kind === 'V' || x.kind === 'N')) symbol({ ident: f.ident + (f.kind === 'V' ? ' VOR' : ' NDB'), kind: f.kind, position: f.position }, '').addTo(layers.procs);
  }
}
function fitProcedures() {
  const b = L.latLngBounds([]);
  layers.procs.eachLayer((l) => { if (l.getBounds) b.extend(l.getBounds()); else if (l.getLatLng) b.extend(l.getLatLng()); });
  if (b.isValid()) map.fitBounds(b.pad(0.05));
  else toast('Nothing drawn: pick a procedure first');
}

/* ───────────── The approach sequence (#396) ───────────── */
let apSeqs = [];
const seqWanted = () => tabVisible('sequence') || layerOn.final || (selectedView() && selectedView().kind === 'arrival' && !selectedView().onGround);
async function pollApproach() {
  if (!data) return;
  const full = tabVisible('sequence');
  const [seq, tower, sep] = await Promise.all([
    api('/api/sequence?icao=' + encodeURIComponent(data.icao)),
    full ? api('/api/runways?icao=' + encodeURIComponent(data.icao)) : null,
    full ? api('/api/separation') : null,
  ]);
  if (!seq.ok) return false;
  apSeqs = seq.data || [];
  if (layerOn.final) drawFinal(apSeqs); else { layers.final.clearLayers(); finalSig = ''; }
  const shorts = apSeqs.reduce((n, r) => n + r.sequence.filter((e, i) => i > 0 && e.distanceToGoNM < 20 && e.distanceToGoNM - r.sequence[i - 1].distanceToGoNM < (e.spacingNM || 0) - 0.3).length, 0);
  const b = $('tab-sequence').querySelector('.badge');
  b.textContent = shorts;
  b.hidden = !shorts;
  if (full) renderApproach(tower && tower.ok ? tower.data : {}, sep && sep.ok ? sep.data : null);
  if (selectedView()) renderCtx();
}
function ladderSVG(r) {
  const x0 = 18, x1 = 330, sc = (x1 - x0) / 20, y = 40, seq = r.sequence;
  let s = `<svg class="ladder" viewBox="0 0 340 78" role="img" aria-label="Final approach: arrivals by distance to the threshold">`;
  s += `<rect class="lad-thr" x="${x0 - 12}" y="${y - 3}" width="12" height="6" rx="1"/><line class="lad-axis" x1="${x0}" y1="${y}" x2="${x1}" y2="${y}"/>`;
  for (let nm = 0; nm <= 20; nm++) {
    const x = x0 + nm * sc, big = nm % 5 === 0;
    s += `<line class="lad-tick" x1="${x}" y1="${y - (big ? 5 : 3)}" x2="${x}" y2="${y + (big ? 5 : 3)}"/>`;
    if (big && nm) s += `<text x="${x}" y="${y + 17}" text-anchor="middle">${nm} NM</text>`;
  }
  seq.forEach((e, i) => {
    const d = Math.min(e.distanceToGoNM, 20.5), x = x0 + d * sc;
    const ours = ctlViews.some((v) => v.tail === e.callsign && !v.done);
    const cls = e.fixed ? 'lad-ac--fix' : ours ? 'lad-ac--ours' : 'lad-ac--other';
    if (i > 0 && e.distanceToGoNM <= 20) {
      const lead = seq[i - 1], gap = e.distanceToGoNM - lead.distanceToGoNM, xp = x0 + Math.min(lead.distanceToGoNM, 20) * sc;
      const k = gap < (e.spacingNM || 0) - 0.3 ? 'short' : 'ok';
      s += `<line class="lad-gap--${k}" x1="${xp + 7}" y1="${y + 26}" x2="${Math.max(xp + 8, x - 7)}" y2="${y + 26}" stroke-width="2"/>`;
      s += `<text class="lad-gaptxt--${k}" x="${(x + xp) / 2}" y="${y + 37}" text-anchor="middle">${gap.toFixed(1)}/${e.spacingNM || 0} NM</text>`;
    }
    if (e.distanceToGoNM <= 20.5) {
      s += `<circle class="lad-ac ${cls}" cx="${x}" cy="${y}" r="6"/>`;
      s += `<text class="lad-cs" x="${x}" y="${i % 2 ? 12 : 26}" text-anchor="middle">${esc(e.callsign)}</text>`;
    }
  });
  return s + '</svg>';
}
function renderApproach(tower, sep) {
  const el = $('apSeq');
  if (!interacting(el)) {
    const byTail = Object.fromEntries(ctlViews.filter((v) => !v.done).map((v) => [v.tail, v]));
    setHTML(el, apSeqs.filter((r) => r.sequence.length).map((r) => {
      const rows = r.sequence.map((e, i) => {
        const v = byTail[e.callsign];
        const lead = i > 0 ? r.sequence[i - 1] : null;
        const gap = lead ? e.distanceToGoNM - lead.distanceToGoNM : null;
        const short = lead && e.distanceToGoNM < 20 && gap < (e.spacingNM || 0) - 0.3;
        const sf = !v ? schedFlight(e.callsign) : null;
        let what = v ? v.state : sf ? `en route from ${sf.origin}` : 'other traffic';
        if (v && v.hold) what = `holding at ${v.hold.ident} ${Math.round(v.hold.altFt)} ft`;
        const acts = v ? seqActs(v, { r, i, e }) : [];
        const btns = acts.map((a) => `<button type="button" class="btn ${AP_ACT[a].urgent ? 'btn--urgent' : ''}" data-ap="${a}" data-cs="${esc(e.callsign)}" title="${AP_ACT[a].title}" aria-label="${AP_ACT[a].title}: ${esc(e.callsign)}"${onMyFrequency(v) ? '' : ' disabled'}>${icon(AP_ACT[a].icon, 'ic ic--sm')}</button>`).join('');
        return `<tr class="${e.fixed ? 'is-fix' : ''} ${v || sf ? '' : 'is-other'} ${short ? 'is-short' : ''}">
          <td class="num mono">${e.number}</td>
          <td><span class="mono strong cs" title="${esc(v ? v.model || '' : '')}">${esc(e.callsign)}</span><br><span class="muted small mono" title="wake ${esc(e.wake.icao)}, RECAT-EU ${esc(e.wake.recat)}">${esc(e.wake.icao)}/${esc(e.wake.recat)}</span></td>
          <td class="mono dist" title="${lead ? `${gap.toFixed(1)} NM behind ${esc(lead.callsign)}, ${e.spacingNM} NM needed${e.spacingWhy ? ' — ' + esc(e.spacingWhy) : ''}` : ''}">${e.distanceToGoNM.toFixed(1)}<span class="muted"> NM</span>${lead ? `<br><span class="small">${short ? '▲ ' : ''}${gap.toFixed(1)}/${e.spacingNM}</span>` : ''}${e.delay > 0 ? `<br><span class="small muted">+${Math.round(e.delay / 6e10)} min</span>` : ''}</td>
          <td class="small">${esc(what)}${v || sf ? '' : '<br><span class="muted">not ours</span>'}${btns ? `<div class="seq-acts">${btns}</div>` : ''}</td></tr>`;
      }).join('');
      const c = r.conditions || {};
      return `<div class="card">
        <div class="card__head"><div class="seqcard__cap"><span class="field__lbl">RWY</span><span class="mono">${esc(r.runway)}</span>${r.lvp ? '<span class="pill pill--warn">LVP</span>' : ''}</div><span class="muted small">${c.visibilityM ? Math.round(c.visibilityM / 100) / 10 + ' km · ' : ''}${esc(c.surface || '')}</span></div>
        ${ladderSVG(r)}
        <table class="tbl seq-tbl"><tbody>${rows}</tbody></table></div>`;
    }).join('') || `<div class="card"><p class="muted small">${simLive ? 'No arrivals in the sequence.' : 'No arrivals: the simulator is not connected.'}</p></div>`);
  }
  $('apTower').innerHTML = Object.entries(tower).map(([rwy, us]) => `<p><span class="mono strong">${esc(rwy)}</span> · ${us.map((u) => `${esc(u.callsign)} ${esc(u.phase)}${u.waiting ? ' <i class="muted">(' + esc(u.waiting) + ')</i>' : ''}`).join(', ')}</p>`).join('') || '<span class="muted">Nobody at the runways.</span>';
  if (sep) {
    const cs = (sep.conflicts || []).map((c) => `<li><span class="mono strong">${esc(c.A)}–${esc(c.B)}</span>: ${c.closestNM.toFixed(1)} NM, ${Math.round(c.verticalFt)} ft in ${Math.round(c.closestIn / 1e9)} s</li>`);
    const rs = (sep.resolutions || []).slice(-5).reverse().map((r) => `<li>${esc(r.said)}</li>`);
    $('apConf').innerHTML = (cs.length ? `<ul class="plain">${cs.join('')}</ul>` : '<p class="muted">No conflicts predicted.</p>') + (rs.length ? `<p><strong>Resolved</strong></p><ul class="plain">${rs.join('')}</ul>` : '');
    $('apConfN').textContent = cs.length;
    $('apConfN').className = `badge${cs.length ? ' badge--danger' : ''}`;
  }
}

/* ───────────── Radio and voice (#419, #425) ───────────── */
let rdFreq = store.get('airportMapRadioFreq', '');
let rdSoundOn = false, rdSync = false;
let voice = null;
let radioCache = [];
const rdKinds = { atis: 'ATIS', clearance: 'Delivery', ground: 'Ground', tower: 'Tower', approach: 'Approach', departure: 'Departure', center: 'Centre', ctaf: 'CTAF' };
// Frequencies as the radio stamps them: 121.910 → "121.91".
const rdMHz = (mhz) => { let s = mhz.toFixed(3); if (s.endsWith('0')) s = s.slice(0, -1); return s; };
// The stations as worked (#722): several per position, one controller
// on several frequencies at night.
let stationsCache = [];
const stPos = { delivery: 'Delivery', ground: 'Ground', tower: 'Tower', approach: 'Approach', departure: 'Departure', center: 'Centre' };
// The frequencies: the stations, the airport's others (ATIS), then any
// heard that it does not list.
function radioFreqs() {
  const out = [], seen = new Set();
  for (const s of stationsCache) {
    if (!s.freq || seen.has(s.freq)) continue;
    seen.add(s.freq);
    const many = stationsCache.filter((o) => o.position === s.position && o.freq !== s.freq).length > 0;
    const label = many ? s.name : (stPos[s.position] || s.position);
    // One controller on other frequencies too: worked together.
    const with_ = [...new Set(stationsCache.filter((o) => s.controller && o.controller === s.controller && o.freq !== s.freq).map((o) => stPos[o.position] || o.position))];
    out.push({ mhz: s.freq, label, with: with_ });
  }
  for (const f of (data && data.frequencies) || []) {
    const mhz = rdMHz(f.mhz);
    if (seen.has(mhz)) continue;
    seen.add(mhz);
    out.push({ mhz, label: rdKinds[f.kind] || f.kind });
  }
  for (const t of radioCache) if (t.frequency && !seen.has(t.frequency)) { seen.add(t.frequency); out.push({ mhz: t.frequency, label: t.position }); }
  return out;
}
const radioWanted = () => tabVisible('radio') || !!selectedView();
async function pollRadio() {
  if (!data) return;
  const r = await api(`/api/radio?icao=${encodeURIComponent(data.icao)}&n=200`);
  if (!r.ok) return false;
  radioCache = r.data || [];
  const st = await api(`/api/stations?icao=${encodeURIComponent(data.icao)}`);
  if (st.ok) stationsCache = (st.data && st.data.stations) || [];
  renderRadio();
  if (selectedView()) renderCtx();
}
function txHTML(t) {
  const kind = t.intent === 'atis' ? 'atis' : t.pilot ? 'pilot' : 'atc';
  const who = t.pilot ? t.callsign : (t.position === 'atis' ? 'ATIS' : t.position);
  return `<div class="tx tx--${kind}"><span class="tx__t">${esc((t.at || '').slice(11, 19))}</span><span class="tx__who">${esc(who)}</span><span class="tx__text">${esc(t.text)}</span></div>`;
}
function renderRadio() {
  const freqs = radioFreqs();
  // One frequency, as on a receiver (#462): the tower until one is picked;
  // following COM1, whatever it is tuned to.
  if (!rdSync && (!rdFreq || (freqs.length && !freqs.some((f) => f.mhz === rdFreq)))) {
    const tower = freqs.find((f) => f.label === 'Tower') || freqs[0];
    if (tower && tower.mhz !== rdFreq) { rdFreq = tower.mhz; if (rdSoundOn) setVoice(true); }
  }
  updateRadioChip(freqs);
  if (!tabVisible('radio')) return;
  const count = {};
  for (const v of ctlViews) if (!v.done && v.frequency) count[v.frequency] = (count[v.frequency] || 0) + 1;
  const com1 = voice && voice.com1;
  if (!interacting($('rdFreqs'))) setHTML($('rdFreqs'), freqs.map((f) => `<button type="button" role="radio" class="freq${com1 && com1 === f.mhz ? ' is-com1' : ''}" aria-checked="${f.mhz === rdFreq}" data-f="${esc(f.mhz)}" title="${count[f.mhz] || 0} aircraft on ${esc(f.mhz)}">
    <span class="freq__pos">${esc(f.label)}</span><span class="freq__mhz">${esc(f.mhz)}</span><span class="freq__n">${icon('i-jet', '')}${count[f.mhz] || 0}</span>${f.with && f.with.length ? `<span class="freq__with" title="One controller works ${esc(f.label)} with ${esc(f.with.join(', '))}">with ${esc(f.with.join(', '))}</span>` : ''}</button>`).join('') || '<p class="muted small">No frequencies known for this airport.</p>');
  const log = $('rdLog');
  // Newest first: the latest call on top.
  const shown = radioCache.filter((t) => t.frequency === rdFreq).reverse();
  const atTop = log.scrollTop <= 20;
  setHTML(log, shown.map(txHTML).join('') || `<p class="muted small rlog__empty">${simLive ? 'Nothing said yet on this frequency.' : 'Nothing said: the simulator is not connected.'}</p>`);
  if (atTop) log.scrollTop = 0;
  const label = (freqs.find((f) => f.mhz === rdFreq) || {}).label || '';
  $('rdNow').innerHTML = rdSoundOn ? `<span class="eq" aria-hidden="true"><i></i><i></i><i></i></span>Listening on ${esc(label)} ${esc(rdFreq)}${rdSync ? ' (my COM1)' : ''}`
    : `<span class="muted">${esc(voice && voice.status && voice.status !== 'off' ? voice.status : 'Voice off: transcript only')}</span>`;
}
// shortPos: a position as a strip shows it ("Praha Tower" → TWR).
function shortPos(label) {
  const k = [[/tower/i, 'TWR'], [/ground/i, 'GND'], [/deliver|clearance/i, 'DEL'], [/approach/i, 'APP'], [/departure/i, 'DEP'], [/cent/i, 'CTR'], [/atis/i, 'ATIS'], [/ctaf/i, 'CTAF']].find(([re]) => re.test(label));
  return k ? k[1] : String(label).slice(0, 4).toUpperCase();
}
function updateRadioChip(freqs = radioFreqs()) {
  const f = freqs.find((x) => x.mhz === rdFreq);
  $('radioChipPos').textContent = f ? shortPos(f.label) : 'RADIO';
  $('radioChipFreq').textContent = rdFreq || '—';
  $('radioChipIc').setAttribute('href', rdSoundOn ? '#i-vol' : '#i-mute');
  $('radioChip').dataset.muted = String(!rdSoundOn);
  $('radioChip').title = `${rdSoundOn ? 'Listening' : 'Voice off'}${f ? ` · ${f.label} ${f.mhz}` : ''}`;
}
function showVoice(v) {
  if (!v) return;
  voice = v;
  rdSoundOn = !!v.on;
  rdSync = !!v.syncCom;
  $('rdSync').checked = rdSync;
  // The frequency heard: COM1's when following it, the voice's while it
  // plays, else the one picked here.
  if (v.frequency && v.frequency !== rdFreq && (rdSync || v.on || !rdFreq)) rdFreq = v.frequency;
  // The outputs, once known (the sound has been on): the voice plays on the one picked.
  const dev = $('rdDevice');
  if (v.devices && v.devices.length) {
    const opts = v.devices.map((d) => `<option value="${esc(d.ID || '')}">${esc(d.Name)}</option>`).join('');
    if (dev.dataset.opts !== opts && document.activeElement !== dev) { dev.innerHTML = opts; dev.dataset.opts = opts; }
    if (document.activeElement !== dev) dev.value = v.device || '';
  }
  // The output only while the voice is on and plays here, on the map's
  // computer (not with Play on this device).
  $('rdDeviceWrap').hidden = !(rdSoundOn && v.devices && v.devices.length && !$('rdHere').checked);
  const b = $('rdSound');
  b.setAttribute('aria-pressed', String(rdSoundOn));
  b.innerHTML = `${icon(rdSoundOn ? 'i-vol' : 'i-mute')}<span>${rdSoundOn ? 'Voice on' : 'Voice off'}</span>`;
  renderRadio();
}
// savedDevice is the output picked before, remembered in this browser.
function savedDevice() { const d = store.get('airportMapVoiceDevice'); return d === null ? {} : { device: d }; }
async function setVoice(on, tune = false) {
  const r = await send('/api/voice', { on, frequency: rdFreq, syncCom: rdSync, tune, ...savedDevice() });
  if (r.ok) showVoice(r.data);
  else toast(`Voice: ${r.error}`, 'err');
}
async function pollVoice() {
  const r = await api('/api/voice');
  if (!r.ok) return false;
  showVoice(r.data);
}
// tuneTo follows a frequency: the radio's tiles, an aircraft's frequency chip.
function tuneTo(freq) {
  rdFreq = freq;
  store.set('airportMapRadioFreq', rdFreq);
  // Following COM1 or tuning it, a frequency picked here tunes it.
  setVoice(rdSoundOn, rdSync || $('rdTuneCom').checked);
  renderRadio();
  renderStrips();
  renderCtx();
  radioPoll.now();
}

/* ───────────── Wiring ───────────── */
function initSections() {
  // Pausing stops the simulator for everyone playing: asked first (no key
  // for it, the button only).
  $('simPause').addEventListener('click', () => {
    const paused = lastOwn && lastOwn.paused;
    if (!confirm(paused ? 'Resume the simulation?' : 'Pause the simulation? It stops for everyone playing.')) return;
    simAct(paused ? 'resume' : 'pause');
  });
  document.addEventListener('click', (e) => {
    if (e.target.closest('[data-resume]')) simAct('resume');
    const r = e.target.closest('[data-rate]');
    if (r) simAct(r.dataset.rate);
  });
  guardPress($('trafficTable'), () => renderOthers(lastTraffic));
  guardPress($('apSeq'), () => approachPoll.now());
  $('trafficTable').addEventListener('click', async (e) => {
    const b = e.target.closest('button.rmOther');
    if (b) {
      const row = b.closest('tr');
      if (!confirm(`Remove ${row.dataset.name} from the simulator?`)) return;
      const r = await send('/api/world/remove', { objectId: Number(b.dataset.oid) });
      if (!r.ok) toast(r.error, 'err');
      trafficPoll.now();
      return;
    }
    const row = e.target.closest('tr[data-lat]');
    if (row) locate(Number(row.dataset.lat), Number(row.dataset.lon), row.dataset.name);
  });
  $('gStart').addEventListener('click', toggleGame);
  $('sStart').addEventListener('click', () => {
    if (sched && sched.enabled) { setSchedule({ enabled: false }); return; }
    if (!data) { toast('Load an airport first', 'err'); return; }
    const airports = $('sAirports').value.toUpperCase().split(/[\s,;]+/).filter((a) => /^[A-Z0-9]{3,4}$/.test(a));
    const density = Number(($('sDensity').querySelector('[aria-checked="true"]') || {}).dataset?.density || 1);
    setSchedule({ enabled: true, airports: airports.length ? airports : [data.icao], density, maxAircraft: Number($('sMax').value), ifr: $('sIFR').checked, vfr: $('sVFR').checked });
    board = 'dep';
  });
  $('sDensity').addEventListener('click', (e) => {
    const b = e.target.closest('[data-density]');
    if (!b) return;
    $$('#sDensity [data-density]').forEach((x) => x.setAttribute('aria-checked', String(x === b)));
    setSchedule({ density: Number(b.dataset.density) });
  });
  $('sMax').addEventListener('change', () => setSchedule({ maxAircraft: Number($('sMax').value) }));
  $('sBoard').addEventListener('change', renderBoard);
  $('p-schedule').addEventListener('click', (e) => {
    const b = e.target.closest('[data-board]');
    if (!b) return;
    board = b.dataset.board;
    $$('[data-board]').forEach((x) => x.setAttribute('aria-selected', String(x === b)));
    renderBoard();
  });
  $('othersMode').addEventListener('change', () => setSchedule({ others: $('othersMode').value }));
  $('sIFR').addEventListener('change', () => setSchedule({ ifr: $('sIFR').checked }));
  $('sVFR').addEventListener('change', () => setSchedule({ vfr: $('sVFR').checked }));
  $('wCentre').addEventListener('change', () => {
    const v = $('wCentre').value;
    if (v === 'follow') setWorld({ follow: true });
    else if (v === 'position') { const c = map.getCenter(); setWorld({ lat: c.lat, lon: c.lng }); }
    else setWorld({ icao: data ? data.icao : '' });
  });
  $('wRadius').addEventListener('change', () => setWorld({ radiusNM: Number($('wRadius').value) }));
  $('apSpeak').addEventListener('click', listenAtis);
  $('dePads').addEventListener('click', (e) => {
    const b = e.target.closest('button[data-pad]');
    if (b) savePads(deicePads.filter((_, i) => i !== Number(b.dataset.pad)));
  });
  $('dePick').addEventListener('change', () => { if ($('dePick').checked) { $('rViaPick').checked = false; toast('Click a taxiway on the map to add a de-icing pad'); } });
  $('rViaPick').addEventListener('change', () => { if ($('rViaPick').checked) $('dePick').checked = false; });
  for (const id of ['pRunway', 'pSID', 'pSTAR', 'pAPP']) $(id).addEventListener('change', () => { fillProcedurePick(); drawProcedures(); });
  for (const id of ['pFix', 'pFilter']) $(id).addEventListener('change', drawProcedures);
  $('pFit').addEventListener('click', fitProcedures);
  $('ciRwy').addEventListener('change', () => { fillCircuitForm(); drawCircuits(); });
  // The circuits card's switch is the Map tab's VFR circuits layer (off by default).
  $('p-airport').addEventListener('change', (e) => { const k = e.target.dataset.layer; if (k) setLayer(k, e.target.checked); });
  $('vpList').addEventListener('click', (e) => {
    const b = e.target.closest('button[data-vp]');
    if (b) saveVfrPoints(vfrPts.filter((_, i) => i !== Number(b.dataset.vp)));
  });
  $('vpPick').addEventListener('change', () => { if ($('vpPick').checked) { $('dePick').checked = false; $('rViaPick').checked = false; toast('Click on the map where the reporting point is'); } });
  $('ciSave').addEventListener('click', () => saveCircuit(false));
  $('ciReset').addEventListener('click', () => saveCircuit(true));

  $('rdSound').addEventListener('click', () => {
    // On the map's own computer the voice and "Play on this device" would
    // echo each other: one or the other.
    if (!rdSoundOn && isLocalHost && $('rdHere').checked) {
      $('rdHere').checked = false;
      $('rdHere').dispatchEvent(new Event('change'));
      toast('Voice on this computer: playing in the browser is off (it would echo)');
    }
    setVoice(!rdSoundOn);
  });
  $('rdDevice').addEventListener('change', async (e) => {
    store.set('airportMapVoiceDevice', e.target.value);
    const r = await send('/api/voice', { on: rdSoundOn, frequency: rdFreq, syncCom: rdSync, device: e.target.value });
    if (r.ok) showVoice(r.data);
  });
  $('rdSync').addEventListener('change', (e) => { rdSync = e.target.checked; setVoice(rdSoundOn); voicePoll.now(); });
  // Tune my COM1: remembered in this browser; on, the frequency followed now is tuned.
  $('rdTuneCom').checked = store.get('airportMapTuneCom') === '1';
  $('rdTuneCom').addEventListener('change', (e) => {
    store.set('airportMapTuneCom', e.target.checked ? '1' : '0');
    if (e.target.checked && rdFreq) setVoice(rdSoundOn, true);
  });
  guardPress($('rdFreqs'), renderRadio);
  $('rdFreqs').addEventListener('click', (e) => { const b = e.target.closest('[data-f]'); if (b) tuneTo(b.dataset.f); });

  // Map section: layers, base map, raw path types.
  $('p-map').addEventListener('change', (e) => {
    const k = e.target.dataset.layer;
    if (k) {
      setLayer(k, e.target.checked);
      if (k === 'ours' || k === 'others') { for (const m of acMarkers.values()) m.remove(); acMarkers.clear(); trafficPoll.now(); if (worldOn) drawWorld(); }
      if (k === 'safe') trafficPoll.now();
      if (k === 'routes') drawRoutes();
      if (k === 'final') { finalSig = ''; if (e.target.checked) approachPoll.now(); }
      if (k === 'occupied') standsPoll.now();
      if (k === 'follow') toast(e.target.checked ? 'The map follows my aircraft' : 'Map follow off');
      return;
    }
    if (e.target.name === 'base') { setBase(e.target.value); return; }
    if (e.target.dataset.path !== undefined) setPathType(e.target.dataset.path, e.target.checked);
  });
}

// The path and point TYPE tables, with a switch per path type.
function renderTypeTables() {
  const pc = {}, qc = {};
  for (const p of data.taxiPaths) pc[p.type] = (pc[p.type] || 0) + 1;
  for (const p of data.taxiPoints) qc[p.type] = (qc[p.type] || 0) + 1;
  $('pathTypes').innerHTML = Object.keys(pc).sort((a, b) => a - b).map((t) => {
    const [tok, , , dash] = PATH_STYLE[t] || PATH_STYLE[0];
    return `<label class="switch"><input type="checkbox" data-path="${t}"${pathOn[t] ? ' checked' : ''}><span class="switch__ui" aria-hidden="true"></span><span class="sw${dash ? ' sw--dash' : ''}" style="background:var(${tok})"></span>${t} ${PATH_TYPES[t] || '(' + t + ')'} <span class="muted mono small">${pc[t]}</span></label>`;
  }).join('') || '<p class="muted small">none</p>';
  $('pointTypes').innerHTML = '<tbody>' + (Object.keys(qc).sort((a, b) => a - b).map((t) => `<tr><td><span class="dot" style="background:var(${HOLD_SHORT.has(Number(t)) ? '--holdbar' : '--text-3'})"></span> ${t} ${POINT_TYPES[t] || '(' + t + ')'}</td><td class="n mono">${qc[t]}</td></tr>`).join('') || '<tr><td class="muted">none</td></tr>') + '</tbody>';
}

/* ───────────── Tower: where the tower camera looks from ───────────── */
let towerInfo = null;     // GET /api/tower
let towerMarker = null;
const TOWER_SOURCE = { yours: 'set by you', known: 'known position', simulator: "the simulator's", airport: 'none known: over the airport' };
async function loadTower() {
  if (!data) return;
  const r = await api(`/api/tower?icao=${encodeURIComponent(data.icao)}`);
  if (r.ok) showTower(r.data);
}
function showTower(t) {
  towerInfo = t;
  $('twSource').textContent = TOWER_SOURCE[t.source] || t.source;
  if (document.activeElement !== $('twCab')) $('twCab').value = String(Math.round(t.cabM));
  $('twCabVal').textContent = `${Math.round(t.cabM)} m`;
  $('twPos').textContent = `${t.lat.toFixed(5)}, ${t.lon.toFixed(5)}`;
  const icon = L.divIcon({ className: 'm-tower', html: icon_('i-tower'), iconSize: [26, 26], iconAnchor: [13, 13] });
  if (!towerMarker) towerMarker = L.marker([t.lat, t.lon], { icon, interactive: false, keyboard: false }).addTo(map);
  else towerMarker.setLatLng([t.lat, t.lon]);
  drawAngle(); // the cone starts at the tower, known only now
}
function icon_(id) { return `<svg class="ic"><use href="#${id}"/></svg>`; }
async function saveTower(body) {
  if (!data) return;
  const r = await send('/api/tower', { icao: data.icao, ...body });
  if (!r.ok) { toast(`Tower: ${r.error}`, 'err'); return; }
  showTower(r.data);
  // A tower camera on air moves there at once.
  if (camView && (camView.mode === 'tower' || camView.mode === 'view' && camView.shot === 'tower')) camViewOf('tower', ctlSelected || -1);
}
function initTower() {
  $('twCab').addEventListener('input', () => { $('twCabVal').textContent = `${$('twCab').value} m`; });
  $('twCab').addEventListener('change', () => saveTower({ cabM: Number($('twCab').value) }));
  $('twReset').addEventListener('click', () => saveTower({ reset: true }));
  let placing = null; // the pending map click; cancelled when toggled off
  $('twPlace').addEventListener('click', () => {
    const on = $('twPlace').getAttribute('aria-pressed') !== 'true';
    $('twPlace').setAttribute('aria-pressed', String(on));
    map.getContainer().classList.toggle('is-placing', on);
    if (placing) { map.off('click', placing); placing = null; }
    if (!on) return;
    toast('Click the tower on the map');
    placing = (e) => {
      placing = null;
      $('twPlace').setAttribute('aria-pressed', 'false');
      map.getContainer().classList.remove('is-placing');
      saveTower({ lat: e.latlng.lat, lon: e.latlng.lng });
    };
    map.once('click', placing);
  });
}

/* ───────────── Tower look: keys and the middle mouse button ───────────── */
// Only while the tower looks round (Tower, no aircraft selected): arrows turn
// it, + and − zoom (Shift: bigger steps); the middle button held and dragged
// turns it, the wheel while held zooms. Steps go out together every 80 ms.
const lookPending = { yaw: 0, tilt: 0, fov: 0 };
let lookFlush = 0;
function towerLooking() { return !!camView && camView.mode === 'tower'; }
function queueLook(yaw, tilt, fov) {
  lookPending.yaw += yaw; lookPending.tilt += tilt; lookPending.fov += fov;
  if (lookFlush) return;
  lookFlush = setTimeout(() => {
    lookFlush = 0;
    const { yaw: y, tilt: t, fov: f } = lookPending;
    lookPending.yaw = lookPending.tilt = lookPending.fov = 0;
    if (y || t || f) send('/api/camera', { mode: 'look', yaw: y, tilt: t, fov: f }).then((r) => { if (r.ok) { camView = r.data; drawAngle(); } });
  }, 80);
}
document.addEventListener('keydown', (e) => {
  if (!towerLooking() || e.ctrlKey || e.metaKey || e.altKey) return;
  if (e.target.closest && e.target.closest('input, select, textarea, [contenteditable]')) return;
  const k = e.shiftKey ? 3 : 1;
  const step = { ArrowLeft: [-3, 0, 0], ArrowRight: [3, 0, 0], ArrowUp: [0, 1, 0], ArrowDown: [0, -1, 0], '+': [0, 0, -3], '=': [0, 0, -3], '-': [0, 0, 3], '_': [0, 0, 3] }[e.key];
  if (!step) return;
  e.preventDefault();
  e.stopPropagation(); // not the map's own arrow-key panning
  queueLook(step[0] * k, step[1] * k, step[2] * k);
}, true);
let midLook = null; // the middle button held: where the pointer was
// A double click of the middle button locks the look on: the mouse turns the
// tower and the wheel zooms it with no button held, until another middle
// click or Escape.
let midLocked = false, midDownAt = 0;
function unlockLook(el) {
  midLocked = false;
  midLook = null;
  el.classList.remove('is-looking', 'is-look-locked');
}
function initTowerMouse() {
  const el = map.getContainer();
  window.addEventListener('keydown', (e) => { if (e.key === 'Escape' && midLocked) unlockLook(el); });
  // Caught on the window, before the map or the browser act on it (no map
  // drag, no auto-scroll, no middle-click paste or link).
  window.addEventListener('mousedown', (e) => {
    if (e.button !== 1 || !towerLooking() || !el.contains(e.target)) return;
    e.preventDefault();
    e.stopPropagation();
    if (midLocked) { unlockLook(el); midDownAt = 0; return; }
    const now = Date.now();
    if (now - midDownAt < 350) {
      midLocked = true;
      el.classList.add('is-look-locked');
      toast('View locked to the mouse: middle click or Esc to unlock');
    }
    midDownAt = now;
    midLook = { x: e.clientX, y: e.clientY };
    el.classList.add('is-looking');
  }, true);
  window.addEventListener('auxclick', (e) => { if (e.button === 1 && towerLooking() && el.contains(e.target)) { e.preventDefault(); e.stopPropagation(); } }, true);
  window.addEventListener('mousemove', (e) => {
    if (!midLook) return;
    if (midLocked && !towerLooking()) { unlockLook(el); return; }
    const dx = e.clientX - midLook.x, dy = e.clientY - midLook.y;
    midLook = { x: e.clientX, y: e.clientY };
    queueLook(dx * 0.15, -dy * 0.1, 0);
  });
  window.addEventListener('mouseup', (e) => {
    if (e.button === 1 && midLook && !midLocked) { midLook = null; el.classList.remove('is-looking'); }
  });
  // The wheel while the middle button is held zooms the tower, not the map.
  window.addEventListener('wheel', (e) => {
    if (!midLook) return;
    e.preventDefault();
    e.stopPropagation();
    queueLook(0, 0, Math.sign(e.deltaY) * 2);
  }, { capture: true, passive: false });
}

/* ───────────── Tower look: the angle on the map ───────────── */
// Where the tower camera looks, drawn from the tower: a cone of its field of
// view, with its bearing, tilt and width. Off by default (the Angle switch).
let showAngle = store.get('airportMapTowerAngle') === '1';
let angleLayer = null;
document.addEventListener('change', (e) => {
  if (!e.target.matches('[data-angle]')) return;
  showAngle = e.target.checked;
  store.set('airportMapTowerAngle', showAngle ? '1' : '0');
  drawAngle();
});
function drawAngle() {
  const k = camView && camView.mode === 'tower' && camView.look;
  if (!showAngle || !k || !towerInfo) {
    if (angleLayer) { angleLayer.remove(); angleLayer = null; }
    return;
  }
  const R = 1500; // meters on the map
  const pts = [[towerInfo.lat, towerInfo.lon]];
  for (let i = 0; i <= 12; i++) {
    const b = (k.yaw - k.fov / 2 + (k.fov * i) / 12) * Math.PI / 180;
    pts.push(offset(towerInfo.lat, towerInfo.lon, Math.sin(b) * R, Math.cos(b) * R));
  }
  const label = `${String(Math.round(((k.yaw % 360) + 360) % 360)).padStart(3, '0')}° · tilt ${k.tilt.toFixed(0)}° · ${k.fov.toFixed(0)}°`;
  if (!angleLayer) {
    angleLayer = L.polygon(pts, { className: 'm-angle', interactive: false }).addTo(map);
    angleLayer.bindTooltip(label, { permanent: true, direction: 'center', className: 'map-lbl' });
  } else {
    angleLayer.setLatLngs(pts);
    angleLayer.setTooltipContent(label);
  }
}
