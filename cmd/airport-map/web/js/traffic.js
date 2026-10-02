/* Traffic: our controlled aircraft (strips, the context panel, clearances)
   and the New flight form (route preview, entries and exits, spawn). */
'use strict';

let ctlViews = [];      // GET /api/control
let ctlSelected = 0;    // the selected aircraft's control ID; 0: none
let ctlFollow = 0;      // the map keeps this aircraft in the middle
const ctlBusy = new Set(); // aircraft with a command on its way: their buttons wait
const waitSince = new Map(); // control ID → when it started waiting (seen here)

const WAITING = /awaiting|holding short|lined up/;
const waits = (v) => !v.done && (WAITING.test(v.state) || v.atLimit);
const isDone = (v) => v.done || v.state === 'complete';
const has = (v, a) => !!(v.actions && v.actions.includes(a));
const selectedView = () => ctlViews.find((x) => x.id === ctlSelected);
const selectedTail = () => (selectedView() || {}).tail;

// schedFlight is the scheduled flight flying under a call sign now.
const schedFlight = (tail) => sched && tail ? (sched.flights || []).find((f) => f.callsign === tail && ['done', 'cancelled', 'scheduled'].indexOf(f.status) < 0) : null;
// What an aircraft of ours is: under a controller of ours (it has a strip),
// an arrival en route to its STAR entry, an overflight, or a departure
// flying on after its SID — the last three flown by MSFS AI on their plans.
function trafficKind(t) {
  if (!t.ours) return 'other';
  if (ctlViews.some((v) => v.tail === t.tail && !isDone(v))) return 'controlled';
  const f = schedFlight(t.tail);
  if (f && f.kind === 'overflight') return 'overflight';
  if (f && f.kind === 'arrival') return 'enroute';
  if (f && f.kind === 'departure') return 'departed';
  return 'controlled';
}

/* ───────────── Clearances ───────────── */
const ACT = {
  pushback: { label: 'Pushback', short: 'Push', phase: 'ground', icon: 'i-push' },
  startup: { label: 'Start up', short: 'Start', phase: 'ground' },
  taxi: { label: 'Taxi', short: 'Taxi', phase: 'ground' },
  cross: { label: 'Cross', short: 'Cross', phase: 'ground' },
  depart: { label: 'Depart now', short: 'Depart', phase: 'ground' },
  lineup: { label: 'Line up', short: 'Line up', phase: 'runway' },
  lineupbehind: { label: 'Line up behind landing', short: 'Behind', phase: 'runway' },
  takeoff: { label: 'Take-off', short: 'Take-off', phase: 'runway' },
  land: { label: 'Cleared to land', short: 'Land', phase: 'approach' },
  hold: { label: 'Hold position', short: 'Hold', urgent: true, icon: 'i-hand' },
  goaround: { label: 'Go around', short: 'Go around', urgent: true, icon: 'i-goaround' },
  abort: { label: 'Abort take-off', short: 'Abort', urgent: true, icon: 'i-abort' },
};
// The approach sequence's controls (POST /api/approach/…).
const AP_ACT = {
  up: { label: 'Earlier', title: 'Earlier in the sequence', icon: 'i-up' },
  down: { label: 'Later', title: 'Later in the sequence', icon: 'i-down' },
  direct: { label: 'Direct', title: 'Direct to the final', icon: 'i-direct' },
  slow: { label: 'Slow', title: 'Lose a minute: speed, then a dog-leg', icon: 'i-slow' },
  hold: { label: 'Hold fix', title: 'Hold at the STAR fix', icon: 'i-hold' },
  release: { label: 'Leave hold', title: 'Leave the hold', icon: 'i-release' },
  goaround: { label: 'Go around', title: 'Go around', icon: 'i-goaround', urgent: true },
};
const PHASES = [
  { key: 'ground', name: 'Ground', acts: ['pushback', 'startup', 'taxi', 'cross', 'depart'] },
  { key: 'runway', name: 'Runway', acts: ['lineup', 'lineupbehind', 'takeoff'] },
];
const URGENT = ['hold', 'goaround', 'abort'];
const NEXT_ORDER = ['land', 'pushback', 'startup', 'taxi', 'cross', 'lineup', 'takeoff', 'lineupbehind', 'depart'];
const FACING = { n: 'north', e: 'east', s: 'south', w: 'west' };

const actLabel = (v, a, short) => a === 'taxi' && v.atLimit && v.limitNode < 0 ? 'Continue taxi' : ACT[a] ? (short ? ACT[a].short : ACT[a].label) : a;
const candidates = (v) => NEXT_ORDER.filter((a) => has(v, a));
// nextAction: the clearance the aircraft waits for, or (not moving on
// the ground) the first one it can take.
function nextAction(v) {
  if (isDone(v)) return null;
  const c = candidates(v);
  if (!c.length) return null;
  return waits(v) || !has(v, 'hold') ? c[0] : null;
}
function statusText(v) {
  return [v.state, v.deicing && 'de-icing', v.pushbackHeld && 'waiting for traffic behind', v.holdingShortOf && `short of ${v.holdingShortOf}`,
    v.atLimit && (v.limitNode >= 0 ? 'at limit' : 'holding position')].filter(Boolean).join(' · ');
}
const routeText = (v) => v.kind === 'arrival' ? `RWY ${v.runway} → ${v.stand}` : `${v.stand} → RWY ${v.runway}`;
// The approach sequence entry of an aircraft: its runway, place, actions.
function seqEntry(tail) {
  for (const r of apSeqs || []) {
    const i = r.sequence.findIndex((e) => e.callsign === tail);
    if (i >= 0) return { r, i, e: r.sequence[i] };
  }
  return null;
}
function seqActs(v, s) {
  if (!s) return [];
  if (s.e.fixed) return ['goaround'];
  return [...(s.i > 0 ? ['up'] : []), ...(s.i < s.r.sequence.length - 1 ? ['down'] : []), 'direct', 'slow', v.hold ? 'release' : 'hold', 'goaround'];
}

async function ctlAct(id, action, node, facing) {
  const v = ctlViews.find((x) => x.id === id);
  if (action === 'remove' && !confirm(`Remove ${v ? v.tail : 'this aircraft'} from the simulator?`)) return;
  if (ctlBusy.has(id)) return;
  ctlBusy.add(id);
  renderTraffic();
  const qs = new URLSearchParams();
  if (node !== undefined) qs.set('node', node);
  if (facing) qs.set('facing', facing);
  if (action === 'pushback' && !ctlPushOnly.has(id)) qs.set('startup', '1');
  const q = qs.toString() ? `?${qs}` : '';
  const r = await send(`/api/control/${id}/${action}${q}`);
  if (!r.ok) toast(`${v ? v.tail + ': ' : ''}${r.error}`, 'err');
  else if (action === 'remove' && ctlSelected === id) select(0);
  ctlBusy.delete(id);
  controlPoll.now();
}
async function approachAct(tail, action) {
  if (!data) { $$('[data-ap][disabled]').forEach((b) => { b.disabled = false; }); return; }
  // Its own airport's sequence, not the one on the map.
  const own = ctlViews.find((v) => v.tail === tail);
  const icao = (own && own.icao) || data.icao;
  const r = await send(`/api/approach/${icao}/${encodeURIComponent(tail)}/${action}`);
  const msg = r.ok ? `${tail}: ${AP_ACT[action].title.toLowerCase()}` : `${tail}: ${r.error}`;
  $('apMsg').textContent = msg;
  toast(msg, r.ok ? '' : 'err');
  $$('[data-ap][disabled]').forEach((b) => { b.disabled = false; });
  approachPoll.now();
}

