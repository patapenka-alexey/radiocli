// Package catalog fetches, caches and searches the radio-browser.info station
// database.
package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"radiocli/internal/paths"
	"radiocli/internal/ports"
	"radiocli/internal/station"
)

var _ ports.Catalog = (*Repository)(nil)

// ErrFetch is returned when every mirror failed.
var ErrFetch = errors.New("fetch station database")

// ErrCache is returned when the fetched database could not be written to disk.
var ErrCache = errors.New("cache station database")

// HTTPDoer is the subset of *http.Client used by the catalog.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// RetryPolicy controls mirror failover. Attempts is the number of full passes
// over the mirror list.
type RetryPolicy struct {
	Attempts  int
	BaseDelay time.Duration
	MaxDelay  time.Duration
}

// Options configures a Repository.
type Options struct {
	Path      string
	Mirrors   []string
	Client    HTTPDoer
	Retry     RetryPolicy
	UserAgent string
}

// StatusError is returned for non-2xx responses.
type StatusError struct {
	Status string
}

func (e *StatusError) Error() string {
	return "unexpected status " + e.Status
}

// DecodeError is returned when the station list cannot be decoded.
type DecodeError struct {
	Err error
}

func (e *DecodeError) Error() string {
	return "decode station database: " + e.Err.Error()
}

func (e *DecodeError) Unwrap() error {
	return e.Err
}

// FetchError ties a network or protocol error to the mirror that produced it.
type FetchError struct {
	URL string
	Err error
}

func (e *FetchError) Error() string {
	return fmt.Sprintf("fetch %s: %v", e.URL, e.Err)
}

func (e *FetchError) Unwrap() error {
	return e.Err
}

var defaultMirrors = []string{
	"https://de1.api.radio-browser.info/json/stations?limit=100000&hidebroken=true",
	"https://de2.api.radio-browser.info/json/stations?limit=100000&hidebroken=true",
	"https://all.api.radio-browser.info/json/stations?limit=100000&hidebroken=true",
}

var defaultRetryPolicy = RetryPolicy{
	Attempts:  2,
	BaseDelay: 250 * time.Millisecond,
	MaxDelay:  2 * time.Second,
}

const defaultUserAgent = "radiocli/1.0"

// Repository is the searchable station database. It is fetched directly from
// the radio-browser.info API - the same source the Cinnamon Radio++ applet
// uses to build its allStations.json - and cached on disk so search still
// works offline.
type Repository struct {
	mu      sync.RWMutex
	entries []station.Station
	err     string
	path    string

	mirrors   []string
	client    HTTPDoer
	retry     RetryPolicy
	userAgent string
	sleep     func(time.Duration)
}

// New builds a repository with the given options. Zero values fall back to the
// production defaults.
func New(opts Options) *Repository {
	if opts.Path == "" {
		opts.Path = paths.CatalogPath()
	}
	if len(opts.Mirrors) == 0 {
		opts.Mirrors = defaultMirrors
	}
	if opts.Client == nil {
		opts.Client = &http.Client{Timeout: 90 * time.Second}
	}
	if opts.UserAgent == "" {
		opts.UserAgent = defaultUserAgent
	}
	if opts.Retry.Attempts <= 0 {
		opts.Retry.Attempts = defaultRetryPolicy.Attempts
	}
	if opts.Retry.BaseDelay < 0 {
		opts.Retry.BaseDelay = 0
	}
	if opts.Retry.MaxDelay <= 0 {
		opts.Retry.MaxDelay = defaultRetryPolicy.MaxDelay
	}

	return &Repository{
		path:      opts.Path,
		mirrors:   opts.Mirrors,
		client:    opts.Client,
		retry:     opts.Retry,
		userAgent: opts.UserAgent,
		sleep:     time.Sleep,
	}
}

// Load loads the cached station list. It never performs a network request;
// call Refresh to download the current list.
func Load() *Repository {
	db := New(Options{})
	db.loadCache()

	return db
}

func (db *Repository) loadCache() {
	data, err := os.ReadFile(db.path)
	if err != nil {
		return
	}

	var raw [][]any
	if err := json.Unmarshal(data, &raw); err != nil {
		db.err = fmt.Sprintf("Failed to load cached station database: %v", err)

		return
	}

	for _, item := range raw {
		if len(item) < 2 {
			continue
		}
		db.entries = append(db.entries, station.Station{Name: toStr(item[0]), URL: toStr(item[1])})
	}
}

// Refresh downloads the full station list, applies the same filtering and
// deduplication the applet does, then writes it to the cache.
func (db *Repository) Refresh() error {
	var lastErr error
	attempts := 0

	for round := 0; round < db.retry.Attempts; round++ {
		for _, url := range db.mirrors {
			if attempts > 0 {
				db.sleep(db.backoff(attempts))
			}
			attempts++

			entries, err := db.fetch(url)
			if err != nil {
				lastErr = err
				continue
			}

			db.mu.Lock()
			db.entries = entries
			db.err = ""
			db.mu.Unlock()

			if err := db.save(); err != nil {
				return fmt.Errorf("%w: %w", ErrCache, err)
			}

			return nil
		}
	}

	if lastErr == nil {
		lastErr = errors.New("no mirrors configured")
	}

	err := fmt.Errorf("%w: %w", ErrFetch, lastErr)

	db.mu.Lock()
	db.err = err.Error()
	db.mu.Unlock()

	return err
}

func (db *Repository) backoff(attempt int) time.Duration {
	delay := db.retry.BaseDelay

	for i := 1; i < attempt && delay < db.retry.MaxDelay; i++ {
		delay *= 2
	}

	if delay > db.retry.MaxDelay {
		delay = db.retry.MaxDelay
	}

	if delay <= 0 {
		return 0
	}

	half := delay / 2

	return half + time.Duration(rand.Int64N(int64(half)+1))
}

func (db *Repository) fetch(url string) ([]station.Station, error) {
	entries, err := db.doFetch(url)
	if err != nil {
		return nil, &FetchError{URL: url, Err: err}
	}

	return entries, nil
}

func (db *Repository) doFetch(url string) ([]station.Station, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", db.userAgent)

	resp, err := db.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &StatusError{Status: resp.Status}
	}

	var raw []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, &DecodeError{Err: err}
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

// Count returns the number of cached stations.
func (db *Repository) Count() int {
	db.mu.RLock()
	defer db.mu.RUnlock()

	return len(db.entries)
}

// Available reports whether any stations are cached.
func (db *Repository) Available() bool {
	return db.Count() > 0
}

// Search returns stations whose name contains the query, up to limit.
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
