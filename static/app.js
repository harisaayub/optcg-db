// ── API base URL ─────────────────────────────────────────────────────────────
// Empty string → same-origin (local dev). Set to Railway URL for production.
const API_BASE = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'
  ? ''
  : 'https://optcg-db-production.up.railway.app';

// ── Tooltips ─────────────────────────────────────────────────────────────────

const tooltipEl = document.createElement('div');
tooltipEl.id = 'tooltip';
document.body.appendChild(tooltipEl);

document.querySelectorAll('[data-tooltip]').forEach(el => {
  el.addEventListener('mouseenter', () => {
    tooltipEl.textContent = el.dataset.tooltip;
    tooltipEl.classList.add('visible');

    const r  = el.getBoundingClientRect();
    const tw = tooltipEl.offsetWidth;
    const th = tooltipEl.offsetHeight;
    const pad = 8;

    // Default: below the element, left-aligned to it
    let top  = r.bottom + 6;
    let left = r.left;

    // Flip above if it would fall below the viewport
    if (top + th + pad > window.innerHeight) top = r.top - th - 6;

    // Clamp horizontally within viewport
    if (left + tw + pad > window.innerWidth) left = window.innerWidth - tw - pad;
    if (left < pad) left = pad;

    tooltipEl.style.left = left + 'px';
    tooltipEl.style.top  = top  + 'px';
  });
  el.addEventListener('mouseleave', () => tooltipEl.classList.remove('visible'));
});

// ── Grid zoom ────────────────────────────────────────────────────────────────

const ZOOM_STEPS = [150, 200, 260, 310, 400, 510, 650];
const ZOOM_DEFAULT_IDX = 3; // 310px

let zoomIdx = parseInt(localStorage.getItem('cardZoomIdx') ?? ZOOM_DEFAULT_IDX, 10);
if (zoomIdx < 0 || zoomIdx >= ZOOM_STEPS.length) zoomIdx = ZOOM_DEFAULT_IDX;

function applyZoom() {
  document.documentElement.style.setProperty('--card-min-width', ZOOM_STEPS[zoomIdx] + 'px');
  document.getElementById('zoom-out').disabled = zoomIdx === 0;
  document.getElementById('zoom-in').disabled  = zoomIdx === ZOOM_STEPS.length - 1;
  localStorage.setItem('cardZoomIdx', zoomIdx);
}

document.getElementById('zoom-out').addEventListener('click', () => {
  if (zoomIdx > 0) { zoomIdx--; applyZoom(); }
});
document.getElementById('zoom-in').addEventListener('click', () => {
  if (zoomIdx < ZOOM_STEPS.length - 1) { zoomIdx++; applyZoom(); }
});

applyZoom();

// ── Constants ────────────────────────────────────────────────────────────────

const CARD_COLOR_HEX = {
  Red:    '#c0392b',
  Green:  '#27ae60',
  Blue:   '#2980b9',
  Purple: '#8e44ad',
  Black:  '#616161',
  Yellow: '#c49b10',
};

const ZOOM_WIDTH_PX      = 280;
const ZOOM_ASPECT_RATIO  = 585 / 421; // card image height:width
const EMPTY_STATE_MESSAGE = 'Use the search box, filters, or click a keyword on any card to begin.';

// ── DOM references ───────────────────────────────────────────────────────────

const searchInput      = document.getElementById('query');
const statusBar        = document.getElementById('status');
const resultsContainer = document.getElementById('results');
const zoomImage        = document.getElementById('art-zoom');

// ── Hover zoom ───────────────────────────────────────────────────────────────