function select(id) {
  const changed = id !== ctlSelected;
  ctlSelected = id;
  if (!id) ctlFollow = 0;
  renderTraffic();
  drawRoutes();
  refreshSelection();
  // Camera on "Follow selected": it follows the newly selected aircraft.
  if (changed && id && camView && camView.mode === 'follow') camPost('/api/camera', { mode: 'follow', id });
  // On a fixed view: the same view of the newly selected aircraft.
  if (changed && id && camView && camView.mode === 'view' && camView.shot) camPost('/api/camera', { mode: 'view', view: camView.shot, id });
  if (id && isPhone() && $('panel').dataset.sheet !== 'peek') setSheet('peek');
}

/* ───────────── Strips ───────────── */
function renderTraffic() {
  renderStrips();
  renderCtx();
}
function stripHTML(v) {
  const w = waits(v), done = isDone(v), busy = ctlBusy.has(v.id) || !onMyFrequency(v);
  const next = nextAction(v);
  let act = '';
  if (!done) {
    if (next && w) act = `<button type="button" class="btn btn--sm btn--primary strip-card__act" data-act="${next}" data-id="${v.id}"${busy ? ' disabled' : ''}>${esc(actLabel(v, next, true))}</button>`;
    else if (has(v, 'hold')) act = `<button type="button" class="btn btn--sm btn--urgent strip-card__act" data-act="hold" data-id="${v.id}"${busy ? ' disabled' : ''}>${icon('i-hand', 'ic ic--sm')}Hold</button>`;
    else if (next) act = `<button type="button" class="btn btn--sm strip-card__act" data-act="${next}" data-id="${v.id}"${busy ? ' disabled' : ''}>${esc(actLabel(v, next, true))}</button>`;
  }
  const kind = v.kind === 'arrival' ? 'arr' : 'dep';
  const freq = v.atc ? (v.frequency ? `<button type="button" class="freq-btn${v.frequency === rdFreq ? ' is-on' : ''}" data-tune="${esc(v.frequency)}" title="Listen on ${esc(v.atc)} ${esc(v.frequency)}">${icon('i-radio', 'ic ic--xs')}${esc(v.atc)} ${esc(v.frequency)}</button>` : `<span>${esc(v.atc)}</span>`) : '';
  const other = !onMyFrequency(v);
  return `<article class="strip-card${done ? ' is-done' : ''}${busy ? ' is-busy' : ''}${other ? ' is-other' : ''}" data-kind="${kind}" data-sel="${v.id}" tabindex="0" aria-current="${v.id === ctlSelected}" aria-label="${esc(v.tail)}, ${esc(v.state)}${other ? ', not on your frequency' : ''}" title="${esc(v.model)} · lights ${esc(v.lights || '—')}${other ? ' · not on your frequency: another position clears it' : ''}">
    <div class="strip-card__l1"><span class="strip-card__cs">${esc(v.tail)}</span>${rulesTag(v)}${v.deicing ? icon('i-snow', 'ic ic--xs') : ''}${timeChip(v, w)}</div>
    ${act}
    <div class="strip-card__l2"><span class="strip-card__state${w ? ' is-wait' : ''}">${esc(statusText(v))}</span></div>
    <div class="strip-card__l3"><span>${kind.toUpperCase()} ${esc(routeText(v).replace('RWY ', ''))}</span>${v.procedure ? `<span>${esc(v.procedure)}</span>` : ''}${freq}<span data-gs="${v.id}"></span></div>
    ${v.error ? `<div class="strip-card__err">${icon('i-warn', 'ic ic--xs')}${esc(v.error)}</div>` : ''}
  </article>`;
}
// timeChip: a scheduled flight shows its STD (departure) or STA (arrival),
// coloured by how late it is (fillSched); any other aircraft waiting for a
// clearance, how long it has waited.
const timeChip = (v, w) => schedFlight(v.tail) ? `<span class="sched-t" data-sched="${v.id}"></span>` : w ? `<span class="wait-t" data-wait="${v.id}"></span>` : '';
const LATE_MIN = 5, VERY_LATE_MIN = 15; // the acceptable window, then late, then very late
const actualAt = new Map(); // control ID → traffic time it left the stand (departure) or landed (arrival)
function fillSched(el, v) {
  const f = schedFlight(v.tail);
  const dep = v.kind !== 'arrival';
  const t = f ? Date.parse(dep ? f.std : f.sta) : 0;
  if (!t) { el.textContent = ''; return; }
  const now = Date.now() + schedSkew;
  // Off the stand once it pushes or taxis; an arrival, once on the ground.
  const off = dep ? !/awaiting pushback|spawning/.test(v.state) : v.onGround || isDone(v);
  if (off && !actualAt.has(v.id)) actualAt.set(v.id, now);
  const est = f.estimated && !f.estimated.startsWith('0001') ? Date.parse(f.estimated) : 0;
  const at = actualAt.get(v.id) || Math.max(now, est);
  const late = Math.round((at - t) / 60000);
  el.textContent = `${dep ? 'STD' : 'STA'} ${hhmm(t)}${late > LATE_MIN ? ` +${late}` : ''}`;
  el.className = `sched-t${late > VERY_LATE_MIN ? ' is-vlate' : late > LATE_MIN ? ' is-late' : ''}`;
  el.title = `${dep ? 'Scheduled off the stand' : 'Scheduled to land'} ${hhmm(t)}${late > 0 ? `, ${late} min late` : ', on time'}${!actualAt.has(v.id) && est ? ` (expected ${hhmm(est)})` : ''}`;
}
// rulesTag: the flight rules, IFR or VFR, by the call sign.
const rulesTag = (v) => v.rules ? `<span class="rules rules--${v.rules === 'VFR' ? 'v' : 'i'}" title="${v.rules === 'VFR' ? 'Visual flight rules' : 'Instrument flight rules'}">${esc(v.rules)}</span>` : '';
const waitSecs = (id) => waitSince.has(id) ? (Date.now() - waitSince.get(id)) / 1000 : 0;
function renderStrips() {
  const el = $('strips');
  if (interacting(el)) { fillLive(); return; }
  const q = $('stripFilter').value.trim().toUpperCase();
  const list = ctlViews.filter((v) => !q || `${v.tail} ${v.stand} ${v.runway} ${v.model} ${v.procedure || ''}`.toUpperCase().includes(q));
  const waiting = list.filter((v) => !isDone(v) && waits(v)).sort((a, b) => (waitSince.get(a.id) || 0) - (waitSince.get(b.id) || 0));
  const moving = list.filter((v) => !isDone(v) && !waits(v));
  const done = list.filter(isDone);
  const grp = (title, arr, cls = '') => arr.length ? `<h4 class="group-h ${cls}">${title} <span class="badge">${arr.length}</span></h4>${arr.map(stripHTML).join('')}` : '';
  setHTML(el, grp('Waiting for you', waiting, 'group-h--warn') + grp('Moving', moving) + grp('Airborne and done', done) ||
    (ctlViews.length ? '<p class="muted small">No aircraft match the filter.</p>' : `<p class="empty">${simLive ? 'No aircraft yet: <b>New flight</b>, or start the schedule or the game.' : 'No aircraft. The simulator is not connected: the map shows the airport, but nothing can be spawned or cleared.'}</p>`));
  const nWait = ctlViews.filter((v) => !isDone(v) && waits(v)).length;
  const b = $('tab-traffic').querySelector('.badge');
  b.textContent = nWait;
  b.hidden = !nWait;
  updateSheetSummary();
  fillLive();
}
// The wait timers tick in place, once a second.
function tickWaits() {
  for (const el of $$('[data-wait]')) el.textContent = mmss(waitSecs(Number(el.dataset.wait)));
  for (const el of $$('[data-sched]')) { const v = ctlViews.find((x) => x.id === Number(el.dataset.sched)); if (v) fillSched(el, v); }
}
// fillLive writes the values that change every second (speed, motion,
// wait) in place, so the strips and the panel re-render only on a change.
function fillLive() {
  const byId = (id) => ctlViews.find((x) => x.id === Number(id));
  for (const el of $$('[data-gs]')) { const v = byId(el.dataset.gs); if (v) el.textContent = `${v.groundSpeed.toFixed(0)} kt`; }
  for (const el of $$('[data-motion]')) { const v = byId(el.dataset.motion); if (v) el.textContent = `${v.groundSpeed.toFixed(0)} kt · ${Math.round(v.heading || 0)}°${v.state === 'spawning' ? ' · appearing' : (v.kind === 'departure' ? v.state !== 'departing' || v.onGround : v.onGround) ? ' · on ground' : ' · airborne'}`; }
  tickWaits();
}

