package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ── Data types ────────────────────────────────────────────────────────────────

type Card struct {
	CardID    string   `json:"card_id"`
	Rarity    string   `json:"rarity"`
	CardType  string   `json:"card_type"`
	Name      string   `json:"name"`
	Life      string   `json:"life,omitempty"`
	Cost      string   `json:"cost,omitempty"`
	Attribute string   `json:"attribute"`
	Power     string   `json:"power"`
	Counter   string   `json:"counter"`
	Colors    []string `json:"colors"`
	Types     []string `json:"types"`
	Text      string   `json:"text"`
	ArtSet    string   `json:"art_set"`
	ImageURL  string   `json:"image_url"`
	Trigger   string   `json:"trigger"`
}

// CardResult is one entry per card_id with alt art URLs bundled in.
// costVal and powerVal are pre-parsed at startup; unexported so they are not
// included in JSON responses.
type CardResult struct {
	Card
	AltArts  []string `json:"alt_arts"`
	costVal  int      // -1 when non-numeric (leaders, events, stages)
	powerVal int      // -1 when non-numeric
}

// SetEntry is returned by /api/sets.
type SetEntry struct {
	Code    string `json:"code"`
	Series  string `json:"series"`
	Rotated bool   `json:"rotated"`
}

// TypeEntry is returned by /api/types: archetype name + unique card count.
type TypeEntry struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// CostEntry is returned by /api/costs: a distinct effect-cost pattern and how often it appears.
type CostEntry struct {
	Text  string `json:"text"`
	Count int    `json:"count"`
}

// MetaResponse is returned by /api/meta for client bootstrap.
type MetaResponse struct {
	Sets     []SetEntry  `json:"sets"`
	Keywords []string    `json:"keywords"`
	Types    []TypeEntry `json:"types"`
	Costs    []CostEntry `json:"costs"`
}

// ── Global state ──────────────────────────────────────────────────────────────

var allCards []CardResult

// permittedCards holds card IDs that are legal in Standard Regulation even
// though their set is otherwise rotated (sourced from the block-icon page).
var permittedCards map[string]bool

