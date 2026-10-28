package catalog

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"radiocli/internal/station"
)

func newTestRepo(t *testing.T, opts Options) *Repository {
	t.Helper()

	if opts.Path == "" {
		opts.Path = filepath.Join(t.TempDir(), "stations.json")
	}

	repo := New(opts)
	repo.sleep = func(time.Duration) {}

	return repo
}

func TestRefreshFallsBackToSecondMirror(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bad":
			http.Error(w, "boom", http.StatusInternalServerError)
		case "/good":
			fmt.Fprint(w, `[{"name":"B","url":"u2"},{"name":"A","url":"u1"}]`)
		}
	}))
	defer server.Close()

	repo := newTestRepo(t, Options{
		Mirrors: []string{server.URL + "/bad", server.URL + "/good"},
		Retry:   RetryPolicy{Attempts: 1},
	})

	if err := repo.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if got := repo.Count(); got != 2 {
		t.Fatalf("Count() = %d, want 2", got)
	}
	if repo.entries[0].Name != "A" || repo.entries[1].Name != "B" {
		t.Fatalf("entries not sorted: %+v", repo.entries)
	}

	reloaded := New(Options{Path: repo.path})
	reloaded.loadCache()
	if got := reloaded.Count(); got != 2 {
		t.Fatalf("cached Count() = %d, want 2", got)
	}
}

func TestRefreshTriesEveryMirrorOnEveryRound(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "boom", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	var sleeps []time.Duration
	repo := New(Options{
		Path:    filepath.Join(t.TempDir(), "stations.json"),
		Mirrors: []string{server.URL + "/one", server.URL + "/two"},
		Retry:   RetryPolicy{Attempts: 2, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond},
		Client:  server.Client(),
	})
	repo.sleep = func(d time.Duration) { sleeps = append(sleeps, d) }

	err := repo.Refresh()

	if !errors.Is(err, ErrFetch) {
		t.Fatalf("error = %v, want ErrFetch", err)
	}
	var fetchErr *FetchError
	if !errors.As(err, &fetchErr) {
		t.Fatalf("error = %v, want FetchError", err)
	}
	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("error = %v, want StatusError", err)
	}
	if got := calls.Load(); got != 4 {
		t.Fatalf("calls = %d, want 4", got)
	}
	if len(sleeps) != 3 {
		t.Fatalf("sleeps = %d, want 3", len(sleeps))
	}
	if repo.Count() != 0 {
		t.Fatalf("Count() = %d, want 0", repo.Count())
	}
}

func TestRefreshDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not json")
	}))
	defer server.Close()

	repo := newTestRepo(t, Options{
		Mirrors: []string{server.URL},
		Retry:   RetryPolicy{Attempts: 1},
	})

	err := repo.Refresh()

	if !errors.Is(err, ErrFetch) {
		t.Fatalf("error = %v, want ErrFetch", err)
	}
	var decodeErr *DecodeError
	if !errors.As(err, &decodeErr) {
		t.Fatalf("error = %v, want DecodeError", err)
	}
}

func TestRefreshCacheWriteFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"name":"A","url":"u1"}]`)
	}))
	defer server.Close()

	repo := newTestRepo(t, Options{
		Path:    t.TempDir(),
		Mirrors: []string{server.URL},
		Retry:   RetryPolicy{Attempts: 1},
	})

	err := repo.Refresh()

	if !errors.Is(err, ErrCache) {
		t.Fatalf("error = %v, want ErrCache", err)
	}
	if got := repo.Count(); got != 1 {
		t.Fatalf("Count() = %d, want 1 after successful fetch", got)
	}
}

func TestRefreshFiltersDeduplicatesAndSorts(t *testing.T) {
	body := `[
		{"name":" B ","url":"u2"},
		{"name":"A","url":"u1"},
		{"name":"B","url":"u2"},
		{"name":"","url":"u3"},
		{"name":"Long","url":"` + strings.Repeat("x", 201) + `"}
	]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	}))
	defer server.Close()

	repo := newTestRepo(t, Options{
		Mirrors: []string{server.URL},
		Retry:   RetryPolicy{Attempts: 1},
	})

	if err := repo.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if got := repo.Count(); got != 2 {
		t.Fatalf("Count() = %d, want 2: %+v", got, repo.entries)
	}
	if repo.entries[0] != (station.Station{Name: "A", URL: "u1"}) ||
		repo.entries[1] != (station.Station{Name: "B", URL: "u2"}) {
		t.Fatalf("entries = %+v", repo.entries)
	}
}

func TestLoadReadsCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "allStations.json")
	t.Setenv("RADIOCLI_DB", path)

	if err := os.WriteFile(path, []byte(`[["B","u2"],["A","u1"]]`), 0o644); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	repo := Load()

	if got := repo.Count(); got != 2 {
		t.Fatalf("Count() = %d, want 2", got)
	}
	if got := repo.Search("a", 10); len(got) != 1 || got[0].Name != "A" {
		t.Fatalf("Search results = %+v", got)
	}
}

func TestBackoffBounds(t *testing.T) {
	repo := New(Options{Retry: RetryPolicy{Attempts: 1, BaseDelay: 100 * time.Millisecond, MaxDelay: 150 * time.Millisecond}})

	for attempt := 1; attempt <= 5; attempt++ {
		delay := repo.backoff(attempt)
		if delay < 50*time.Millisecond || delay > 150*time.Millisecond {
			t.Fatalf("attempt %d: delay = %v, want within [50ms, 150ms]", attempt, delay)
		}
	}
}