function showZoom(thumbEl) {
  zoomImage.src = thumbEl.src;
  zoomImage.style.display = 'block';

  const rect  = thumbEl.getBoundingClientRect();
  const zw    = ZOOM_WIDTH_PX;
  const zh    = zw * ZOOM_ASPECT_RATIO;

  // Try right of card first, then left
  let left = rect.right + 14;
  if (left + zw > window.innerWidth - 8) left = rect.left - zw - 14;
  // If card is very wide (grid layout), center the zoom above/below instead
  if (left < 8) left = Math.min(rect.left, window.innerWidth - zw - 8);

  let top = rect.top;
  if (top + zh > window.innerHeight - 8) top = window.innerHeight - zh - 8;

  zoomImage.style.left = Math.max(8, left) + 'px';
  zoomImage.style.top  = Math.max(8, top) + 'px';

  requestAnimationFrame(() => zoomImage.classList.add('visible'));
}

function hideZoom() {
  zoomImage.classList.remove('visible');
  zoomImage.addEventListener('transitionend', () => {
    if (!zoomImage.classList.contains('visible')) zoomImage.style.display = 'none';
  }, { once: true });
}

// ── Text helpers ─────────────────────────────────────────────────────────────

function escapeHtml(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

function stripHtml(html) {
  return (html || '')
    .replace(/<br[^>]*>/gi, '\n')
    .replace(/<[^>]+>/g, '')
    .replace(/&amp;/g, '&')
    .replace(/&lt;/g,  '<')
    .replace(/&gt;/g,  '>')
    .replace(/&nbsp;/g, ' ');
}

// Apply regex highlight to text nodes only, skipping HTML tag attributes.
function applyHighlight(html, searchRegex) {
  if (!searchRegex) return html;
  return html.replace(/(<[^>]+>)|([^<]+)/g, (_, tag, text) => {
    if (tag) return tag;
    return text.replace(searchRegex, m => `<mark>${escapeHtml(m)}</mark>`);
  });
}

function renderText(rawHtml, searchRegex) {
  const plain   = stripHtml(rawHtml);
  const escaped = escapeHtml(plain);

  const withKeywords = escaped.replace(/\[([^\]]+)\]/g, (_, kw) =>
    `<span class="kw" onclick="setKeyword('[${escapeHtml(kw)}]')">[${escapeHtml(kw)}]</span>`
  );

  return applyHighlight(withKeywords, searchRegex).replace(/\n/g, '<br>');
}

function imageUrl(rawPath) {
  const path = (rawPath || '').replace('../', '/');
  return `${API_BASE}/image?path=${encodeURIComponent(path)}`;
}

// ── Keyword / name filter helpers ────────────────────────────────────────────

function setKeyword(kw) {
  keywordInput.value = kw;
  doSearch();
}

function clearNameFilter() {
  document.getElementById('name-filter').value = '';
  doSearch();
}

// ── Series / set filter ──────────────────────────────────────────────────────

let allSets = []; // SetEntry objects: { code, series, rotated }

function activeSeries() {
  return [...document.querySelectorAll('#series-btns .filter-btn.active')]
    .map(b => b.dataset.series);
}

// ── Card render helpers ──────────────────────────────────────────────────────

function renderColorBadges(colors) {
  return (colors || [])
    .map(c => `<span class="color-badge" style="background:${CARD_COLOR_HEX[c] || '#555'}">${c}</span>`)
    .join('');
}

function renderTypePills(types) {
  return (types || [])
    .filter(t => t && t !== '-')
    .map(t => `<span class="type-pill">${escapeHtml(t)}</span>`)
    .join('');
}

function renderCostStat(card) {
  return card.card_type === 'LEADER'
    ? `<span class="stat"><label>Life</label>${card.life ?? '-'}</span>`
    : `<span class="stat"><label>Cost</label>${card.cost ?? '-'}</span>`;
}

function renderTrigger(card, searchRegex) {
  if (!card.trigger || card.trigger === '-') return '';
  return `<div class="card-trigger"><strong>Trigger:</strong> ${renderText(card.trigger, searchRegex)}</div>`;
}

