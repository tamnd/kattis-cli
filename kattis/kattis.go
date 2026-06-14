// Package kattis is the library behind the kattis command line:
// the HTTP client, request shaping, and typed data models for
// the Kattis Online Judge (https://open.kattis.com/).
//
// The Client fetches the paginated problem list and parses problems from the
// HTML table using only the standard library. No account or API key is needed.
package kattis

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultUserAgent identifies the client to open.kattis.com.
const DefaultUserAgent = "kattis/dev (+https://github.com/tamnd/kattis-cli)"

// Config holds constructor parameters for Client.
type Config struct {
	// BaseURL is the root of the Kattis site. Override in tests.
	BaseURL   string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
	UserAgent string
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://open.kattis.com",
		Rate:      300 * time.Millisecond,
		Retries:   5,
		Timeout:   30 * time.Second,
		UserAgent: DefaultUserAgent,
	}
}

// Client talks to open.kattis.com over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	last time.Time
}

// NewClient returns a Client configured from cfg.
func NewClient(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultConfig().BaseURL
	}
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: timeout},
	}
}

// Problem is one problem from the Kattis problem list.
type Problem struct {
	// ID is the slug from the URL path, e.g. "helloworld".
	ID         string `json:"id"`
	Name       string `json:"name"`
	Difficulty string `json:"difficulty"`
	Category   string `json:"category"`
	Solved     string `json:"solved"`
	URL        string `json:"url"`
}

// Problems fetches pages of /problems and returns up to limit problems.
// When limit is 0 it fetches the first page only (up to ~100 problems).
// When limit > 0 it fetches as many pages as needed to satisfy the limit.
// sort controls the order column: "difficulty", "name", "-difficulty", "-name", etc.
// An empty sort defaults to alphabetical by name (title_link asc).
func (c *Client) Problems(ctx context.Context, sort string, limit int) ([]Problem, error) {
	base := strings.TrimRight(c.cfg.BaseURL, "/")
	orderParam := sortToOrder(sort)
	var out []Problem
	for page := 0; ; page++ {
		url := fmt.Sprintf("%s/problems?page=%d&order=%s", base, page, orderParam)
		body, err := c.get(ctx, url)
		if err != nil {
			return nil, fmt.Errorf("fetch problems page %d: %w", page, err)
		}
		rows := parseProblems(string(body), base)
		if len(rows) == 0 {
			break
		}
		out = append(out, rows...)
		if limit > 0 && len(out) >= limit {
			out = out[:limit]
			break
		}
		// Only fetch page 0 when no limit is set (polite default).
		if limit == 0 {
			break
		}
	}
	return out, nil
}

// sortToOrder translates a user-facing sort token to a Kattis order parameter.
// Accepted tokens: "name", "-name", "difficulty", "-difficulty".
// Defaults to title_link (name ascending) for unknown/empty values.
func sortToOrder(sort string) string {
	switch strings.ToLower(sort) {
	case "difficulty":
		return "difficulty_data"
	case "-difficulty":
		return "-difficulty_data"
	case "-name":
		return "-title_link"
	default: // "name" or anything else
		return "title_link"
	}
}

// Search returns problems whose name or ID contains query (case-insensitive).
// The Kattis problem list is paginated; Search walks pages until limit results
// are found or all pages are exhausted.
// When limit is 0 it scans the first page only.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Problem, error) {
	base := strings.TrimRight(c.cfg.BaseURL, "/")
	q := strings.ToLower(query)
	var out []Problem
	for page := 0; ; page++ {
		url := fmt.Sprintf("%s/problems?page=%d&order=title_link", base, page)
		body, err := c.get(ctx, url)
		if err != nil {
			return nil, fmt.Errorf("fetch search page %d: %w", page, err)
		}
		rows := parseProblems(string(body), base)
		if len(rows) == 0 {
			break
		}
		for _, p := range rows {
			if strings.Contains(strings.ToLower(p.Name), q) ||
				strings.Contains(strings.ToLower(p.ID), q) {
				out = append(out, p)
				if limit > 0 && len(out) >= limit {
					return out, nil
				}
			}
		}
		// Stop after first page when no limit set.
		if limit == 0 {
			break
		}
	}
	return out, nil
}