var (
	htmlTagRe = regexp.MustCompile(`<[^>]+>`)
	keywordRe = regexp.MustCompile(`\[[^\]]+\]`)
	donRe     = regexp.MustCompile(`[①②③④⑤⑥⑦⑧⑨⑩]+`)
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func plainText(html string) string {
	return htmlTagRe.ReplaceAllString(html, " ")
}

// cardSet returns the set code portion of a card ID (e.g. "OP05" from "OP05-093").
func cardSet(id string) string {
	if i := strings.IndexByte(id, '-'); i >= 0 {
		return id[:i]
	}
	return id
}

// seriesFromSetCode strips trailing digits from a set code (e.g. "OP" from "OP05").
func seriesFromSetCode(setCode string) string {
	i := len(setCode)
	for i > 0 && setCode[i-1] >= '0' && setCode[i-1] <= '9' {
		i--
	}
	return setCode[:i]
}

// cardSeries returns the series prefix for a full card ID (e.g. "OP" from "OP05-093").
func cardSeries(id string) string {
	return seriesFromSetCode(cardSet(id))
}

// isRotated reports whether a set code belongs to the rotated (non-Standard) pool.
// Rotated: OP01–OP04, ST01–ST09.
func isRotated(setCode string) bool {
	if strings.HasPrefix(setCode, "OP") {
		n, err := strconv.Atoi(strings.TrimPrefix(setCode, "OP"))
		return err == nil && n < 5
	}
	if strings.HasPrefix(setCode, "ST") {
		n, err := strconv.Atoi(strings.TrimPrefix(setCode, "ST"))
		return err == nil && n < 10
	}
	return false
}

// extractEffectCosts returns the cost portions of all effects in plain card text.
// It strips [keyword] brackets (removing internal colons like Activate:Main),
// then collects the text before each remaining ':' separator. Multiple effects
// in one text block are separated by '.', so we take the last sentence before ':'.
func extractEffectCosts(plain string) string {
	stripped := keywordRe.ReplaceAllString(plain, "")
	parts := strings.Split(stripped, ":")
	var costs []string
	for i := 0; i < len(parts)-1; i++ {
		seg := parts[i]
		if dot := strings.LastIndex(seg, "."); dot >= 0 {
			seg = seg[dot+1:]
		}
		if seg = strings.TrimSpace(seg); seg != "" {
			costs = append(costs, seg)
		}
	}
	return strings.Join(costs, " ")
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// ── CORS middleware ───────────────────────────────────────────────────────────

func withCORS(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h(w, r)
	}
}

// ── Entry point ───────────────────────────────────────────────────────────────

func main() {
	data, err := os.ReadFile("card_list.json")
	if err != nil {
		log.Fatal(err)
	}
	var rawCards []Card
	if err := json.Unmarshal(data, &rawCards); err != nil {
		log.Fatal(err)
	}
	// Pre-group alt arts and pre-parse numeric fields so handlers never repeat this work.
	idxByID := make(map[string]int, len(rawCards))
	for _, raw := range rawCards {
		if idx, seen := idxByID[raw.CardID]; seen {
			allCards[idx].AltArts = append(allCards[idx].AltArts, raw.ImageURL)
			continue
		}
		cv, err := strconv.Atoi(raw.Cost)
		if err != nil {
			cv = -1
		}
		pv, err := strconv.Atoi(raw.Power)
		if err != nil {
			pv = -1
		}
		idxByID[raw.CardID] = len(allCards)
		allCards = append(allCards, CardResult{Card: raw, AltArts: []string{}, costVal: cv, powerVal: pv})
	}
	log.Printf("Loaded %d cards (%d unique)", len(rawCards), len(allCards))

	permittedCards = make(map[string]bool)
	if pb, err := os.ReadFile("permitted_cards.json"); err == nil {
		var ids []string
		if err := json.Unmarshal(pb, &ids); err == nil {
			for _, id := range ids {
				permittedCards[id] = true
			}
			log.Printf("Loaded %d permitted cards", len(ids))
		}
	} else {
		log.Println("permitted_cards.json not found; rotation exceptions disabled")
	}

	mux := http.NewServeMux()

	// API routes — all CORS-enabled, all under /api/
	mux.HandleFunc("GET /api/search", withCORS(searchHandler))
	mux.HandleFunc("GET /api/leaders", withCORS(leadersHandler))
	mux.HandleFunc("GET /api/card/{id}", withCORS(cardHandler))
	mux.HandleFunc("GET /api/sets", withCORS(setsHandler))
	mux.HandleFunc("GET /api/keywords", withCORS(keywordsHandler))
	mux.HandleFunc("GET /api/types", withCORS(typesHandler))
	mux.HandleFunc("GET /api/costs", withCORS(costsHandler))
	mux.HandleFunc("GET /api/meta", withCORS(metaHandler))
	// OPTIONS preflight catch-all for the /api/ subtree
	mux.HandleFunc("OPTIONS /api/", withCORS(func(w http.ResponseWriter, r *http.Request) {}))

	// Image proxy
	mux.HandleFunc("GET /image", withCORS(imageProxyHandler))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}
	log.Printf("Listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

// ── Search ────────────────────────────────────────────────────────────────────

func searchHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	colorsParam := r.URL.Query().Get("colors")
	typesParam := r.URL.Query().Get("types")
	keywordParam := r.URL.Query().Get("keyword")
	keywordMode := r.URL.Query().Get("keyword_mode") // "and" (default) or "or"
	keywordExclude := r.URL.Query().Get("keyword_exclude")
	costParam := r.URL.Query().Get("cost")
	costMode := r.URL.Query().Get("cost_mode")
	costExclude := r.URL.Query().Get("cost_exclude")
	setsParam        := r.URL.Query().Get("sets")
	setsExcludeParam := r.URL.Query().Get("sets_exclude")
	seriesParam      := r.URL.Query().Get("series")
	tagsIncludeParam := r.URL.Query().Get("tags_include")
	tagsExcludeParam := r.URL.Query().Get("tags_exclude")
	excludeRotated   := r.URL.Query().Get("exclude_rotated") == "1"
	nameParam        := r.URL.Query().Get("name")
	costMinParam     := r.URL.Query().Get("cost_min")
	costMaxParam     := r.URL.Query().Get("cost_max")
	powerMinParam    := r.URL.Query().Get("power_min")
	powerMaxParam    := r.URL.Query().Get("power_max")

	if q == "" && colorsParam == "" && typesParam == "" && keywordParam == "" &&
		keywordExclude == "" && costParam == "" && costExclude == "" && setsParam == "" &&
		setsExcludeParam == "" && seriesParam == "" && tagsIncludeParam == "" &&
		tagsExcludeParam == "" && !excludeRotated && nameParam == "" &&
		costMinParam == "" && costMaxParam == "" && powerMinParam == "" && powerMaxParam == "" {
		writeJSON(w, []CardResult{})
		return
	}

	var re *regexp.Regexp
	if q != "" {
		var err error
		re, err = regexp.Compile("(?i)" + q)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid regex: "+err.Error())
			return
		}
	}

	var nameRe *regexp.Regexp
	if nameParam != "" {
		var err error
		nameRe, err = regexp.Compile("(?i)" + nameParam)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid name regex: "+err.Error())
			return
		}
	}

	costMin, costMax := -1, -1
	if costMinParam != "" {
		if v, err := strconv.Atoi(costMinParam); err == nil {
			costMin = v
		}
	}
	if costMaxParam != "" {
		if v, err := strconv.Atoi(costMaxParam); err == nil {
			costMax = v
		}
	}

	powerMin, powerMax := -1, -1
	if powerMinParam != "" {
		if v, err := strconv.Atoi(powerMinParam); err == nil {
			powerMin = v
		}
	}
	if powerMaxParam != "" {
		if v, err := strconv.Atoi(powerMaxParam); err == nil {
			powerMax = v
		}
	}

	keywords := splitList(keywordParam)
	keywordExcludes := splitList(keywordExclude)
	keywordOR := keywordMode == "or"
	costOR := costMode == "or"
	costIncludeREs, err := compileRegexList(splitList(costParam))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid cost regex: "+err.Error())
		return
	}
	costExcludeREs, err := compileRegexList(splitList(costExclude))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid cost_exclude regex: "+err.Error())
		return
	}

	filterColors := splitParam(colorsParam)
	filterTypes := splitParam(typesParam)
	filterSets        := splitParam(setsParam)
	filterSetsExclude := splitParam(setsExcludeParam)
	filterSeries      := splitParam(seriesParam)
	filterTagsInclude := splitParam(tagsIncludeParam)
	filterTagsExclude := splitParam(tagsExcludeParam)

	results := make([]CardResult, 0)

	for _, card := range allCards {
		plain := plainText(card.Text)
		plainTrigger := plainText(card.Trigger)

		if re != nil && !re.MatchString(plain) && !re.MatchString(plainTrigger) {
			continue
		}
		if nameRe != nil && !nameRe.MatchString(card.Name) {
			continue
		}
		if costMin >= 0 || costMax >= 0 {
			if card.costVal < 0 || (costMin >= 0 && card.costVal < costMin) || (costMax >= 0 && card.costVal > costMax) {
				continue
			}
		}
		if powerMin >= 0 || powerMax >= 0 {
			if card.powerVal < 0 || (powerMin >= 0 && card.powerVal < powerMin) || (powerMax >= 0 && card.powerVal > powerMax) {
				continue
			}
		}
		if filterColors != nil && !anyMatch(card.Colors, filterColors) {
			continue
		}
		if filterTypes != nil && !filterTypes[card.CardType] {
			continue
		}
		if len(keywords) > 0 {
			if keywordOR {
				found := false
				for _, kw := range keywords {
					if strings.Contains(plain, kw) || strings.Contains(plainTrigger, kw) {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			} else {
				skip := false
				for _, kw := range keywords {
					if !strings.Contains(plain, kw) && !strings.Contains(plainTrigger, kw) {
						skip = true
						break
					}
				}
				if skip {
					continue
				}
			}
		}
		skip := false
		for _, kw := range keywordExcludes {
			if strings.Contains(plain, kw) || strings.Contains(plainTrigger, kw) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		if filterSets != nil && !filterSets[cardSet(card.CardID)] {
			continue
		}
		if filterSetsExclude != nil && filterSetsExclude[cardSet(card.CardID)] {
			continue
		}
		if filterSeries != nil && !filterSeries[cardSeries(card.CardID)] {
			continue
		}
		if excludeRotated && isRotated(cardSet(card.CardID)) && !permittedCards[card.CardID] {
			continue
		}
		if filterTagsInclude != nil && !anyMatch(card.Types, filterTagsInclude) {
			continue
		}
		if filterTagsExclude != nil && anyMatch(card.Types, filterTagsExclude) {
			continue
		}
		if len(costIncludeREs) > 0 || len(costExcludeREs) > 0 {
			costText := extractEffectCosts(plain) + " " + extractEffectCosts(plainTrigger)
			passed := true
			if len(costIncludeREs) > 0 {
				if costOR {
					found := false
					for _, re := range costIncludeREs {
						if re.MatchString(costText) {
							found = true
							break
						}
					}
					passed = found
				} else {
					for _, re := range costIncludeREs {
						if !re.MatchString(costText) {
							passed = false
							break
						}
					}
				}
			}
			if passed {
				for _, re := range costExcludeREs {
					if re.MatchString(costText) {
						passed = false
						break
					}
				}
			}
			if !passed {
				continue
			}
		}

		results = append(results, card)
	}

	writeJSON(w, results)
}

// compileRegexList compiles a slice of pattern strings into case-insensitive regexps.
func compileRegexList(patterns []string) ([]*regexp.Regexp, error) {
	res := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile("(?i)" + p)
		if err != nil {
			return nil, err
		}
		res = append(res, re)
	}
	return res, nil
}

// splitList splits a comma-separated param into a trimmed string slice, or nil if empty.
func splitList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// splitParam splits a comma-separated query param into a lookup set, or nil if empty.
func splitParam(s string) map[string]bool {
	if s == "" {
		return nil
	}
	m := make(map[string]bool)
	for _, v := range strings.Split(s, ",") {
		m[strings.TrimSpace(v)] = true
	}
	return m
}

// anyMatch reports whether any value in vals is present in the allowed set.
func anyMatch(vals []string, allowed map[string]bool) bool {
	for _, v := range vals {
		if allowed[v] {
			return true
		}
	}
	return false
}

// ── Leaders ───────────────────────────────────────────────────────────────────

// leadersHandler returns every unique leader card with alt arts bundled,
// sorted alphabetically by name then by card ID.
func leadersHandler(w http.ResponseWriter, r *http.Request) {
	results := make([]CardResult, 0)
	for _, card := range allCards {
		if card.CardType == "LEADER" {
			results = append(results, card)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		ni, nj := strings.ToLower(results[i].Name), strings.ToLower(results[j].Name)
		if ni != nj {
			return ni < nj
		}
		return results[i].CardID < results[j].CardID
	})
	writeJSON(w, results)
}

// ── Single card ───────────────────────────────────────────────────────────────

// cardHandler returns a single card by ID with alt arts bundled.
func cardHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	for _, card := range allCards {
		if card.CardID == id {
			writeJSON(w, card)
			return
		}
	}
	http.NotFound(w, r)
}