function setupArtSwitcher(cardEl, allArts) {
  if (allArts.length <= 1) return;

  let currentArtIndex = 0;
  const imgEl    = cardEl.querySelector('.card-img');
  const artNav   = cardEl.querySelector('.art-nav');
  const artCount = cardEl.querySelector('.art-count');

  artNav.style.display   = 'flex';
  artCount.textContent   = `1/${allArts.length}`;

  cardEl.querySelector('.art-prev').addEventListener('click', () => {
    currentArtIndex = (currentArtIndex - 1 + allArts.length) % allArts.length;
    imgEl.src = imageUrl(allArts[currentArtIndex]);
    artCount.textContent = `${currentArtIndex + 1}/${allArts.length}`;
  });

  cardEl.querySelector('.art-next').addEventListener('click', () => {
    currentArtIndex = (currentArtIndex + 1) % allArts.length;
    imgEl.src = imageUrl(allArts[currentArtIndex]);
    artCount.textContent = `${currentArtIndex + 1}/${allArts.length}`;
  });
}

// ── Card renderer ────────────────────────────────────────────────────────────

function renderCard(card, searchRegex) {
  const cardEl = document.createElement('div');
  cardEl.className = 'card';
  if (card.colors && card.colors.length > 0) cardEl.dataset.color = card.colors[0];

  const fullText = stripHtml(card.text) + ' ' + stripHtml(card.trigger || '');
  if (isLeaderIncompatible(fullText)) cardEl.classList.add('leader-incompatible');
  else if (isLeaderSynergy(fullText)) cardEl.classList.add('leader-synergy');

  const allArts = [card.image_url, ...(card.alt_arts || [])];

  cardEl.innerHTML = `
    <div class="art-wrap">
      <img class="card-img" src="${imageUrl(card.image_url)}" alt="${escapeHtml(card.name)}" loading="lazy">
      <div class="art-nav">
        <button class="art-btn art-prev">&#8249;</button>
        <span class="art-count"></span>
        <button class="art-btn art-next">&#8250;</button>
      </div>
    </div>
    <div class="card-info">
      <div class="card-header">
        <span class="card-name">${escapeHtml(card.name)}</span>
        <span class="card-id">${escapeHtml(card.card_id)}</span>
        <span class="badge type-${card.card_type.toLowerCase()}">${card.card_type}</span>
        <span class="badge rarity-badge">${escapeHtml(card.rarity)}</span>
        ${renderColorBadges(card.colors)}
      </div>
      <div class="card-stats">
        ${renderCostStat(card)}
        <span class="stat"><label>Power</label>${card.power}</span>
        <span class="stat"><label>Counter</label>${card.counter}</span>
        <span class="stat"><label>Attr</label>${card.attribute}</span>
      </div>
      <div class="card-types">${renderTypePills(card.types)}</div>
      <div class="card-text">${renderText(card.text, searchRegex)}</div>
      ${renderTrigger(card, searchRegex)}
    </div>`;

  const imgEl = cardEl.querySelector('.card-img');
  imgEl.addEventListener('mouseenter', () => showZoom(imgEl));
  imgEl.addEventListener('mouseleave', hideZoom);

  setupArtSwitcher(cardEl, allArts);

  return cardEl;
}

// ── Search ───────────────────────────────────────────────────────────────────

function buildSearchParams() {
  const q              = searchInput.value.trim();
  const name           = document.getElementById('name-filter').value.trim();
  const colors         = [...document.querySelectorAll('.color-btn.active')].map(b => b.dataset.color);
  const types          = [...document.querySelectorAll('.type-btn.active')].map(b => b.dataset.type);
  const series         = activeSeries();
  const excludeRotated = document.getElementById('exclude-rotated-btn').classList.contains('active');
  const costMin        = document.getElementById('cost-min').value.trim();
  const costMax        = document.getElementById('cost-max').value.trim();
  const powerMin       = document.getElementById('power-min').value.trim();
  const powerMax       = document.getElementById('power-max').value.trim();
  const tagsInclude    = [];
  const tagsExclude    = [];
  tagStates.forEach((state, tag) => {
    if (state === 'include') tagsInclude.push(tag);
    else if (state === 'exclude') tagsExclude.push(tag);
  });

  return { q, name, colors, types, series, excludeRotated, costMin, costMax, powerMin, powerMax, tagsInclude, tagsExclude };
}

