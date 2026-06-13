// Package cpalgorithms is the library behind the cpa command line:
// the HTTP client, request shaping, and the typed data models for
// CP-Algorithms (https://cp-algorithms.com).
//
// The Client fetches the navigation page and parses article links and titles
// using only the standard library. No key or account is required.
package cpalgorithms

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultUserAgent identifies the client to cp-algorithms.com.
const DefaultUserAgent = "cpa/dev (+https://github.com/tamnd/cpalgorithms-cli)"

// Config holds constructor parameters for Client.
type Config struct {
	// BaseURL is the root of the CP-Algorithms site. Override in tests.
	BaseURL   string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
	UserAgent string
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://cp-algorithms.com",
		Rate:      200 * time.Millisecond,
		Retries:   5,
		Timeout:   30 * time.Second,
		UserAgent: DefaultUserAgent,
	}
}

// Client talks to cp-algorithms.com over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	last time.Time
}

// NewClient returns a Client with the given config.
func NewClient(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultConfig().BaseURL
	}
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// Article is one article listed on cp-algorithms.com.
type Article struct {
	// Rank is the 1-based position in the navigation list.
	Rank     int    `json:"rank"`
	Title    string `json:"title"`
	Category string `json:"category"`
	URL      string `json:"url"`
	Path     string `json:"path"`
}

// List fetches the navigation page and returns all articles. If limit > 0 at
// most that many results are returned.
func (c *Client) List(ctx context.Context, limit int) ([]Article, error) {
	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/navigation.html"
	body, err := c.get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("fetch navigation: %w", err)
	}
	articles := parseArticles(string(body), c.cfg.BaseURL)
	if limit > 0 && limit < len(articles) {
		articles = articles[:limit]
	}
	return articles, nil
}

// Search returns articles whose title contains query (case-insensitive).
// If limit > 0 at most that many matches are returned.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Article, error) {
	all, err := c.List(ctx, 0)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var out []Article
	for _, a := range all {
		if strings.Contains(strings.ToLower(a.Title), q) ||
			strings.Contains(strings.ToLower(a.Category), q) {
			out = append(out, a)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// parseArticles parses the navigation HTML and returns Article records.
// It uses only stdlib string scanning — no external HTML parser.
// The MkDocs page renders every link twice (sidebar + main body), so we
// deduplicate by URL while preserving first-seen order.
func parseArticles(html, baseURL string) []Article {
	base := strings.TrimRight(baseURL, "/")

	// Skip prefixes that are not article paths.
	skipped := map[string]bool{
		"index.html":           true,
		"navigation.html":      true,
		"tags.html":            true,
		"contrib.html":         true,
		"code_of_conduct.html": true,
		"preview.html":         true,
	}

	seen := map[string]bool{}
	var articles []Article
	rank := 1

	// Scan for <a href="...">...</a> occurrences.
	rest := html
	for {
		// Find next <a
		aStart := strings.Index(rest, "<a ")
		if aStart < 0 {
			break
		}
		rest = rest[aStart:]

		// Find end of opening tag
		tagEnd := strings.Index(rest, ">")
		if tagEnd < 0 {
			break
		}
		openTag := rest[:tagEnd+1]

		// Find href="..."
		href := extractAttr(openTag, "href")
		if href == "" {
			rest = rest[tagEnd+1:]
			continue
		}

		// Skip non-article hrefs
		if strings.HasPrefix(href, "http") && !strings.HasPrefix(href, base) {
			rest = rest[tagEnd+1:]
			continue
		}
		if strings.HasPrefix(href, "#") ||
			strings.HasPrefix(href, "assets/") ||
			strings.HasPrefix(href, ".") ||
			strings.HasPrefix(href, "feed_rss") {
			rest = rest[tagEnd+1:]
			continue
		}

		// Normalise to a relative path
		path := href
		if strings.HasPrefix(path, base) {
			path = strings.TrimPrefix(path, base)
			path = strings.TrimPrefix(path, "/")
		}

		if skipped[path] {
			rest = rest[tagEnd+1:]
			continue
		}

		// The path must contain a slash (category/article.html)
		if !strings.Contains(path, "/") {
			rest = rest[tagEnd+1:]
			continue
		}

		// Find closing </a>
		rest = rest[tagEnd+1:]
		closeTag := strings.Index(rest, "</a>")
		if closeTag < 0 {
			continue
		}
		rawTitle := rest[:closeTag]
		title := strings.TrimSpace(stripTags(rawTitle))
		rest = rest[closeTag+4:]

		if title == "" {
			continue
		}

		fullURL := base + "/" + path
		if seen[fullURL] {
			continue
		}
		seen[fullURL] = true

		// Derive category from the first path segment.
		parts := strings.SplitN(path, "/", 2)
		category := categoryLabel(parts[0])

		articles = append(articles, Article{
			Rank:     rank,
			Title:    title,
			Category: category,
			URL:      fullURL,
			Path:     path,
		})
		rank++
	}
	return articles
}

// extractAttr returns the value of the named attribute from an HTML open tag string.
func extractAttr(tag, name string) string {
	needle := name + `="`
	i := strings.Index(strings.ToLower(tag), needle)
	if i < 0 {
		needle = name + `='`
		i = strings.Index(strings.ToLower(tag), needle)
		if i < 0 {
			return ""
		}
	}
	start := i + len(needle)
	quote := tag[i+len(name)+1]
	end := strings.IndexByte(tag[start:], quote)
	if end < 0 {
		return ""
	}
	return tag[start : start+end]
}

// stripTags removes HTML tags from s.
func stripTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	out := b.String()
	out = strings.ReplaceAll(out, "&amp;", "&")
	out = strings.ReplaceAll(out, "&lt;", "<")
	out = strings.ReplaceAll(out, "&gt;", ">")
	out = strings.ReplaceAll(out, "&quot;", `"`)
	out = strings.ReplaceAll(out, "&#39;", "'")
	out = strings.ReplaceAll(out, "&apos;", "'")
	return strings.TrimSpace(out)
}

// categoryLabel converts a URL path segment into a human-readable category.
func categoryLabel(seg string) string {
	labels := map[string]string{
		"algebra":          "Algebra",
		"combinatorics":    "Combinatorics",
		"data_structures":  "Data Structures",
		"dynamic_programming": "Dynamic Programming",
		"game_theory":      "Game Theory",
		"geometry":         "Geometry",
		"graph":            "Graph Theory",
		"linear_algebra":   "Linear Algebra",
		"num_methods":      "Numerical Methods",
		"others":           "Other",
		"schedules":        "Scheduling",
		"sequences":        "Sequences",
		"string":           "Strings",
	}
	if l, ok := labels[seg]; ok {
		return l
	}
	// Fallback: title-case the segment.
	return strings.Title(strings.ReplaceAll(seg, "_", " "))
}

// ─── HTTP helpers ─────────────────────────────────────────────────────────────

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, url string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
