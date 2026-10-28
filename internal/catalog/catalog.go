// Package catalog fetches, caches and searches the radio-browser.info station
// database.
package catalog

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"radiocli/internal/paths"
	"radiocli/internal/station"
)

// Repository is the searchable station database. It is fetched directly from
// the radio-browser.info API - the same source the Cinnamon Radio++ applet
// uses to build its allStations.json - and cached on disk so search still
// works offline.
type Repository struct {
	mu      sync.RWMutex
	entries []station.Station
	err     string
	path    string
}

// radioBrowserURLs mirror the endpoint the applet queries. Extra hosts are
// tried in order if the first one is unavailable.
var radioBrowserURLs = []string{
	"https://de1.api.radio-browser.info/json/stations?limit=100000&hidebroken=true",
	"https://de2.api.radio-browser.info/json/stations?limit=100000&hidebroken=true",
	"https://all.api.radio-browser.info/json/stations?limit=100000&hidebroken=true",
}

const userAgent = "radiocli/1.0"

// Load loads the cached station list. It never performs a network request;
// call Refresh to download the current list.
func Load() *Repository {
	db := &Repository{path: paths.CatalogPath()}

	data, err := os.ReadFile(db.path)
	if err != nil {
		return db
	}

	var raw [][]any
	if err := json.Unmarshal(data, &raw); err != nil {
		db.err = fmt.Sprintf("Failed to load cached station database: %v", err)
		return db
	}

	for _, item := range raw {
		if len(item) < 2 {
			continue
		}
		db.entries = append(db.entries, station.Station{Name: toStr(item[0]), URL: toStr(item[1])})
	}
	return db
}

// Refresh downloads the full station list, applies the same filtering and
// deduplication the applet does, then writes it to the cache.
func (db *Repository) Refresh() error {
	client := &http.Client{Timeout: 90 * time.Second}

	var lastErr error
	for _, url := range radioBrowserURLs {
		entries, err := fetchStations(client, url)
		if err != nil {
			lastErr = err
			continue
		}

		db.mu.Lock()
		db.entries = entries
		db.err = ""
		db.mu.Unlock()

		return db.save()
	}

	db.mu.Lock()
	db.err = fmt.Sprintf("Failed to fetch station database: %v", lastErr)
	db.mu.Unlock()

	return lastErr
}

func fetchStations(client *http.Client, url string) ([]station.Station, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}

	var raw []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	seen := make(map[station.Station]bool, len(raw))
	entries := make([]station.Station, 0, len(raw))

	for _, s := range raw {
		name := strings.TrimSpace(s.Name)
		url := strings.TrimSpace(s.URL)
		if name == "" || len(name) > 200 || len(url) > 200 {
			continue
		}

		entry := station.Station{Name: name, URL: url}
		if seen[entry] {
			continue
		}
		seen[entry] = true
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })

	return entries, nil
}

func (db *Repository) save() error {
	db.mu.RLock()
	raw := make([][2]string, len(db.entries))

	for i, s := range db.entries {
		raw[i] = [2]string{s.Name, s.URL}
	}
	db.mu.RUnlock()

	data, err := json.Marshal(raw)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(db.path), 0o755); err != nil {
		return err
	}

	tmp := db.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tmp, db.path)
}

func toStr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func (db *Repository) Count() int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return len(db.entries)
}

func (db *Repository) Available() bool {
	return db.Count() > 0
}

func (db *Repository) Search(query string, limit int) []station.Station {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	results := make([]station.Station, 0, limit)
	for _, entry := range db.entries {
		if strings.Contains(strings.ToLower(entry.Name), query) {
			results = append(results, entry)
			if len(results) >= limit {
				break
			}
		}
	}

	return results
}