function showEmptyState(message) {
  resultsContainer.innerHTML = `<p class="empty">${message}</p>`;
}

async function doSearch() {
  const { q, name, colors, types, series, excludeRotated, costMin, costMax, powerMin, powerMax, tagsInclude, tagsExclude } = buildSearchParams();

  if (!q && !name && !colors.length && !types.length && keywordFilter.isEmpty() && costFilter.isEmpty()
      && !series.length && setFilter.isEmpty() && !excludeRotated && !costMin && !costMax
      && !powerMin && !powerMax && !tagsInclude.length && !tagsExclude.length) {
    searchInput.classList.remove('error');
    statusBar.textContent = '';
    statusBar.className   = '';
    showEmptyState(EMPTY_STATE_MESSAGE);
    return;
  }

  const params = new URLSearchParams();
  if (q)              params.set('q',         q);
  if (name)           params.set('name',      name);
  if (colors.length)  params.set('colors',    colors.join(','));
  if (types.length)   params.set('types',     types.join(','));
  if (costMin)        params.set('cost_min',  costMin);
  if (costMax)        params.set('cost_max',  costMax);
  if (powerMin)       params.set('power_min', powerMin);
  if (powerMax)       params.set('power_max', powerMax);
  keywordFilter.buildParams(params);
  costFilter.buildParams(params);
  setFilter.buildParams(params);
  if (series.length)  params.set('series', series.join(','));
  if (excludeRotated)           params.set('exclude_rotated', '1');
  if (tagsInclude.length)       params.set('tags_include',    tagsInclude.join(','));
  if (tagsExclude.length)       params.set('tags_exclude',    tagsExclude.join(','));

  let res, data;
  try {
    res  = await fetch(`${API_BASE}/api/search?${params}`);
    data = await res.json();
  } catch {
    statusBar.textContent = 'Network error';
    statusBar.className   = 'err';
    return;
  }

  if (!res.ok) {
    searchInput.classList.add('error');
    statusBar.textContent = data.error || 'Error';
    statusBar.className   = 'err';
    resultsContainer.innerHTML = '';
    return;
  }

  searchInput.classList.remove('error');

  let searchRegex = null;
  if (q) try { searchRegex = new RegExp(q, 'gi'); } catch { /* invalid regex — skip highlight */ }

  statusBar.className   = '';
  statusBar.textContent = `${data.length} card${data.length !== 1 ? 's' : ''} matched`;
  resultsContainer.innerHTML = '';

  if (!data.length) {
    showEmptyState('No cards matched.');
    return;
  }

  const frag = document.createDocumentFragment();
  for (const card of data) frag.appendChild(renderCard(card, searchRegex));
  resultsContainer.appendChild(frag);
}

// ── Leader compatibility ──────────────────────────────────────────────────────

let selectedLeader = null; // { name, types, colors }
let allLeaders     = [];

// Returns true if the card text contains a leader condition that the selected
// leader cannot satisfy (name mismatch or no matching archetype).
function isLeaderIncompatible(text) {
  if (!selectedLeader) return false;

  // "if your Leader is [Name]" — supports "or" variants: [X] or [Y]
  const nameMatches = [...text.matchAll(/if your leader is (\[[^\]]+\](?:\s+or\s+\[[^\]]+\])*)/gi)];
  for (const m of nameMatches) {
    const condNames = [...m[1].matchAll(/\[([^\]]+)\]/g)].map(n => n[1].toLowerCase());
    if (!condNames.some(n => selectedLeader.name.toLowerCase() === n)) return true;
  }

  // "if your Leader has the {Type} type" — supports "or" variants: {X} or {Y}
  const typeMatches = [...text.matchAll(/if your leader has the (\{[^}]+\}(?:\s+or\s+\{[^}]+\})*) type/gi)];
  for (const m of typeMatches) {
    const condTypes   = [...m[1].matchAll(/\{([^}]+)\}/g)].map(n => n[1].toLowerCase());
    const leaderTypes = (selectedLeader.types || []).map(t => t.toLowerCase());
    if (!condTypes.some(t => leaderTypes.includes(t))) return true;
  }

  return false;
}