// ── Sets ──────────────────────────────────────────────────────────────────────

// setsHandler returns all known sets with their series and rotation status.
func setsHandler(w http.ResponseWriter, r *http.Request) {
	seen := make(map[string]bool)
	for _, card := range allCards {
		seen[cardSet(card.CardID)] = true
	}
	result := make([]SetEntry, 0, len(seen))
	for code := range seen {
		result = append(result, SetEntry{
			Code:    code,
			Series:  seriesFromSetCode(code),
			Rotated: isRotated(code),
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Code < result[j].Code })
	writeJSON(w, result)
}

// ── Keywords ──────────────────────────────────────────────────────────────────

func keywordsHandler(w http.ResponseWriter, r *http.Request) {
	seen := make(map[string]bool)
	for _, card := range allCards {
		for _, kw := range keywordRe.FindAllString(plainText(card.Text), -1) {
			seen[kw] = true
		}
		for _, kw := range keywordRe.FindAllString(card.Trigger, -1) {
			seen[kw] = true
		}
	}
	keywords := make([]string, 0, len(seen))
	for kw := range seen {
		keywords = append(keywords, kw)
	}
	sort.Strings(keywords)
	writeJSON(w, keywords)
}

// ── Types (archetypes) ────────────────────────────────────────────────────────

func typesHandler(w http.ResponseWriter, r *http.Request) {
	counts := make(map[string]int)
	for _, card := range allCards {
		for _, t := range card.Types {
			t = strings.TrimSpace(t)
			if t != "" && t != "-" {
				counts[t]++
			}
		}
	}
	result := make([]TypeEntry, 0, len(counts))
	for name, count := range counts {
		result = append(result, TypeEntry{Name: name, Count: count})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].Name < result[j].Name
	})
	writeJSON(w, result)
}