// onMyFrequency: working one position (As), only aircraft on its frequency
// take clearances from here (approach also works departures); not yet
// handed to anyone, any. The server refuses the others anyway (#511).
function onMyFrequency(v) {
  if (!atcPosition || atcPosition === 'all' || !v.atc) return true;
  return v.atc === atcPosition || atcPosition === 'approach' && v.atc === 'departure';
}

/* ───────────── The context panel ───────────── */
function actBtn(v, a, busy, extra = '') {
  const A = ACT[a] || { label: a };
  const on = has(v, a) && !busy;
  return `<button type="button" class="btn ${A.urgent ? 'btn--urgent' : ''} ${extra}" data-act="${esc(a)}" data-id="${v.id}"${on ? '' : ' disabled'}>${A.icon ? icon(A.icon, 'ic ic--sm') : ''}${esc(actLabel(v, a))}</button>`;
}
// Runway entries for a departure's Entry choice, by "ICAO runway model"
// (GET /api/entries, with whether the type can take off from each).
const entriesCache = new Map();
const ENTRY_STATES = ['spawning', 'awaiting pushback', 'pushback', 'awaiting taxi', 'taxiing', 'holding short'];
function entryRow(v, other) {
  if (v.kind !== 'departure' || v.done || !ENTRY_STATES.includes(v.state) || !data) return '';
  // Its own airport, not the one on the map (live: EDKG asked for LKPR's 24).
  const icao = v.icao || data.icao;
  const key = `${icao} ${v.runway} ${v.model}`;
  const list = entriesCache.get(key);
  if (list === undefined) {
    entriesCache.set(key, null);
    api(`/api/entries?icao=${encodeURIComponent(icao)}&runway=${encodeURIComponent(v.runway)}&model=${encodeURIComponent(v.model)}`).then((r) => {
      entriesCache.set(key, r.ok ? r.data || [] : []);
      if (!r.ok) setTimeout(() => entriesCache.delete(key), 5000); // asked again, not cached as empty
      renderCtx();
    });
  }
  const opts = [['', 'Full length']].concat((list || []).slice(1).filter((e) => e.taxiway).map((e) => [e.taxiway, `${e.taxiway} · ${Math.round(e.remaining)} m${e.ok === false ? ' (too short)' : ''}`]));
  return `<dt>Entry</dt><dd><select class="input input--inline" data-entry="${v.id}" title="The intersection it takes the runway from; changed on the stand or while taxiing"${other ? ' disabled' : ''}>${opts.map(([k, t]) => `<option value="${esc(k)}"${(v.entry || '') === k ? ' selected' : ''}>${esc(t)}</option>`).join('')}</select></dd>`;
}
document.addEventListener('change', (e) => {
  const s = e.target.closest('select[data-entry]');
  if (!s) return;
  const v = ctlViews.find((x) => x.id === Number(s.dataset.entry));
  if (!v) return;
  s.disabled = true;
  send(`/api/control/${v.id}/entry?entry=${encodeURIComponent(s.value)}`).then((r) => {
    toast(r.ok ? `${v.tail}: runway ${v.runway}${s.value ? ' at ' + s.value : ', full length'}` : `${v.tail}: ${r.error}`, r.ok ? '' : 'err');
    s.disabled = false;
    s.blur(); // the card renders again (it waits while a select has the focus)
    controlPoll.now();
  });
});