// Returns true if the card text contains a leader condition that explicitly
// matches the selected leader (name or archetype). Cards flagged here synergize
// with the leader on purpose — as opposed to generic cards with no condition.
function isLeaderSynergy(text) {
  if (!selectedLeader) return false;

  const nameMatches = [...text.matchAll(/if your leader is (\[[^\]]+\](?:\s+or\s+\[[^\]]+\])*)/gi)];
  for (const m of nameMatches) {
    const condNames = [...m[1].matchAll(/\[([^\]]+)\]/g)].map(n => n[1].toLowerCase());
    if (condNames.some(n => selectedLeader.name.toLowerCase() === n)) return true;
  }

  const typeMatches = [...text.matchAll(/if your leader has the (\{[^}]+\}(?:\s+or\s+\{[^}]+\})*) type/gi)];
  for (const m of typeMatches) {
    const condTypes   = [...m[1].matchAll(/\{([^}]+)\}/g)].map(n => n[1].toLowerCase());
    const leaderTypes = (selectedLeader.types || []).map(t => t.toLowerCase());
    if (condTypes.some(t => leaderTypes.includes(t))) return true;
  }

  return false;
}

function applyLeaderColors(colors) {
  document.querySelectorAll('.color-btn').forEach(btn => {
    btn.classList.toggle('active', colors.includes(btn.dataset.color));
  });
}

function clearLeader() {
  document.getElementById('leader-input').value = '';
  document.getElementById('leader-dropdown').hidden = true;
  selectedLeader = null;
  applyLeaderColors([]); // clear colour filters
  doSearch();
}

// ── Tag (archetype) filter ───────────────────────────────────────────────────

const tagStates = new Map(); // name → 'include' | 'exclude'

function cycleTagState(name, btn) {
  const current = tagStates.get(name) || '';
  const next = current === '' ? 'include' : current === 'include' ? 'exclude' : '';
  if (next) {
    tagStates.set(name, next);
    btn.className = `tag-btn ${next}`;
  } else {
    tagStates.delete(name);
    btn.className = 'tag-btn';
  }
  doSearch();
}

function clearTags() {
  tagStates.clear();
  document.querySelectorAll('.tag-btn').forEach(btn => { btn.className = 'tag-btn'; });
  doSearch();
}

// ── Generic chip filter ───────────────────────────────────────────────────────

class ChipFilter {
  constructor(id, apiParam) {
    this.id       = id;
    this.apiParam = apiParam;
    this.includes = [];
    this.excludes = [];
    this.mode     = 'and'; // 'and' | 'or' for include logic
    this.items    = [];    // string[] or {text,count}[]
  }

  setItems(items) { this.items = items; }

  itemText(item) { return typeof item === 'string' ? item : item.text; }

  add(text, target) {
    const arr = target === 'include' ? this.includes : this.excludes;
    if (arr.includes(text)) return false;
    arr.push(text);
    this._render();
    return true;
  }

  remove(text, target) {
    const arr = target === 'include' ? this.includes : this.excludes;
    const i = arr.indexOf(text);
    if (i >= 0) arr.splice(i, 1);
    this._render();
  }

  _render() {
    for (const target of ['include', 'exclude']) {
      const inputId = target === 'include' ? `${this.id}-filter` : `${this.id}-exclude`;
      const wrap    = document.getElementById(`${this.id}-${target}-wrap`);
      const input   = document.getElementById(inputId);
      wrap.querySelectorAll('.keyword-chip').forEach(c => c.remove());
      (target === 'include' ? this.includes : this.excludes).forEach(text => {
        wrap.insertBefore(this._makeChip(text, target), input);
      });
    }
  }

