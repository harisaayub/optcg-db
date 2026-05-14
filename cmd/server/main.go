package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
)

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

// CardResult is what the API returns: one entry per card_id, with alt art URLs bundled in.
type CardResult struct {
	Card
	AltArts []string `json:"alt_arts"`
}

// TypeEntry is returned by /types: archetype name + how many unique cards carry it.
type TypeEntry struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

var cards []Card

var (
	htmlTagRe = regexp.MustCompile(`<[^>]+>`)
	keywordRe = regexp.MustCompile(`\[[^\]]+\]`)
)

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
	mux.HandleFunc("GET /search", searchHandler)
	mux.HandleFunc("GET /keywords", keywordsHandler)
	mux.HandleFunc("GET /sets", setsHandler)
	mux.HandleFunc("GET /types", typesHandler)
	mux.HandleFunc("GET /image", imageProxyHandler)
	mux.Handle("/", http.FileServer(http.Dir("static")))

	log.Println("Listening on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

// plainText strips HTML tags so regex and keyword matching run on readable text.
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

// cardSeries strips trailing digits from the set code (e.g. "OP" from "OP05", "PRB" from "PRB01").
func cardSeries(id string) string {
	s := cardSet(id)
	i := len(s)
	for i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
		i--
	}
	return s[:i]
}

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

	if q == "" && colorsParam == "" && typesParam == "" && keyword == "" && keywordExclude == "" && setsParam == "" && seriesParam == "" && tagsIncludeParam == "" && tagsExcludeParam == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]CardResult{})
		return
	}

	var re *regexp.Regexp
	if q != "" {
		var err error
		re, err = regexp.Compile("(?i)" + q)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid regex: " + err.Error()})
			return
		}
	}

	var filterColors map[string]bool
	if colorsParam != "" {
		filterColors = make(map[string]bool)
		for _, c := range strings.Split(colorsParam, ",") {
			filterColors[strings.TrimSpace(c)] = true
		}
	}

	var filterTypes map[string]bool
	if typesParam != "" {
		filterTypes = make(map[string]bool)
		for _, t := range strings.Split(typesParam, ",") {
			filterTypes[strings.TrimSpace(t)] = true
		}
	}

	var filterSets map[string]bool
	if setsParam != "" {
		filterSets = make(map[string]bool)
		for _, s := range strings.Split(setsParam, ",") {
			filterSets[strings.TrimSpace(s)] = true
		}
	}

	var filterSeries map[string]bool
	if seriesParam != "" {
		filterSeries = make(map[string]bool)
		for _, s := range strings.Split(seriesParam, ",") {
			filterSeries[strings.TrimSpace(s)] = true
		}
	}

	var filterTagsInclude map[string]bool
	if tagsIncludeParam != "" {
		filterTagsInclude = make(map[string]bool)
		for _, t := range strings.Split(tagsIncludeParam, ",") {
			filterTagsInclude[strings.TrimSpace(t)] = true
		}
	}

	var filterTagsExclude map[string]bool
	if tagsExcludeParam != "" {
		filterTagsExclude = make(map[string]bool)
		for _, t := range strings.Split(tagsExcludeParam, ",") {
			filterTagsExclude[strings.TrimSpace(t)] = true
		}
	}

	// Group by card_id as we filter — first occurrence wins for card data,
	// subsequent occurrences are alt arts.
	grouped := make(map[string]*CardResult)
	order := make([]string, 0)

	for _, card := range cards {
		plain := plainText(card.Text)
		plainTrigger := plainText(card.Trigger)

		// Only run filters against the first (canonical) copy of each card.
		if _, seen := grouped[card.CardID]; !seen {
			if re != nil && !re.MatchString(plain) && !re.MatchString(plainTrigger) {
				continue
			}
			if filterColors != nil {
				match := false
				for _, c := range card.Colors {
					if filterColors[c] {
						match = true
						break
					}
				}
				if !match {
					continue
				}
			}
			if filterTypes != nil && !filterTypes[card.CardType] {
				continue
			}
			// Keyword filters are exact substring matches against plain text — no regex.
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
			if filterTagsInclude != nil {
				match := false
				for _, t := range card.Types {
					if filterTagsInclude[t] {
						match = true
						break
					}
				}
				if !match {
					continue
				}
			}
			if filterTagsExclude != nil {
				excluded := false
				for _, t := range card.Types {
					if filterTagsExclude[t] {
						excluded = true
						break
					}
				}
				if excluded {
					continue
				}
			}

			grouped[card.CardID] = &CardResult{Card: card, AltArts: []string{}}
			order = append(order, card.CardID)
		} else {
			// This is an alt art of a card that already passed filters.
			grouped[card.CardID].AltArts = append(grouped[card.CardID].AltArts, card.ImageURL)
		}
	}

	results := make([]CardResult, 0, len(order))
	for _, id := range order {
		results = append(results, *grouped[id])
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func setsHandler(w http.ResponseWriter, r *http.Request) {
	seen := make(map[string]bool)
	for _, card := range cards {
		seen[cardSet(card.CardID)] = true
	}
	result := make([]string, 0, len(seen))
	for s := range seen {
		result = append(result, s)
	}
	sort.Strings(result)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(keywords)
}

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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

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
