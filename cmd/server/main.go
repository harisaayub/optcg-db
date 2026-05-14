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
type CardResult struct {
	Card
	AltArts []string `json:"alt_arts"`
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

// MetaResponse is returned by /api/meta for client bootstrap.
type MetaResponse struct {
	Sets     []SetEntry  `json:"sets"`
	Keywords []string    `json:"keywords"`
	Types    []TypeEntry `json:"types"`
}

// ── Global state ──────────────────────────────────────────────────────────────

var cards []Card

var (
	htmlTagRe = regexp.MustCompile(`<[^>]+>`)
	keywordRe = regexp.MustCompile(`\[[^\]]+\]`)
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
	if err := json.Unmarshal(data, &cards); err != nil {
		log.Fatal(err)
	}
	log.Printf("Loaded %d cards", len(cards))

	mux := http.NewServeMux()

	// API routes — all CORS-enabled, all under /api/
	mux.HandleFunc("GET /api/search", withCORS(searchHandler))
	mux.HandleFunc("GET /api/leaders", withCORS(leadersHandler))
	mux.HandleFunc("GET /api/card/{id}", withCORS(cardHandler))
	mux.HandleFunc("GET /api/sets", withCORS(setsHandler))
	mux.HandleFunc("GET /api/keywords", withCORS(keywordsHandler))
	mux.HandleFunc("GET /api/types", withCORS(typesHandler))
	mux.HandleFunc("GET /api/meta", withCORS(metaHandler))
	// OPTIONS preflight catch-all for the /api/ subtree
	mux.HandleFunc("OPTIONS /api/", withCORS(func(w http.ResponseWriter, r *http.Request) {}))

	// Image proxy and static frontend — not versioned API
	mux.HandleFunc("GET /image", imageProxyHandler)
	mux.Handle("/", http.FileServer(http.Dir("static")))

	log.Println("Listening on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

// ── Search ────────────────────────────────────────────────────────────────────

func searchHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	colorsParam := r.URL.Query().Get("colors")
	typesParam := r.URL.Query().Get("types")
	keyword := r.URL.Query().Get("keyword")
	keywordExclude := r.URL.Query().Get("keyword_exclude")
	setsParam := r.URL.Query().Get("sets")
	seriesParam := r.URL.Query().Get("series")
	tagsIncludeParam := r.URL.Query().Get("tags_include")
	tagsExcludeParam := r.URL.Query().Get("tags_exclude")
	excludeRotated := r.URL.Query().Get("exclude_rotated") == "1"

	if q == "" && colorsParam == "" && typesParam == "" && keyword == "" &&
		keywordExclude == "" && setsParam == "" && seriesParam == "" &&
		tagsIncludeParam == "" && tagsExcludeParam == "" && !excludeRotated {
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

	filterColors := splitParam(colorsParam)
	filterTypes := splitParam(typesParam)
	filterSets := splitParam(setsParam)
	filterSeries := splitParam(seriesParam)
	filterTagsInclude := splitParam(tagsIncludeParam)
	filterTagsExclude := splitParam(tagsExcludeParam)

	grouped := make(map[string]*CardResult)
	order := make([]string, 0)

	for _, card := range cards {
		plain := plainText(card.Text)
		plainTrigger := plainText(card.Trigger)

		if _, seen := grouped[card.CardID]; !seen {
			if re != nil && !re.MatchString(plain) && !re.MatchString(plainTrigger) {
				continue
			}
			if filterColors != nil && !anyMatch(card.Colors, filterColors) {
				continue
			}
			if filterTypes != nil && !filterTypes[card.CardType] {
				continue
			}
			if keyword != "" && !strings.Contains(plain, keyword) && !strings.Contains(plainTrigger, keyword) {
				continue
			}
			if keywordExclude != "" && (strings.Contains(plain, keywordExclude) || strings.Contains(plainTrigger, keywordExclude)) {
				continue
			}
			if filterSets != nil && !filterSets[cardSet(card.CardID)] {
				continue
			}
			if filterSeries != nil && !filterSeries[cardSeries(card.CardID)] {
				continue
			}
			if excludeRotated && isRotated(cardSet(card.CardID)) {
				continue
			}
			if filterTagsInclude != nil && !anyMatch(card.Types, filterTagsInclude) {
				continue
			}
			if filterTagsExclude != nil && anyMatch(card.Types, filterTagsExclude) {
				continue
			}

			grouped[card.CardID] = &CardResult{Card: card, AltArts: []string{}}
			order = append(order, card.CardID)
		} else {
			grouped[card.CardID].AltArts = append(grouped[card.CardID].AltArts, card.ImageURL)
		}
	}

	results := make([]CardResult, 0, len(order))
	for _, id := range order {
		results = append(results, *grouped[id])
	}
	writeJSON(w, results)
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

// leadersHandler returns every unique leader card with alt arts bundled.
func leadersHandler(w http.ResponseWriter, r *http.Request) {
	grouped := make(map[string]*CardResult)
	order := make([]string, 0)
	for _, card := range cards {
		if card.CardType != "LEADER" {
			continue
		}
		if _, seen := grouped[card.CardID]; !seen {
			grouped[card.CardID] = &CardResult{Card: card, AltArts: []string{}}
			order = append(order, card.CardID)
		} else {
			grouped[card.CardID].AltArts = append(grouped[card.CardID].AltArts, card.ImageURL)
		}
	}
	results := make([]CardResult, 0, len(order))
	for _, id := range order {
		results = append(results, *grouped[id])
	}
	writeJSON(w, results)
}

// ── Single card ───────────────────────────────────────────────────────────────

// cardHandler returns a single card by ID with alt arts bundled.
func cardHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var result *CardResult
	for _, card := range cards {
		if card.CardID != id {
			continue
		}
		if result == nil {
			result = &CardResult{Card: card, AltArts: []string{}}
		} else {
			result.AltArts = append(result.AltArts, card.ImageURL)
		}
	}
	if result == nil {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, result)
}

// ── Sets ──────────────────────────────────────────────────────────────────────

// setsHandler returns all known sets with their series and rotation status.
func setsHandler(w http.ResponseWriter, r *http.Request) {
	seen := make(map[string]bool)
	for _, card := range cards {
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
	for _, card := range cards {
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
	seenCard := make(map[string]bool)
	for _, card := range cards {
		if seenCard[card.CardID] {
			continue
		}
		seenCard[card.CardID] = true
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
	for _, card := range cards {
		seenSets[cardSet(card.CardID)] = true
	}
	sets := make([]SetEntry, 0, len(seenSets))
	for code := range seenSets {
		sets = append(sets, SetEntry{
			Code:    code,
			Series:  cardSeries(code + "-"),
			Rotated: isRotated(code),
		})
	}
	sort.Slice(sets, func(i, j int) bool { return sets[i].Code < sets[j].Code })

	// Keywords
	seenKW := make(map[string]bool)
	for _, card := range cards {
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
	seenCard := make(map[string]bool)
	for _, card := range cards {
		if seenCard[card.CardID] {
			continue
		}
		seenCard[card.CardID] = true
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

	writeJSON(w, MetaResponse{Sets: sets, Keywords: keywords, Types: types})
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