  _makeChip(text, target) {
    const chip = document.createElement('span');
    chip.className = `keyword-chip ${target}`;
    const label = document.createElement('span');
    label.textContent = text;
    const btn = document.createElement('button');
    btn.textContent = '×';
    btn.addEventListener('mousedown', e => e.preventDefault());
    btn.addEventListener('click', () => { this.remove(text, target); doSearch(); });
    chip.append(label, btn);
    return chip;
  }

  buildParams(params) {
    if (this.includes.length) {
      params.set(this.apiParam, this.includes.join(','));
      if (this.mode === 'or') params.set(`${this.apiParam}_mode`, 'or');
    }
    if (this.excludes.length) {
      params.set(`${this.apiParam}_exclude`, this.excludes.join(','));
    }
  }

  isEmpty() { return !this.includes.length && !this.excludes.length; }
}

const keywordFilter = new ChipFilter('keyword', 'keyword');
const costFilter    = new ChipFilter('cost', 'cost');
const setFilter     = new ChipFilter('set', 'sets');

// ── Shared chip dropdown ──────────────────────────────────────────────────────

let activeChipFilter = null; // ChipFilter currently driving the dropdown
let activeChipTarget = null; // 'include' | 'exclude'

function positionChipDropdown() {
  const wrapId = `${activeChipFilter.id}-${activeChipTarget}-wrap`;
  const rect   = document.getElementById(wrapId).getBoundingClientRect();
  const dd     = document.getElementById('chip-dropdown');
  dd.style.top      = (rect.bottom + 4) + 'px';
  dd.style.left     = rect.left + 'px';
  dd.style.minWidth = Math.max(220, rect.width) + 'px';
}

function buildChipDropdown(query) {
  const dd = document.getElementById('chip-dropdown');
  if (!activeChipFilter) { dd.hidden = true; return; }

  const current  = activeChipTarget === 'include' ? activeChipFilter.includes : activeChipFilter.excludes;
  const q        = query.toLowerCase();
  const filtered = (q
    ? activeChipFilter.items.filter(it => activeChipFilter.itemText(it).toLowerCase().includes(q))
    : activeChipFilter.items
  ).slice(0, 60);

  if (!filtered.length) { dd.hidden = true; return; }

  dd.innerHTML = '';
  filtered.forEach(it => {
    const text = activeChipFilter.itemText(it);
    const el   = document.createElement('div');
    el.className   = 'keyword-option' + (current.includes(text) ? ' already-added' : '');
    el.textContent = text;
    el.addEventListener('mousedown', e => e.preventDefault());
    el.addEventListener('click', () => {
      if (current.includes(text)) return;
      activeChipFilter.add(text, activeChipTarget);
      const inputId = activeChipTarget === 'include'
        ? `${activeChipFilter.id}-filter` : `${activeChipFilter.id}-exclude`;
      document.getElementById(inputId).value = '';
      buildChipDropdown('');
      doSearch();
    });
    dd.appendChild(el);
  });

  positionChipDropdown();
  dd.hidden = false;
}

function openChipDropdown(filter, target) {
  activeChipFilter = filter;
  activeChipTarget = target;
  buildChipDropdown('');
}

function closeChipDropdown() {
  activeChipFilter = null;
  activeChipTarget = null;
  document.getElementById('chip-dropdown').hidden = true;
}

function wireChipInput(inputEl, filter, target) {
  inputEl.closest('.chip-input-wrap').addEventListener('click', () => {
    inputEl.focus();
    openChipDropdown(filter, target);
  });
  inputEl.addEventListener('focus',  () => openChipDropdown(filter, target));
  inputEl.addEventListener('input',  () => buildChipDropdown(inputEl.value));
  inputEl.addEventListener('blur',   () => setTimeout(closeChipDropdown, 200));
  inputEl.addEventListener('keydown', e => {
    if (e.key === 'Enter') {
      e.preventDefault();
      const text = inputEl.value.trim();
      if (text) {
        filter.add(text, target);
        inputEl.value = '';
        buildChipDropdown('');
        doSearch();
      }
    }
  });
  inputEl.removeAttribute('readonly');
}