// parseProblems parses one Kattis problem-list page and returns all problems.
// It uses only stdlib string scanning — no external HTML parser.
//
// Each row in the table looks like:
//
//	<tr class="" >
//	  <td class="  "><a href="/problems/helloworld"  >Hello World!</a></td>
//	  <td ...>0.00</td>          <!-- fastest -->
//	  <td ...>28</td>            <!-- shortest -->
//	  <td ...>4317</td>          <!-- total submissions -->
//	  <td ...>3442</td>          <!-- accepted submissions -->
//	  <td ...>80%</td>           <!-- acceptance rate = solved proxy -->
//	  <td ...><span class="... difficulty_easy">1.7 - 1.9</span>Easy</td>
//	  ...
//	</tr>
func parseProblems(html, baseURL string) []Problem {
	base := strings.TrimRight(baseURL, "/")
	var problems []Problem

	rest := html
	for {
		// Find the next <tr
		trIdx := strings.Index(rest, "<tr ")
		if trIdx < 0 {
			break
		}
		// Find end of row
		endIdx := strings.Index(rest[trIdx:], "</tr>")
		if endIdx < 0 {
			break
		}
		row := rest[trIdx : trIdx+endIdx+5]
		rest = rest[trIdx+endIdx+5:]

		// Only process rows that contain /problems/ links
		if !strings.Contains(row, `href="/problems/`) {
			continue
		}

		// Extract the first <a href="/problems/SLUG"> in the row
		needle := `href="/problems/`
		hrefIdx := strings.Index(row, needle)
		if hrefIdx < 0 {
			continue
		}
		afterHref := row[hrefIdx+len(needle):]
		// Find end of slug (next " character)
		slashOrQuote := strings.IndexByte(afterHref, '"')
		if slashOrQuote < 0 {
			continue
		}
		slug := afterHref[:slashOrQuote]
		// Skip sub-paths like /problems/helloworld/statistics or /problems/helloworld/en
		if strings.ContainsRune(slug, '/') {
			continue
		}

		// Extract problem name (text inside the <a>…</a>)
		aOpenEnd := strings.Index(row[hrefIdx:], ">")
		if aOpenEnd < 0 {
			continue
		}
		afterOpenTag := row[hrefIdx+aOpenEnd+1:]
		closeA := strings.Index(afterOpenTag, "</a>")
		if closeA < 0 {
			continue
		}
		name := strings.TrimSpace(stripTags(afterOpenTag[:closeA]))

		// Extract the difficulty span: class contains "difficulty_easy/medium/hard/..."
		// and the difficulty number/range is inside the <span>, category text follows.
		diffNumber := ""
		diffCategory := ""
		diffSpanNeedle := `class="whitespace-nowrap difficulty_number`
		dIdx := strings.Index(row, diffSpanNeedle)
		if dIdx >= 0 {
			spanClose := strings.Index(row[dIdx:], ">")
			if spanClose >= 0 {
				spanTag := row[dIdx : dIdx+spanClose+1]
				// Extract category from class name: difficulty_easy / difficulty_medium / difficulty_hard
				for _, cls := range []string{"difficulty_easy", "difficulty_medium", "difficulty_hard", "difficulty_very_hard"} {
					if strings.Contains(spanTag, cls) {
						switch cls {
						case "difficulty_easy":
							diffCategory = "Easy"
						case "difficulty_medium":
							diffCategory = "Medium"
						case "difficulty_hard":
							diffCategory = "Hard"
						case "difficulty_very_hard":
							diffCategory = "Very Hard"
						}
						break
					}
				}
				// Number is the text inside the span
				afterSpanOpen := row[dIdx+spanClose+1:]
				endSpan := strings.Index(afterSpanOpen, "</span>")
				if endSpan >= 0 {
					diffNumber = strings.TrimSpace(stripTags(afterSpanOpen[:endSpan]))
				}
			}
		}

		// Extract acceptance rate (6th <td>: fastest, shortest, total, accepted, rate)
		solved := extractNthTD(row, 5) // 0-indexed: 0=name,1=fastest,2=shortest,3=total,4=accepted,5=rate

		if name == "" || slug == "" {
			continue
		}

		diff := diffNumber
		if diffCategory != "" {
			if diff != "" {
				diff = diff + " " + diffCategory
			} else {
				diff = diffCategory
			}
		}

		problems = append(problems, Problem{
			ID:         slug,
			Name:       name,
			Difficulty: diff,
			Category:   diffCategory,
			Solved:     solved,
			URL:        base + "/problems/" + slug,
		})
	}
	return problems
}

// extractNthTD returns the text content of the nth <td> element (0-indexed).
func extractNthTD(row string, n int) string {
	rest := row
	for i := 0; ; i++ {
		tdIdx := strings.Index(rest, "<td")
		if tdIdx < 0 {
			return ""
		}
		// Skip to end of opening tag
		openEnd := strings.Index(rest[tdIdx:], ">")
		if openEnd < 0 {
			return ""
		}
		content := rest[tdIdx+openEnd+1:]
		closeIdx := strings.Index(content, "</td>")
		if closeIdx < 0 {
			return ""
		}
		if i == n {
			return strings.TrimSpace(stripTags(content[:closeIdx]))
		}
		rest = content[closeIdx+5:]
	}
}

// stripTags removes HTML tags from s and decodes common entities.
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
	out = strings.ReplaceAll(out, "&#039;", "'")
	out = strings.ReplaceAll(out, "&apos;", "'")
	return strings.TrimSpace(out)
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
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
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