// ctlPushOnly: the aircraft whose pushback leaves out the start-up (by
// default the two are approved in one).
const ctlPushOnly = new Set();
function facingRow(v, busy) {
  return `<label class="switch switch--sm" title="Pushback and start-up approved in one"><input type="checkbox" data-withstart="${v.id}"${ctlPushOnly.has(v.id) ? '' : ' checked'}${busy ? ' disabled' : ''}><span class="switch__ui" aria-hidden="true"></span>with start-up</label><div class="facing"><span>facing</span>${['n', 'e', 's', 'w'].map((d) => `<button type="button" class="btn" data-act="pushback" data-facing="${d}" data-id="${v.id}" title="Pushback, ending facing ${FACING[d]} where it can" aria-label="Pushback facing ${FACING[d]}"${busy ? ' disabled' : ''}>${d.toUpperCase()}</button>`).join('')}</div>`;
}
function renderCtx() {
  const el = $('ctx');
  const v = selectedView();
  if (!v) { el.hidden = true; setHTML(el, ''); return; }
  if (interacting(el)) { fillLive(); return; }
  el.hidden = false;
  el.dataset.kind = v.kind === 'arrival' ? 'arr' : 'dep';
  const other = !onMyFrequency(v), w = waits(v), busy = ctlBusy.has(v.id) || other, done = isDone(v);
  // Working one position, an aircraft on another frequency is shown but not
  // cleared from here: every control that changes it is off (#511).
  const off = other ? ` disabled title="Not on your frequency: ${esc(v.atc || 'another position')} works it"` : '';
  let h = `<header class="ctx__head">
    <div class="ctx__cs">${esc(v.tail)}<span class="ctx__kind">${v.kind === 'arrival' ? 'ARR' : 'DEP'}</span>${rulesTag(v)}${ctlBusy.has(v.id) ? '<span class="pill pill--accent small">sending…</span>' : other ? `<span class="pill small" title="Another position clears it">${esc(v.atc)}</span>` : ''}</div>
    <button type="button" class="btn btn--icon btn--ghost ctx__close" data-close-ctx aria-label="Deselect (Esc)" title="Deselect (Esc)">${icon('i-x')}</button>
    <div class="ctx__sub">${esc(v.model)}</div>
    <div class="ctx__state${w ? ' is-wait' : ''}">${timeChip(v, w)}${v.deicing ? icon('i-snow', 'ic ic--sm') : ''}${esc(statusText(v))}</div>
  </header>`;
  if (v.error) h += `<section class="ctx__sec ctx__err">${icon('i-warn', 'ic ic--sm')}${esc(v.error)}</section>`;
  if (!done) {
    const next = nextAction(v);
    let facingShown = false;
    if (next) {
      facingShown = next === 'pushback';
      h += `<section class="ctx__sec ctx__sec--next"><div class="ctx__lbl">Next</div><div class="next">
        <button type="button" class="btn btn--primary next__main" data-act="${next}" data-id="${v.id}"${busy ? ' disabled' : ''}>${esc(actLabel(v, next))}${next === 'pushback' ? ' <span class="next__sub">auto facing</span>' : ''}</button>
        ${facingShown ? facingRow(v, busy) : ''}</div></section>`;
    }
    // Clearances by phase: phases with something to give, the rest greyed in place.
    const groups = PHASES.filter((p) => p.acts.some((a) => has(v, a))).map((p) => {
      let g = `<div class="phase"><div class="phase__name">${p.name}</div><div class="acts">${p.acts.map((a) => actBtn(v, a, busy)).join('')}</div>`;
      if (p.key === 'ground' && has(v, 'pushback') && !facingShown) g += facingRow(v, busy);
      return g + '</div>';
    });
    const s = v.kind === 'arrival' && !v.onGround ? seqEntry(v.tail) : null;
    const ap = seqActs(v, s).filter((a) => a !== 'goaround');
    if (has(v, 'land') || ap.length) {
      groups.push(`<div class="phase"><div class="phase__name">Approach${s ? ` · #${s.e.number} RWY ${esc(s.r.runway)}, ${s.e.distanceToGoNM.toFixed(1)} NM` : ''}</div><div class="acts">${has(v, 'land') ? actBtn(v, 'land', busy) : ''}${ap.map((a) => `<button type="button" class="btn" data-ap="${a}" data-cs="${esc(v.tail)}" title="${AP_ACT[a].title}"${other ? ' disabled' : ''}>${icon(AP_ACT[a].icon, 'ic ic--sm')}${AP_ACT[a].label}</button>`).join('')}</div></div>`);
    }
    const known = new Set([...PHASES.flatMap((p) => p.acts), ...URGENT, 'land', 'upto']);
    const other = (v.actions || []).filter((a) => !known.has(a));
    if (other.length) groups.push(`<div class="phase"><div class="phase__name">Other</div><div class="acts">${other.map((a) => actBtn(v, a, busy)).join('')}</div></div>`);
    if (groups.length) h += `<section class="ctx__sec"><div class="ctx__lbl">Clearances</div>${groups.join('')}</section>`;
    // Urgent: always in the same place, enabled when they apply.
    const apGo = !has(v, 'goaround') && s && !s.e.fixed;
    h += `<section class="ctx__sec ctx__sec--urgent"><div class="ctx__lbl">Urgent</div><div class="urgent-row">${actBtn(v, 'hold', busy)}${apGo
      ? `<button type="button" class="btn btn--urgent" data-ap="goaround" data-cs="${esc(v.tail)}"${other ? ' disabled' : ''}>${icon('i-goaround', 'ic ic--sm')}Go around</button>` : actBtn(v, 'goaround', busy)}${actBtn(v, 'abort', busy)}</div>
      ${has(v, 'upto') ? '<p class="small muted ctx__hint">Click a point of its route on the map: taxi and hold there.</p>' : ''}</section>`;
  }
  const freq = v.atc ? (v.frequency ? `<button type="button" class="freq-btn${v.frequency === rdFreq ? ' is-on' : ''}" data-tune="${esc(v.frequency)}" title="Listen on ${esc(v.frequency)}">${icon('i-radio', 'ic ic--xs')}${esc(v.atc)} ${esc(v.frequency)}</button>` : esc(v.atc)) : '';
  h += `<section class="ctx__sec"><dl class="kv">
    <dt>Route</dt><dd class="mono">${esc(routeText(v))}</dd>${v.squawk ? `
    <dt>Squawk</dt><dd class="mono">${esc(v.squawk)}</dd>` : ''}
    ${entryRow(v, other)}
    ${v.procedure ? `<dt>Procedure</dt><dd class="mono">${esc(v.procedure)}</dd>` : ''}
    ${freq ? `<dt>Frequency</dt><dd>${freq}</dd>` : ''}
    <dt>Motion</dt><dd class="mono" data-motion="${v.id}"></dd>
    <dt>Lights</dt><dd>${esc(v.lights || '—')}</dd>
  </dl></section>`;
  const mine = (radioCache || []).filter((t) => t.callsign === v.tail).slice(-4).reverse();
  h += `<section class="ctx__sec"><div class="ctx__lbl">Radio <button type="button" class="btn btn--sm btn--ghost" data-goto="radio">Open console</button></div>
    <div class="mini-log">${mine.length ? mine.map(txHTML).join('') : '<p class="muted small">Nothing said yet.</p>'}</div></section>`;
  const camOn = camView && camView.mode === 'follow' && camView.subject === v.tail;
  const located = v.position && (v.position.lat || v.position.lon);
  h += `<section class="ctx__sec"><div class="tools-row">
    <button type="button" class="btn btn--sm" data-tool="locate" data-id="${v.id}"${located ? '' : ' disabled'}>${icon('i-locate', 'ic ic--sm')}Show</button>
    <button type="button" class="btn btn--sm btn--toggle" data-tool="follow" data-id="${v.id}" aria-pressed="${ctlFollow === v.id}" title="The map keeps it in the middle (Esc stops)"${located ? '' : ' disabled'}>${icon('i-target', 'ic ic--sm')}Follow</button>
    <button type="button" class="btn btn--sm btn--toggle" data-tool="camera" data-id="${v.id}" aria-pressed="${camOn}" title="The simulator camera follows it">${icon('i-camera', 'ic ic--sm')}Camera</button>
    <button type="button" class="btn btn--sm btn--toggle" data-tool="manual" data-id="${v.id}" aria-pressed="${!!v.manual}" title="Manual: you give every clearance. Off: ATC answers the crew and the tower clears it by itself. Your first clearance turns it on."${v.done ? ' disabled' : off}>${icon('i-hand', 'ic ic--sm')}Manual</button>
    <button type="button" class="btn btn--sm btn--toggle" data-tool="rush" data-id="${v.id}" aria-pressed="${!!v.rush}" title="Expedite: immediate take-off, expedite crossing and vacating; the crew hurries"${busy || v.done ? ' disabled' : ''}>${icon('i-bolt', 'ic ic--sm')}Rush</button>
    <button type="button" class="btn btn--sm btn--danger" data-tool="remove" data-id="${v.id}"${busy ? ' disabled' : ''}>${icon('i-trash', 'ic ic--sm')}Remove</button>
  </div></section>`;
  setHTML(el, h);
  fillLive();
}

/* ───────────── New flight ───────────── */
let nfKind = 'dep';
let routeFrom = null;   // the parking spot picked
let choices = [];       // entries (departure) or exits (arrival) of the runway
let viaPoints = [];     // {id, position, taxiways}
let activeUse = null;   // {departure, arrival}: the runway in use (airportinfo)
const arrival = () => nfKind === 'arr';
// New flight is a window over the blurred map (full screen on a tablet or
// phone). To pick on the map, a stand or via points, it steps aside for a
// bar on the map and comes back when done.
let nfPicking = '';     // '' (the window), 'stand' or 'via': picking on the map
const nfOpen = () => !$('nfModal').hidden || !!nfPicking;

function openNewFlight(open) {
  nfPicking = '';
  $('nfPick').hidden = true;
  $('nfModal').hidden = !open;
  document.body.classList.toggle('nf-open', open);
  $('newFlightBtn').setAttribute('aria-expanded', String(open));
  for (const g of ['choices', 'preview', 'via']) { if (open) layers[g].addTo(map); else layers[g].remove(); }
  if (open) {
    loadModels();
    const k = $('newFlight').querySelector('[data-kind][aria-checked="true"]');
    if (k) k.focus({ preventScroll: true });
  } else {
    $('rViaPick').checked = false;
  }
}
// nfStepAside hides the window for picking on the map: 'stand' comes back
// with the stand clicked, 'via' when the bar's Done is pressed.
function nfStepAside(what) {
  nfPicking = what;
  $('nfModal').hidden = true;
  document.body.classList.remove('nf-open');
  $('nfPickMsg').textContent = what === 'via'
    ? 'Click taxiways to add via points; click a via point to remove it'
    : 'Click a stand on the map';
  $('nfPickBack').textContent = what === 'via' ? 'Done' : 'Back to New flight';
  $('nfPick').hidden = false;
}
function nfBack() {
  if (nfPicking === 'via') $('rViaPick').checked = false;
  nfPicking = '';
  $('nfPick').hidden = true;
  $('nfModal').hidden = false;
  document.body.classList.add('nf-open');
}
function setKind(k) {
  nfKind = k;
  $$('#newFlight [data-kind]').forEach((b) => b.setAttribute('aria-checked', String(b.dataset.kind === k)));
  const opt = $('rTo').querySelector('option[value="active"]');
  if (opt) opt.textContent = activeLabel();
  loadChoices();
}

// The runway: "Active" by default — the runway in use from the weather
// (departures and arrivals may differ), followed when it changes — or a
// particular runway end to override it.
function fillRunwayEnds() {
  const sel = $('rTo'), keep = sel.value || 'active', ends = runwayEnds();
  sel.innerHTML = `<option value="active">${esc(activeLabel())}</option>` + ends.map((e) => `<option>${esc(e)}</option>`).join('');
  sel.value = keep === 'active' || ends.includes(keep) ? keep : 'active';
}
function activeLabel() {
  const r = activeUse && (arrival() ? activeUse.arrival : activeUse.departure);
  return r ? `Active (${r})` : 'Active (runway in use)';
}
function selectedRunway() {
  const v = $('rTo').value;
  if (v && v !== 'active') return v;
  const r = activeUse && (arrival() ? activeUse.arrival : activeUse.departure);
  if (r) return r;
  const first = $('rTo').options[1];
  return first ? first.value : '';
}
function setActiveUse(use) {
  const before = selectedRunway();
  activeUse = use;
  const opt = $('rTo').querySelector('option[value="active"]');
  if (opt) opt.textContent = activeLabel();
  if ($('rTo').value === 'active' && selectedRunway() !== before) loadChoices();
}

function pickStand(p) {
  routeFrom = p;
  $('rFrom').textContent = `${parkingLabel(p)}`;
  $('rFromHint').textContent = `#${p.index}`;
  markPicked();
  if (nfPicking === 'stand') nfBack();
  else if (!nfOpen()) openNewFlight(true);
  computeRoute();
}
// Hide the planned route: no stand picked, nothing drawn (button or Esc).
function hideRoute() {
  routeFrom = null;
  $('rFrom').textContent = '—';
  $('rFromHint').textContent = 'click a stand on the map';
  markPicked();
  computeRoute();
}

// loadChoices fetches the entries or exits of the runway end, fills the
// picker and marks them on the map (click a marker to pick it).
let choicesSeq = 0;
// The pick (an entry, or an exit) stays across a reload for the same
// runway and kind: a new aircraft type reloads the entries (which are long
// enough depends on it), and must not drop "06 at E" back to full length.
let choicesFor = '';
async function loadChoices() {
  const rwy = selectedRunway(), arr = arrival(), seq = ++choicesSeq;
  const forKey = `${arr ? 'arr' : 'dep'} ${rwy}`;
  const keep = forKey === choicesFor ? $('rPick').value : '';
  choicesFor = forKey;
  $('rStandLbl').textContent = arr ? 'To stand' : 'From stand';
  $('rPickLbl').textContent = arr ? 'Exit' : 'Entry';
  choices = [];
  $('rPick').innerHTML = `<option value="">${arr ? 'Auto (best exit)' : 'Full length'}</option>`;
  if (!data || !rwy) { computeRoute(); return; }
  const q = { icao: data.icao, runway: rwy };
  if (!arr) q.model = $('cModel').value; // which entries leave enough runway for it
  const r = await api(`/api/${arr ? 'exits' : 'entries'}?${new URLSearchParams(q)}`);
  if (seq !== choicesSeq) return;
  if (!r.ok) $('rInfo').innerHTML = `<span class="err-text">${esc(r.error)}</span>`;
  choices = (r.ok && r.data) || [];
  const opts = [];
  if (arr) {
    choices.forEach((x, i) => opts.push(`<option value="${i}">${esc(x.taxiway || 'unnamed')} · ${x.along.toFixed(0)} m · ${x.angle.toFixed(0)}°${x.highSpeed ? ' rapid' : ''} ${x.side === 0 ? 'L' : 'R'}</option>`));
  } else {
    // The API routes to the entry of that name with the most runway ahead;
    // entries too short for the model are shown but cannot be picked.
    const best = new Map();
    for (const e of choices) if (e.taxiway && (!best.has(e.taxiway) || e.remaining > best.get(e.taxiway).remaining)) best.set(e.taxiway, e);
    for (const e of best.values()) {
      const short = e.required && !e.ok;
      opts.push(`<option value="${esc(e.taxiway)}"${short ? ' disabled' : ''}>${esc(rwy)} at ${esc(e.taxiway)} · ${e.remaining.toFixed(0)} m ahead · ${e.angle.toFixed(0)}°${short ? ` · too short (needs ${e.required.toFixed(0)} m)` : ''}</option>`);
    }
  }
  $('rPick').innerHTML += opts.join('');
  if (keep) {
    const o = [...$('rPick').options].find((x) => x.value === keep);
    if (o && !o.disabled) $('rPick').value = keep;
    else if (!arr) toast(o ? `${rwy} at ${keep} is too short for this aircraft: full length` : `${rwy} has no entry ${keep} now: full length`, 'err');
  }
  computeRoute();
}
function drawChoices() {
  layers.choices.clearLayers();
  const arr = arrival(), picked = $('rPick').value;
  choices.forEach((x, i) => {
    const key = arr ? String(i) : x.taxiway;
    if (!arr && !x.taxiway) return;
    const on = picked !== '' && key === picked;
    L.circleMarker([x.position.lat, x.position.lon], { radius: on ? 8 : 6, className: `m-choice m-choice--${arr ? 'exit' : 'entry'}${on ? ' is-on' : ''}`, bubblingMouseEvents: false })
      .bindTooltip(`${arr ? 'Exit' : 'Entry'} ${esc(x.taxiway || 'unnamed')}`, { permanent: true, direction: 'top', className: 'map-lbl' })
      .on('click', () => { $('rPick').value = key; computeRoute(); })
      .addTo(layers.choices);
  });
}

// Custom route (#340): via points picked on the map, named taxiways.
function customRoute() {
  const taxiways = $('rTaxiways').value.split(/[\s,]+/).map((t) => t.trim().toUpperCase()).filter(Boolean);
  return { via: viaPoints.map((v) => v.id), taxiways };
}
function drawVia() {
  layers.via.clearLayers();
  viaPoints.forEach((v, i) => {
    L.marker([v.position.lat, v.position.lon], { icon: L.divIcon({ className: 'via-m', iconSize: [22, 22], iconAnchor: [11, 11], html: `<span>${i + 1}</span>` }) })
      .bindTooltip(`Via ${i + 1}: ${esc(v.taxiways.join('/') || 'node ' + v.id)} — click to remove`, { direction: 'top', className: 'node-tip' })
      .on('click', () => { viaPoints.splice(i, 1); drawVia(); computeRoute(); })
      .addTo(layers.via);
  });
  $('rVia').textContent = viaPoints.length ? 'Via ' + viaPoints.map((v, i) => `${i + 1}. ${v.taxiways.join('/') || '#' + v.id}`).join(' → ') : '';
}

let routeSeq = 0;
async function computeRoute() {
  updateSpawn();
  layers.preview.clearLayers();
  drawChoices();
  const seq = ++routeSeq;
  if (!data || !routeFrom) { $('rInfo').textContent = ''; return; }
  const rwy = selectedRunway(), pick = $('rPick').value, arr = arrival();
  const stand = parkingLabel(routeFrom);
  $('rInfo').textContent = arr ? `Routing ${rwy} → ${stand}…` : `Routing ${stand} → ${rwy}…`;
  const q = new URLSearchParams({ icao: data.icao });
  if ($('rRwyPaths').checked) q.set('runwayPaths', '1');
  for (const [k, v] of Object.entries(customRoute())) if (v.length) q.set(k, v.join(','));
  if ($('cModel').value) q.set('model', $('cModel').value); // the route must fit the aircraft
  if (arr) {
    q.set('runway', rwy); q.set('to', routeFrom.index);
    if (pick !== '') q.set('exit', pick);
  } else {
    q.set('from', routeFrom.index); q.set('to', rwy);
    if (pick !== '') q.set('entry', pick);
  }
  const res = await api(`/api/${arr ? 'arrival' : 'route'}?${q}`);
  if (seq !== routeSeq) return;
  if (!res.ok) { $('rInfo').innerHTML = `<span class="err-text">${esc(res.error)}</span>`; return; }
  const body = res.data;
  const r = arr ? body.route : body;
  const pts = r.points.map((p) => [p.lat, p.lon]);
  L.polyline(pts, { className: 'm-route-casing', interactive: false }).addTo(layers.preview);
  L.polyline(pts, { className: 'm-route m-route--preview', interactive: false }).addTo(layers.preview);
  L.circleMarker(pts[0], { radius: 7, className: 'm-start', interactive: false }).addTo(layers.preview);
  const via = `Via ${r.taxiways.length ? esc(r.taxiways.join(' → ')) : '—'} · crossings ${r.runwayCrossings.length ? esc(r.runwayCrossings.join(', ')) : 'none'}`;
  if (arr) {
    const x = body.exit;
    L.circleMarker([body.vacate.lat, body.vacate.lon], { radius: 8, className: 'm-vacate', interactive: false })
      .bindTooltip('Vacate stop', { permanent: true, direction: 'right', className: 'map-lbl' }).addTo(layers.preview);
    L.circleMarker([body.stop.lat, body.stop.lon], { radius: 9, className: 'm-stop', interactive: false })
      .bindTooltip(`Stop ${esc(stand)}`, { permanent: true, direction: 'right', className: 'map-lbl' }).addTo(layers.preview);
    $('rInfo').innerHTML = `<b>${esc(rwy)} exit ${esc(x.taxiway || 'unnamed')} → ${esc(stand)}</b> · ${r.length.toFixed(0)} m<br>` +
      `Exit ${x.along.toFixed(0)} m from the threshold, ${x.angle.toFixed(0)}°${x.highSpeed ? ' (rapid)' : ''}${pick === '' ? ' — chosen automatically' : ''}<br>${via}`;
  } else {
    L.circleMarker(pts[pts.length - 1], { radius: 9, className: 'm-stop', interactive: false })
      .bindTooltip(`Hold short ${esc(r.runwayEnd)}${r.entry ? ' at ' + esc(r.entry) : ''}`, { permanent: true, direction: 'right', className: 'map-lbl' }).addTo(layers.preview);
    const h = r.holdShort;
    $('rInfo').innerHTML = `<b>${esc(stand)} → ${esc(r.runwayEnd)}${r.entry ? ' at ' + esc(r.entry) : ' full length'}</b> · ${r.length.toFixed(0)} m<br>${via}` +
      (h ? `<br>Hold short ${esc(h.name)}${h.ils ? ' (ILS)' : ''}, ${h.offset.toFixed(0)} m from the centreline, taxi point #${r.nodes[r.nodes.length - 1]}` : '');
  }
}

// updateSpawn labels the spawn button with what it will do, and shows only
// the options of the kind.
function updateSpawn() {
  const form = $('newFlight'), arr = arrival();
  form.classList.toggle('arr', arr);
  form.classList.toggle('dep', !arr);
  for (const el of $$('.fpHere')) el.textContent = data ? data.icao : '';
  const auto = $('cAutoStand').checked, rwy = selectedRunway() || '?';
  const stand = auto ? 'a free stand' : routeFrom ? parkingLabel(routeFrom) : null;
  const entry = !arr && $('rPick').value ? ` at ${$('rPick').value}` : '';
  $('cGatesLbl').textContent = arr ? 'Hold for clearances (taxi after landing, runway crossings)' : 'Hold for clearances (pushback, taxi, crossings, line-up, take-off)';
  $('rTo').querySelector('option[value="active"]') && ($('rTo').querySelector('option[value="active"]').textContent = activeLabel());
  const b = $('cSpawn');
  b.disabled = !data || !stand;
  $('cSpawnLbl').textContent = !stand ? 'Pick a stand on the map (or a free one)' : arr ? `Spawn arrival: runway ${rwy} → ${stand}` : `Spawn departure: ${stand} → runway ${rwy}${entry}`;
  const opts = [$('cGates').checked && 'hold at clearances', !arr && $('cTug').checked && 'tug', arr && $('cInjectApproach').checked && 'approach', arr && $('cTurn').checked && 'turnaround', arr && $('cCircuit').checked && 'VFR circuit', $('cProc').checked && 'procedures', !arr && $('cDeice').value && 'de-icing'].filter(Boolean);
  $('optSummary').textContent = opts.join(' · ');
  const c = customRoute();
  $('customSummary').textContent = c.via.length || c.taxiways.length ? `${c.via.length} via · ${c.taxiways.join(', ') || 'no taxiways'}` : '';
}

async function ctlSpawn() {
  const kind = arrival() ? 'arrival' : 'departure';
  const auto = $('cAutoStand').checked;
  if (!data || (!routeFrom && !auto)) { $('cInfo').innerHTML = '<span class="err-text">Pick a stand first (click a parking spot), or tick Free stand.</span>'; return; }
  if (!needSim('Spawn')) { $('cInfo').innerHTML = '<span class="err-text">Spawning needs the simulator (not connected).</span>'; return; }
  const num = (id) => { const s = $(id).value.trim(); return s === '' || !isFinite(Number(s)) ? undefined : Number(s); };
  const body = {
    kind, icao: data.icao, stand: auto ? -1 : routeFrom.index, runway: selectedRunway(), model: $('cModel').value, tail: $('cTail').value.trim().toUpperCase(), squawk: $('cSquawk').value.trim(),
    gates: $('cGates').checked, injectApproach: $('cInjectApproach').checked, tug: $('cTug').checked,
    turnaround: kind === 'arrival' && $('cTurn').checked, dwellSec: Number($('cDwell').value) || 90,
    procedure: $('cProc').checked,
    circuit: kind === 'arrival' && $('cCircuit').checked,
    other: $('cOther').value.trim().toUpperCase(),
    ...customRoute(),
    deice: kind === 'departure' ? $('cDeice').value : '', deiceSec: Number($('cDeiceSec').value) || 0,
  };
  if (kind === 'departure' && $('cTug').checked) {
    const title = $('cTugTitle').value.trim(), yaw = num('cTugYaw'), ahead = num('cTugAhead');
    if (title) body.tugTitle = title;
    if (yaw !== undefined) body.tugYaw = yaw;
    if (ahead !== undefined) body.tugAhead = ahead;
  }
  if (kind === 'departure' && $('rPick').value) body.entry = $('rPick').value;
  if (kind === 'arrival' && $('rPick').value !== '') body.exit = Number($('rPick').value); // the exit picked
  $('cInfo').textContent = `Spawning ${kind}…`;
  $('cSpawn').disabled = true;
  const r = await send('/api/control', body);
  updateSpawn();
  if (!r.ok) { $('cInfo').innerHTML = `<span class="err-text">${esc(r.error)}</span>`; toast(r.error, 'err'); return; }
  $('cInfo').textContent = `${r.data.tail}: ${kind} spawned at ${r.data.stand}`;
  toast(`${r.data.tail}: ${kind} spawned at ${r.data.stand}`);
  openNewFlight(false); // back to the map: the new aircraft is selected there
  ctlSelected = r.data.id;
  controlPoll.now();
}

/* ───────────── Model picker ───────────── */
// A searchable dropdown of the simulator's aircraft titles: click the field
// or the arrow to open it, type to filter (all words must match), pick with
// a click or ↑/↓ and Enter.
let models = [];
let modelActive = -1;
let modelsLoading = false;
let modelsTimer = 0; // one retry pending at a time
async function loadModels() {
  if (models.length || modelsLoading) return;
  modelsLoading = true;
  const r = await api('/api/models');
  modelsLoading = false;
  models = (r.ok && r.data) || [];
  $('cModelCount').textContent = models.length ? `· ${models.length} models` : simLive ? '· loading the list…' : '· list needs the simulator';
  if (!models.length && nfOpen() && !modelsTimer) modelsTimer = setTimeout(() => { modelsTimer = 0; loadModels(); }, 5000);
}
function modelMatches(showAll) {
  const words = showAll ? [] : $('cModel').value.toLowerCase().split(/\s+/).filter(Boolean);
  return models.filter((m) => words.every((w) => m.toLowerCase().includes(w)));
}
function openModelMenu(showAll) {
  const menu = $('modelMenu');
  modelActive = -1;
  if (!models.length) {
    menu.innerHTML = `<div class="combo-note">${simLive ? 'Loading the aircraft list…' : 'The aircraft list comes from the simulator (not connected). Type a model title.'}</div>`;
  } else {
    const list = modelMatches(showAll);
    menu.innerHTML = list.length
      ? list.slice(0, 300).map((m) => `<div class="combo-item" role="option" data-model="${esc(m)}">${esc(m)}</div>`).join('') +
        (list.length > 300 ? `<div class="combo-note">${list.length - 300} more — type to narrow</div>` : '')
      : '<div class="combo-note">No aircraft match</div>';
  }
  menu.hidden = false;
  $('cModel').setAttribute('aria-expanded', 'true');
}
function closeModelMenu() { $('modelMenu').hidden = true; $('cModel').setAttribute('aria-expanded', 'false'); }
function pickModel(m) {
  $('cModel').value = m;
  closeModelMenu();
  if (!arrival()) loadChoices(); // the entries long enough depend on the type
  else computeRoute();
}

/* ───────────── Wiring ───────────── */
function initTraffic() {
  guardPress($('strips'), renderStrips);
  guardPress($('ctx'), renderCtx);
  $('newFlightBtn').addEventListener('click', () => openNewFlight(!nfOpen()));
  $('newFlight').addEventListener('submit', (e) => e.preventDefault());
  $('newFlight').addEventListener('click', (e) => {
    const k = e.target.closest('[data-kind]');
    if (k) setKind(k.dataset.kind);
    if (e.target.closest('[data-close-nf]')) openNewFlight(false);
  });
  $('rPickStand').addEventListener('click', () => nfStepAside('stand'));
  // Ctrl+Enter (Cmd+Enter) spawns from anywhere in the window.
  $('nfModal').addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey) && !$('cSpawn').disabled) { e.preventDefault(); $('cSpawn').click(); }
  });
  $('rViaPick').addEventListener('change', () => { if ($('rViaPick').checked) nfStepAside('via'); });
  $('nfPickBack').addEventListener('click', nfBack);
  // A click on the blurred backdrop closes the window (pressed and released
  // there: not a text selection that ends outside).
  let downOnBackdrop = false;
  $('nfModal').addEventListener('pointerdown', (e) => { downOnBackdrop = e.target === e.currentTarget; });
  $('nfModal').addEventListener('click', (e) => { if (downOnBackdrop && e.target === e.currentTarget) openNewFlight(false); });
  // Esc: the window closes (the stand stays); picking, back to the window.
  document.addEventListener('keydown', (e) => {
    if (e.key !== 'Escape' || !nfOpen()) return;
    if (!$('modelMenu').hidden) return; // the model menu closes first
    e.stopPropagation();
    if (nfPicking) nfBack(); else openNewFlight(false);
  }, true);
  $('rTo').addEventListener('change', loadChoices);
  $('rPick').addEventListener('change', computeRoute);
  $('rRwyPaths').addEventListener('change', computeRoute);
  $('rTaxiways').addEventListener('change', computeRoute);
  $('rCustomClear').addEventListener('click', () => { viaPoints = []; $('rTaxiways').value = ''; drawVia(); computeRoute(); });
  $('rHide').addEventListener('click', hideRoute);
  $('cSpawn').addEventListener('click', ctlSpawn);
  for (const id of ['cAutoStand', 'cGates', 'cTug', 'cInjectApproach', 'cTurn', 'cProc', 'cCircuit', 'cDeice']) $(id).addEventListener('change', updateSpawn);
  $('cTug').addEventListener('change', () => { $('tugBox').classList.toggle('is-off', !$('cTug').checked); });
  $('stripFilter').addEventListener('input', renderStrips);
  // Enter: to that aircraft. The call sign typed, else the first match still
  // flying or moving; selected and centred on the map.
  $('stripFilter').addEventListener('keydown', (e) => {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    const q = e.target.value.trim().toUpperCase();
    if (!q) return;
    const match = (v) => `${v.tail} ${v.stand} ${v.runway} ${v.model} ${v.procedure || ''}`.toUpperCase().includes(q);
    const v = ctlViews.find((x) => x.tail.toUpperCase() === q) || ctlViews.find((x) => !isDone(x) && match(x)) || ctlViews.find(match);
    if (!v) { toast(`No aircraft ${q}`, 'err'); return; }
    select(v.id);
    if (v.position) locate(v.position.lat, v.position.lon, v.tail);
  });

  $('cModel').addEventListener('focus', () => openModelMenu(true));
  $('cModel').addEventListener('click', () => openModelMenu(true));
  $('cModel').addEventListener('input', () => openModelMenu(false));
  $('cModel').addEventListener('change', () => { if (!arrival()) loadChoices(); });
  $('cModelOpen').addEventListener('click', () => {
    if ($('modelMenu').hidden) { openModelMenu(true); $('cModel').focus(); } else closeModelMenu();
  });
  $('cModel').addEventListener('keydown', (e) => {
    const items = [...$('modelMenu').querySelectorAll('.combo-item')];
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      if ($('modelMenu').hidden) openModelMenu(false);
      modelActive = Math.max(0, Math.min(items.length - 1, modelActive + (e.key === 'ArrowDown' ? 1 : -1)));
      items.forEach((el, i) => el.classList.toggle('active', i === modelActive));
      if (items[modelActive]) items[modelActive].scrollIntoView({ block: 'nearest' });
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (modelActive >= 0 && items[modelActive]) pickModel(items[modelActive].dataset.model);
      else closeModelMenu();
    } else if (e.key === 'Escape') {
      e.stopPropagation();
      closeModelMenu();
    }
  });
  $('modelMenu').addEventListener('mousedown', (e) => {
    const item = e.target.closest('.combo-item');
    if (item) { e.preventDefault(); pickModel(item.dataset.model); }
  });
  document.addEventListener('mousedown', (e) => { if (!e.target.closest('.combo')) closeModelMenu(); });

  // Clearances, tools and selection, wherever their buttons are.
  document.addEventListener('click', (e) => {
    const tune = e.target.closest('[data-tune]');
    if (tune) { e.stopPropagation(); tuneTo(tune.dataset.tune); return; }
    const ap = e.target.closest('[data-ap]');
    if (ap) { if (!ap.disabled) { ap.disabled = true; approachAct(ap.dataset.cs, ap.dataset.ap); } return; }
    const ws = e.target.closest('[data-withstart]');
    if (ws) { const id = Number(ws.dataset.withstart); if (ws.checked) ctlPushOnly.delete(id); else ctlPushOnly.add(id); return; }
    const a = e.target.closest('[data-act]');
    if (a) { if (!a.disabled) ctlAct(Number(a.dataset.id), a.dataset.act, undefined, a.dataset.facing); return; }
    const tool = e.target.closest('[data-tool]');
    if (tool) {
      if (tool.disabled) return;
      const v = ctlViews.find((x) => x.id === Number(tool.dataset.id));
      if (!v) return;
      if (tool.dataset.tool === 'locate') locate(v.position.lat, v.position.lon, v.tail);
      if (tool.dataset.tool === 'follow') {
        ctlFollow = ctlFollow === v.id ? 0 : v.id;
        if (ctlFollow) map.panTo([v.position.lat, v.position.lon]);
        toast(ctlFollow ? `Map follows ${v.tail} (Esc stops)` : 'Map follow off');
        renderCtx();
      }
      if (tool.dataset.tool === 'camera') {
        const on = camView && camView.mode === 'follow' && camView.subject === v.tail;
        camPost('/api/camera', on ? { mode: 'off' } : { mode: 'follow', id: v.id });
      }
      if (tool.dataset.tool === 'remove') ctlAct(v.id, 'remove');
      if (tool.dataset.tool === 'manual') {
        send(`/api/control/${v.id}/manual?on=${v.manual ? 0 : 1}`).then((r) => {
          toast(r.ok ? `${v.tail}: ${v.manual ? 'automatic again' : 'under your control'}` : `${v.tail}: ${r.error}`, r.ok ? '' : 'err');
          controlPoll.now();
        });
      }
      if (tool.dataset.tool === 'rush') {
        send(`/api/control/${v.id}/rush?on=${v.rush ? 0 : 1}`).then((r) => {
          toast(r.ok ? `${v.tail}: ${v.rush ? 'no more rush' : 'expedite'}` : `${v.tail}: ${r.error}`, r.ok ? '' : 'err');
          controlPoll.now();
        });
      }
      return;
    }
    if (e.target.closest('[data-close-ctx]')) { select(0); return; }
    const card = e.target.closest('[data-sel]');
    if (card) select(Number(card.dataset.sel) === ctlSelected ? 0 : Number(card.dataset.sel));
  });
  $('strips').addEventListener('keydown', (e) => {
    const card = e.target.closest('[data-sel]');
    if (card && (e.key === 'Enter' || e.key === ' ') && e.target === card) { e.preventDefault(); select(Number(card.dataset.sel) === ctlSelected ? 0 : Number(card.dataset.sel)); }
  });

  // A map click: a de-icing pad (Airport), or a via point (custom route).
  map.on('click', async (e) => {
    if (!data) return;
    if ($('dePick').checked) { addPadAt(e.latlng); return; }
    if (!$('rViaPick').checked || !nfOpen()) return;
    const r = await api(`/api/node?${new URLSearchParams({ icao: data.icao, lat: e.latlng.lat, lon: e.latlng.lng })}`);
    if (!r.ok) { $('rInfo').innerHTML = `<span class="err-text">${esc(r.error)}</span>`; return; }
    viaPoints.push(r.data);
    drawVia();
    computeRoute();
  });
}

// controlUpdated takes a new GET /api/control: wait timers, strips, the
// context panel, routes and the map follow.
function controlUpdated(list) {
  ctlViews = list.sort((a, b) => a.id - b.id);
  const now = Date.now();
  for (const v of ctlViews) {
    if (waits(v) && !isDone(v)) { if (!waitSince.has(v.id)) waitSince.set(v.id, now); } else waitSince.delete(v.id);
  }
  if (ctlSelected && !selectedView()) { ctlSelected = 0; ctlFollow = 0; }
  renderTraffic();
  drawRoutes();
  drawTugs();
  const f = ctlFollow && ctlViews.find((x) => x.id === ctlFollow);
  if (f && f.position && (f.position.lat || f.position.lon)) map.panTo([f.position.lat, f.position.lon], { animate: true });
}