function wireModeBtn(btnEl, filter) {
  btnEl.addEventListener('click', () => {
    filter.mode = filter.mode === 'and' ? 'or' : 'and';
    btnEl.textContent = filter.mode === 'and' ? 'ALL' : 'ANY';
    btnEl.classList.toggle('or-mode', filter.mode === 'or');
    if (filter.includes.length) doSearch();
  });
}

// ── API loaders ──────────────────────────────────────────────────────────────

// loadMeta fetches /api/meta once to populate sets, keywords, and types.
function loadMeta() {
  fetch(`${API_BASE}/api/meta`).then(r => r.json()).then(({ sets, keywords, types, costs }) => {
    // Keywords and costs — populate chip filter dropdowns
    keywordFilter.setItems(keywords);
    costFilter.setItems(costs);

    // Sets — store as SetEntry objects, populate set chip filter, build series buttons
    allSets = sets;
    setFilter.setItems(sets.map(s => s.code));
    const seriesList = [...new Set(sets.map(s => s.series))].sort();
    const seriesBtns = document.getElementById('series-btns');
    seriesList.forEach(sr => {
      const btn = document.createElement('button');
      btn.className      = 'filter-btn';
      btn.dataset.series = sr;
      btn.textContent    = sr;
      btn.addEventListener('click', () => {
        btn.classList.toggle('active');
        doSearch();
      });
      seriesBtns.appendChild(btn);
    });

    // Types (archetype tag buttons)
    const tagList = document.getElementById('tag-list');
    types.forEach(({ name, count }) => {
      const btn = document.createElement('button');
      btn.className = 'tag-btn';
      btn.dataset.name = name;
      btn.innerHTML = `${escapeHtml(name)}<span class="tag-count">${count}</span>`;
      btn.addEventListener('click', () => cycleTagState(name, btn));
      tagList.appendChild(btn);
    });
    document.getElementById('tag-search').addEventListener('input', () => {
      const q = document.getElementById('tag-search').value.toLowerCase();
      document.querySelectorAll('.tag-btn').forEach(btn => {
        btn.style.display = btn.dataset.name.toLowerCase().includes(q) ? '' : 'none';
      });
    });
  });
}

function loadLeaders() {
  fetch(`${API_BASE}/api/leaders`)
    .then(r => r.json())
    .then(leaders => { allLeaders = leaders; });
}

function positionLeaderDropdown() {
  const rect     = document.getElementById('leader-input').getBoundingClientRect();
  const dropdown = document.getElementById('leader-dropdown');
  dropdown.style.top     = (rect.bottom + 4) + 'px';
  dropdown.style.left    = rect.left + 'px';
  dropdown.style.minWidth = Math.max(320, rect.width) + 'px';
}

function buildLeaderDropdown(query) {
  const dropdown = document.getElementById('leader-dropdown');
  const q = query.toLowerCase().trim();

  const filtered = q
    ? allLeaders.filter(l =>
        l.name.toLowerCase().includes(q) ||
        l.card_id.toLowerCase().includes(q)
      )
    : allLeaders;

  if (!filtered.length) { dropdown.hidden = true; return; }

  dropdown.innerHTML = '';
  filtered.slice(0, 30).forEach(leader => {
    const item = document.createElement('div');
    item.className = 'leader-option';

    const types  = (leader.types  || []).join(', ');
    const colors = (leader.colors || []).join('/');
    const meta   = [leader.card_id, types, colors].filter(Boolean).join(' · ');

    item.innerHTML = `
      <img class="leader-option-img" src="${imageUrl(leader.image_url)}" alt="" loading="lazy">
      <div class="leader-option-info">
        <div class="leader-option-name">${escapeHtml(leader.name)}</div>
        <div class="leader-option-meta">${escapeHtml(meta)}</div>
      </div>`;

    item.addEventListener('mousedown', e => e.preventDefault()); // keep focus on input
    item.addEventListener('click', () => {
      document.getElementById('leader-input').value = `${leader.card_id}  ${leader.name}`;
      selectedLeader = { name: leader.name, types: leader.types || [], colors: leader.colors || [] };
      dropdown.hidden = true;
      applyLeaderColors(leader.colors || []);
      doSearch();
    });
    dropdown.appendChild(item);
  });

  positionLeaderDropdown();
  dropdown.hidden = false;
}


