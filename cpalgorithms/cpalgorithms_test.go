package cpalgorithms_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tamnd/cpalgorithms-cli/cpalgorithms"
)

// minimalNav is a trimmed snippet of the real navigation page — enough to
// exercise the parser without downloading the real site.
const minimalNav = `<!doctype html>
<html>
<body>
<nav>
  <a href="index.html" class="md-nav__link">Main Page</a>
  <a href="navigation.html" class="md-nav__link md-nav__link--active">Navigation</a>
  <a href="tags.html" class="md-nav__link">Tag index</a>
  <a href="contrib.html" class="md-nav__link">How to Contribute</a>
  <a href="algebra/binary-exp.html" class="md-nav__link">Binary Exponentiation</a>
  <a href="algebra/euclid-algorithm.html" class="md-nav__link">Euclidean algorithm for computing the greatest common divisor</a>
  <a href="data_structures/segment_tree.html" class="md-nav__link">Segment Tree</a>
  <a href="graph/dijkstra.html" class="md-nav__link">Dijkstra Algorithm</a>
  <a href="string/suffix-array.html" class="md-nav__link">Suffix Array</a>
</nav>
</body>
</html>`

func newTestServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}))
}

func TestListParsesArticles(t *testing.T) {
	srv := newTestServer(t, minimalNav)
	defer srv.Close()

	cfg := cpalgorithms.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	c := cpalgorithms.NewClient(cfg)

	articles, err := c.List(context.Background(), 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// nav + contrib are skipped; expect 5 articles
	if len(articles) != 5 {
		t.Errorf("got %d articles, want 5", len(articles))
		for _, a := range articles {
			t.Logf("  %d %s %s", a.Rank, a.Category, a.Title)
		}
	}

	first := articles[0]
	if first.Title != "Binary Exponentiation" {
		t.Errorf("first title = %q, want %q", first.Title, "Binary Exponentiation")
	}
	if first.Category != "Algebra" {
		t.Errorf("first category = %q, want %q", first.Category, "Algebra")
	}
	if !strings.Contains(first.URL, "/algebra/binary-exp.html") {
		t.Errorf("first URL = %q does not contain expected path", first.URL)
	}
	if first.Rank != 1 {
		t.Errorf("first rank = %d, want 1", first.Rank)
	}
}

func TestListLimit(t *testing.T) {
	srv := newTestServer(t, minimalNav)
	defer srv.Close()

	cfg := cpalgorithms.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	c := cpalgorithms.NewClient(cfg)

	articles, err := c.List(context.Background(), 2)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(articles) != 2 {
		t.Errorf("got %d articles with limit=2, want 2", len(articles))
	}
}

func TestSearchFilters(t *testing.T) {
	srv := newTestServer(t, minimalNav)
	defer srv.Close()

	cfg := cpalgorithms.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	c := cpalgorithms.NewClient(cfg)

	hits, err := c.Search(context.Background(), "segment", 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Errorf("got %d hits for 'segment', want 1", len(hits))
	}
	if len(hits) > 0 && hits[0].Title != "Segment Tree" {
		t.Errorf("hit title = %q, want %q", hits[0].Title, "Segment Tree")
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	srv := newTestServer(t, minimalNav)
	defer srv.Close()

	cfg := cpalgorithms.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	c := cpalgorithms.NewClient(cfg)

	hits, err := c.Search(context.Background(), "EUCLID", 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Errorf("got %d hits for 'EUCLID', want 1", len(hits))
	}
}

func TestSearchByCategory(t *testing.T) {
	srv := newTestServer(t, minimalNav)
	defer srv.Close()

	cfg := cpalgorithms.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	c := cpalgorithms.NewClient(cfg)

	hits, err := c.Search(context.Background(), "algebra", 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 2 {
		t.Errorf("got %d hits for 'algebra', want 2 (algebra articles)", len(hits))
	}
}

func TestGetSendsUserAgent(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Header.Get("User-Agent") == "" {
			t.Error("no User-Agent header")
		}
		_, _ = w.Write([]byte(minimalNav))
	}))
	defer srv.Close()

	cfg := cpalgorithms.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	c := cpalgorithms.NewClient(cfg)

	_, err := c.List(context.Background(), 1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !called {
		t.Error("handler never called")
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(minimalNav))
	}))
	defer srv.Close()

	cfg := cpalgorithms.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := cpalgorithms.NewClient(cfg)

	_, err := c.List(context.Background(), 0)
	if err != nil {
		t.Fatalf("List after retries: %v", err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}

// TestListLive hits the real site and is skipped in short mode.
func TestListLive(t *testing.T) {
	if testing.Short() {
		t.Skip("live: skipped in short mode")
	}
	c := cpalgorithms.NewClient(cpalgorithms.DefaultConfig())
	articles, err := c.List(context.Background(), 10)
	if err != nil {
		t.Fatalf("List live: %v", err)
	}
	if len(articles) == 0 {
		t.Fatal("got 0 articles from live site")
	}
	t.Logf("live: got %d articles (capped at 10), first: %s", len(articles), articles[0].Title)
}
