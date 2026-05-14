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
const SEARCH_DEBOUNCE_MS = 300;

const EMPTY_STATE_MESSAGE = 'Use the search box, filters, or click a keyword on any card to begin.';

// ── DOM references ───────────────────────────────────────────────────────────

const searchInput      = document.getElementById('query');
const keywordInput     = document.getElementById('keyword-filter');
const keywordExcludeInput = document.getElementById('keyword-exclude');
const setInput         = document.getElementById('set-filter');
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
  return `/image?path=${encodeURIComponent(path)}`;
}

// ── Keyword filter ───────────────────────────────────────────────────────────

function setKeyword(kw) {
  keywordInput.value = kw;
  doSearch();
}

function clearKeyword() {
  keywordInput.value = '';
  doSearch();
}

function clearKeywordExclude() {
  keywordExcludeInput.value = '';
  doSearch();
}

// ── Series / set filter ──────────────────────────────────────────────────────

let allSets = [];

function seriesOf(setCode) {
  return setCode.replace(/\d+$/, '') || setCode;
}

function activeSeries() {
  return [...document.querySelectorAll('#series-btns .filter-btn.active')]
    .map(b => b.dataset.series);
}

function updateSetDatalist() {
  const active = activeSeries();
  const setDatalist = document.getElementById('set-list');
  setDatalist.innerHTML = '';
  const visible = active.length
    ? allSets.filter(s => active.includes(seriesOf(s)))
    : allSets;
  visible.forEach(s => {
    const opt = document.createElement('option');
    opt.value = s;
    setDatalist.appendChild(opt);
  });
}

function clearSet() {
  setInput.value = '';
  doSearch();
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

  const fullText = stripHtml(card.text) + ' ' + stripHtml(card.trigger || '');
  if (isLeaderIncompatible(fullText)) cardEl.classList.add('leader-incompatible');

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
  const colors         = [...document.querySelectorAll('.color-btn.active')].map(b => b.dataset.color);
  const types          = [...document.querySelectorAll('.type-btn.active')].map(b => b.dataset.type);
  const keyword        = keywordInput.value.trim();
  const keywordExclude = keywordExcludeInput.value.trim();
  const series         = activeSeries();
  const set            = setInput.value.trim();
  const excludeRotated = document.getElementById('exclude-rotated-btn').classList.contains('active');
  const tagsInclude    = [];
  const tagsExclude    = [];
  tagStates.forEach((state, name) => {
    if (state === 'include') tagsInclude.push(name);
    else if (state === 'exclude') tagsExclude.push(name);
  });

  return { q, colors, types, keyword, keywordExclude, series, set, excludeRotated, tagsInclude, tagsExclude };
}

function showEmptyState(message) {
  resultsContainer.innerHTML = `<p class="empty">${message}</p>`;
}

async function doSearch() {
  const { q, colors, types, keyword, keywordExclude, series, set, excludeRotated, tagsInclude, tagsExclude } = buildSearchParams();

  if (!q && !colors.length && !types.length && !keyword && !keywordExclude && !series.length && !set && !excludeRotated && !tagsInclude.length && !tagsExclude.length) {
    searchInput.classList.remove('error');
    statusBar.textContent = '';
    statusBar.className   = '';
    showEmptyState(EMPTY_STATE_MESSAGE);
    return;
  }

  const params = new URLSearchParams();
  if (q)                params.set('q',               q);
  if (colors.length)    params.set('colors',          colors.join(','));
  if (types.length)     params.set('types',           types.join(','));
  if (keyword)          params.set('keyword',         keyword);
  if (keywordExclude)   params.set('keyword_exclude', keywordExclude);
  if (series.length)    params.set('series',          series.join(','));
  if (set)              params.set('sets',            set);
  if (excludeRotated)       params.set('exclude_rotated', '1');
  if (tagsInclude.length)   params.set('tags_include',    tagsInclude.join(','));
  if (tagsExclude.length)   params.set('tags_exclude',    tagsExclude.join(','));

  let res, data;
  try {
    res  = await fetch(`/search?${params}`);
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

function clearLeader() {
  document.getElementById('leader-input').value = '';
  selectedLeader = null;
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

// ── API loaders ──────────────────────────────────────────────────────────────

function loadKeywords() {
  fetch('/keywords').then(r => r.json()).then(keywords => {
    const keywordDatalist = document.getElementById('keyword-list');
    keywords.forEach(kw => {
      const opt = document.createElement('option');
      opt.value = kw;
      keywordDatalist.appendChild(opt);
    });
  });
}

function loadLeaders() {
  fetch('/search?types=LEADER')
    .then(r => r.json())
    .then(leaders => {
      allLeaders = leaders;
      const dl   = document.getElementById('leader-list');
      const seen = new Set();
      leaders.forEach(l => {
        if (seen.has(l.name)) return;
        seen.add(l.name);
        const opt = document.createElement('option');
        opt.value = l.name;
        dl.appendChild(opt);
      });
    });
}

function loadTypes() {
  fetch('/types').then(r => r.json()).then(types => {
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

function loadSets() {
  fetch('/sets').then(r => r.json()).then(sets => {
    allSets = sets;

    const seriesList = [...new Set(sets.map(seriesOf))].sort();
    const seriesBtns = document.getElementById('series-btns');
    seriesList.forEach(sr => {
      const btn = document.createElement('button');
      btn.className      = 'filter-btn';
      btn.dataset.series = sr;
      btn.textContent    = sr;
      btn.addEventListener('click', () => {
        btn.classList.toggle('active');
        updateSetDatalist();
        const setVal = setInput.value.trim();
        const active = activeSeries();
        if (setVal && active.length && !active.includes(seriesOf(setVal))) {
          setInput.value = '';
        }
        doSearch();
      });
      seriesBtns.appendChild(btn);
    });

    updateSetDatalist();
  });
}

// ── Init ─────────────────────────────────────────────────────────────────────

function init() {
  const headerEl = document.querySelector('header');
  let prevScrollY = window.scrollY;
  window.addEventListener('scroll', () => {
    const y = window.scrollY;
    if (y > prevScrollY && y > 60) {
      headerEl.classList.add('collapsed');
    } else if (y < prevScrollY || y <= 5) {
      headerEl.classList.remove('collapsed');
    }
    prevScrollY = y;
  }, { passive: true });

  let debounceTimer = null;
  function scheduleSearch() {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(doSearch, SEARCH_DEBOUNCE_MS);
  }

  searchInput.addEventListener('input', scheduleSearch);
  keywordInput.addEventListener('input', scheduleSearch);
  keywordExcludeInput.addEventListener('input', scheduleSearch);
  setInput.addEventListener('input', scheduleSearch);

  document.querySelectorAll('.filter-btn').forEach(btn =>
    btn.addEventListener('click', () => { btn.classList.toggle('active'); doSearch(); })
  );

  document.querySelectorAll('.example-chip').forEach(chip =>
    chip.addEventListener('click', () => { searchInput.value = chip.dataset.q; doSearch(); })
  );

  document.getElementById('leader-input').addEventListener('input', () => {
    const name  = document.getElementById('leader-input').value.trim();
    const match = allLeaders.find(l => l.name === name);
    selectedLeader = match
      ? { name: match.name, types: match.types || [], colors: match.colors || [] }
      : null;
    doSearch();
  });

  loadKeywords();
  loadSets();
  loadTypes();
  loadLeaders();
}

init();