// ── Init ─────────────────────────────────────────────────────────────────────

function init() {
  const headerEl = document.querySelector('header');
  let headerCollapsed = false;
  let lastScrollY = window.scrollY;
  window.addEventListener('scroll', () => {
    const y = window.scrollY;
    if (!headerCollapsed && y > 120) {
      headerCollapsed = true;
      headerEl.classList.add('collapsed');
    } else if (headerCollapsed && y <= 1 && y < lastScrollY) {
      // Only re-open when actively scrolling upward to the very top,
      // not from a layout shift caused by the collapse itself.
      headerCollapsed = false;
      headerEl.classList.remove('collapsed');
    }
    lastScrollY = y;
  }, { passive: true });

// Text inputs fire search only on Enter or the Search button, not on every keystroke.
  const textInputs = [
    searchInput,
    document.getElementById('name-filter'),
    document.getElementById('cost-min'),
    document.getElementById('cost-max'),
    document.getElementById('power-min'),
    document.getElementById('power-max'),
  ];
  textInputs.forEach(el => {
    el.addEventListener('keydown', e => { if (e.key === 'Enter') doSearch(); });
  });

  document.getElementById('search-btn').addEventListener('click', doSearch);

  // Keyword chip inputs
  wireChipInput(document.getElementById('keyword-filter'), keywordFilter, 'include');
  wireChipInput(document.getElementById('keyword-exclude'), keywordFilter, 'exclude');
  wireModeBtn(document.getElementById('keyword-mode-btn'), keywordFilter);

  // Cost chip inputs
  wireChipInput(document.getElementById('cost-filter'), costFilter, 'include');
  wireChipInput(document.getElementById('cost-exclude'), costFilter, 'exclude');
  wireModeBtn(document.getElementById('cost-mode-btn'), costFilter);

  // Set chip inputs
  wireChipInput(document.getElementById('set-filter'), setFilter, 'include');
  wireChipInput(document.getElementById('set-exclude'), setFilter, 'exclude');

  // Shared chip dropdown repositioning
  const chipDropdown = document.getElementById('chip-dropdown');
  window.addEventListener('scroll', () => {
    if (!chipDropdown.hidden) positionChipDropdown();
  }, { passive: true });
  window.addEventListener('resize', () => {
    if (!chipDropdown.hidden) positionChipDropdown();
  });

  document.querySelectorAll('.filter-btn').forEach(btn =>
    btn.addEventListener('click', () => { btn.classList.toggle('active'); doSearch(); })
  );

  document.querySelectorAll('.example-chip').forEach(chip =>
    chip.addEventListener('click', () => { searchInput.value = chip.dataset.q; doSearch(); })
  );

  const leaderInput    = document.getElementById('leader-input');
  const leaderDropdown = document.getElementById('leader-dropdown');

  leaderInput.addEventListener('focus', () => buildLeaderDropdown(leaderInput.value));
  leaderInput.addEventListener('input', () => {
    selectedLeader = null;
    buildLeaderDropdown(leaderInput.value);
    if (!leaderInput.value.trim()) doSearch();
  });
  leaderInput.addEventListener('blur', () => {
    setTimeout(() => { leaderDropdown.hidden = true; }, 200);
  });
  window.addEventListener('scroll', () => {
    if (!leaderDropdown.hidden) positionLeaderDropdown();
  }, { passive: true });
  window.addEventListener('resize', () => {
    if (!leaderDropdown.hidden) positionLeaderDropdown();
  });

  loadMeta();
  loadLeaders();
}

init();