// ── Meta ──────────────────────────────────────────────────────────────────────

// metaHandler returns all bootstrap data a client needs in one round-trip.
func metaHandler(w http.ResponseWriter, r *http.Request) {
	// Sets
	seenSets := make(map[string]bool)
	for _, card := range allCards {
		seenSets[cardSet(card.CardID)] = true
	}
	sets := make([]SetEntry, 0, len(seenSets))
	for code := range seenSets {
		sets = append(sets, SetEntry{
			Code:    code,
			Series:  seriesFromSetCode(code),
			Rotated: isRotated(code),
		})
	}
	sort.Slice(sets, func(i, j int) bool { return sets[i].Code < sets[j].Code })

	// Keywords
	seenKW := make(map[string]bool)
	for _, card := range allCards {
		for _, kw := range keywordRe.FindAllString(plainText(card.Text), -1) {
			seenKW[kw] = true
		}
		for _, kw := range keywordRe.FindAllString(card.Trigger, -1) {
			seenKW[kw] = true
		}
	}
	keywords := make([]string, 0, len(seenKW))
	for kw := range seenKW {
		keywords = append(keywords, kw)
	}
	sort.Strings(keywords)

	// Types
	counts := make(map[string]int)
	for _, card := range allCards {
		for _, t := range card.Types {
			t = strings.TrimSpace(t)
			if t != "" && t != "-" {
				counts[t]++
			}
		}
	}
	types := make([]TypeEntry, 0, len(counts))
	for name, count := range counts {
		types = append(types, TypeEntry{Name: name, Count: count})
	}
	sort.Slice(types, func(i, j int) bool {
		if types[i].Count != types[j].Count {
			return types[i].Count > types[j].Count
		}
		return types[i].Name < types[j].Name
	})

	costs := computeCosts()
	writeJSON(w, MetaResponse{Sets: sets, Keywords: keywords, Types: types, Costs: costs})
}

// ── Costs ─────────────────────────────────────────────────────────────────────

// computeCosts extracts and counts distinct effect-cost patterns across all cards.
// DON!! cost indicators (circled digits) are stripped so "① Rest this Character"
// and "② Rest this Character" both become "Rest this Character".
func computeCosts() []CostEntry {
	counts := make(map[string]int)
	for _, card := range allCards {
		plain := plainText(card.Text)
		stripped := keywordRe.ReplaceAllString(plain, "")
		parts := strings.Split(stripped, ":")
		for i := 0; i < len(parts)-1; i++ {
			seg := parts[i]
			if dot := strings.LastIndex(seg, "."); dot >= 0 {
				seg = seg[dot+1:]
			}
			// Strip DON!! cost indicators and surrounding parentheses/symbols
			// so "① Rest" and "(1) Rest" normalise to the same cost string.
			seg = donRe.ReplaceAllString(seg, "")
			seg = strings.Trim(strings.TrimSpace(seg), "()+-")
			if seg != "" && seg != "-" {
				counts[seg]++
			}
		}
	}
	result := make([]CostEntry, 0, len(counts))
	for text, count := range counts {
		result = append(result, CostEntry{Text: text, Count: count})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].Text < result[j].Text
	})
	return result
}

// costsHandler is the dedicated /api/costs endpoint.
func costsHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, computeCosts())
}

// ── Image proxy ───────────────────────────────────────────────────────────────

func imageProxyHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if !strings.HasPrefix(path, "/images/") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	resp, err := http.Get("https://en.onepiece-cardgame.com" + path)
	if err != nil || resp.StatusCode != 200 {
		http.NotFound(w, r)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Cache-Control", "max-age=86400")
	io.Copy(w, resp.Body)
}
