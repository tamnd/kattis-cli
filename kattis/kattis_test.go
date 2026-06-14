package kattis_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tamnd/kattis-cli/kattis"
)

// minimalPage builds a minimal Kattis-like problem-list HTML page with the
// given problems for use in httptest servers.
func minimalPage(problems []struct{ id, name, difficulty string }) string {
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html><html><body>`)
	sb.WriteString(`<section data-cy="problems-table">`)
	sb.WriteString(`<table class="table2 "><thead><tr><th>Name</th><th>Fastest</th><th>Shortest</th><th>Total</th><th>Acc</th><th>Ratio</th><th>Difficulty</th></tr></thead><tbody>`)
	for _, p := range problems {
		diffClass := "difficulty_easy"
		diffLabel := "Easy"
		switch p.difficulty {
		case "Medium":
			diffClass = "difficulty_medium"
			diffLabel = "Medium"
		case "Hard":
			diffClass = "difficulty_hard"
			diffLabel = "Hard"
		}
		fmt.Fprintf(&sb,
			`<tr class="" ><td class="  "><a href="/problems/%s"  >%s</a></td>`+
				`<td class="  ">0.00</td><td class="  ">100</td><td class="  ">1000</td><td class="  ">800</td><td class="  ">80%%</td>`+
				`<td class="  "><span class="whitespace-nowrap difficulty_number difficulty_number-problems_table %s">1.0</span>%s</td>`+
				`<td class="  "><span class="bubble-container"><a class="bubble" href="/problems/%s/en">en</a></span></td>`+
				`<td class="table-item-autofit"><a href="/problems/%s/statistics">stats</a></td></tr>`,
			p.id, p.name, diffClass, diffLabel, p.id, p.id)
	}
	sb.WriteString(`</tbody></table></section></body></html>`)
	return sb.String()
}

func TestProblems(t *testing.T) {
	page := minimalPage([]struct{ id, name, difficulty string }{
		{"helloworld", "Hello World!", "Easy"},
		{"aplusb", "A+B", "Easy"},
		{"sortingtest", "Sorting Test", "Medium"},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/problems" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, page)
	}))
	defer srv.Close()

	cfg := kattis.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	client := kattis.NewClient(cfg)

	problems, err := client.Problems(context.Background(), "", 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(problems) != 3 {
		t.Errorf("got %d problems, want 3", len(problems))
	}

	names := make(map[string]bool, len(problems))
	for _, p := range problems {
		names[p.Name] = true
		if p.URL == "" {
			t.Errorf("problem %q has empty URL", p.Name)
		}
		if p.ID == "" {
			t.Errorf("problem %q has empty ID", p.Name)
		}
	}
	for _, want := range []string{"Hello World!", "A+B", "Sorting Test"} {
		if !names[want] {
			t.Errorf("missing problem %q in results", want)
		}
	}
}

func TestProblemsLimit(t *testing.T) {
	page := minimalPage([]struct{ id, name, difficulty string }{
		{"helloworld", "Hello World!", "Easy"},
		{"aplusb", "A+B", "Easy"},
		{"sortingtest", "Sorting Test", "Medium"},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, page)
	}))
	defer srv.Close()

	cfg := kattis.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	client := kattis.NewClient(cfg)

	problems, err := client.Problems(context.Background(), "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 2 {
		t.Errorf("got %d problems with limit=2, want 2", len(problems))
	}
}

func TestSearch(t *testing.T) {
	page := minimalPage([]struct{ id, name, difficulty string }{
		{"helloworld", "Hello World!", "Easy"},
		{"sortingtest", "Sorting Test", "Medium"},
		{"sortingexpert", "Sorting Expert", "Hard"},
		{"graphbfs", "Graph BFS", "Medium"},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, page)
	}))
	defer srv.Close()

	cfg := kattis.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	client := kattis.NewClient(cfg)

	hits, err := client.Search(context.Background(), "sorting", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Errorf("search 'sorting': got %d hits, want 2", len(hits))
	}

	hits, err = client.Search(context.Background(), "graph", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Name != "Graph BFS" {
		t.Errorf("search 'graph': got %v", hits)
	}
}

func TestSearchLimit(t *testing.T) {
	page := minimalPage([]struct{ id, name, difficulty string }{
		{"sortinga", "Sorting A", "Easy"},
		{"sortingb", "Sorting B", "Easy"},
		{"sortingc", "Sorting C", "Medium"},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, page)
	}))
	defer srv.Close()

	cfg := kattis.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	client := kattis.NewClient(cfg)

	hits, err := client.Search(context.Background(), "sorting", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Errorf("search with limit=2: got %d hits, want 2", len(hits))
	}
}

func TestProblemURL(t *testing.T) {
	page := minimalPage([]struct{ id, name, difficulty string }{
		{"helloworld", "Hello World!", "Easy"},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, page)
	}))
	defer srv.Close()

	cfg := kattis.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	client := kattis.NewClient(cfg)

	problems, err := client.Problems(context.Background(), "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 {
		t.Fatalf("got %d problems, want 1", len(problems))
	}
	p := problems[0]
	wantURL := srv.URL + "/problems/helloworld"
	if p.URL != wantURL {
		t.Errorf("URL = %q, want %q", p.URL, wantURL)
	}
	if p.ID != "helloworld" {
		t.Errorf("ID = %q, want %q", p.ID, "helloworld")
	}
}

func TestProblemDifficulty(t *testing.T) {
	page := minimalPage([]struct{ id, name, difficulty string }{
		{"easy1", "Easy Problem", "Easy"},
		{"medium1", "Medium Problem", "Medium"},
		{"hard1", "Hard Problem", "Hard"},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, page)
	}))
	defer srv.Close()

	cfg := kattis.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	client := kattis.NewClient(cfg)

	problems, err := client.Problems(context.Background(), "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 3 {
		t.Fatalf("got %d problems, want 3", len(problems))
	}

	byID := make(map[string]kattis.Problem)
	for _, p := range problems {
		byID[p.ID] = p
	}

	if byID["easy1"].Category != "Easy" {
		t.Errorf("easy1 category = %q, want Easy", byID["easy1"].Category)
	}
	if byID["medium1"].Category != "Medium" {
		t.Errorf("medium1 category = %q, want Medium", byID["medium1"].Category)
	}
	if byID["hard1"].Category != "Hard" {
		t.Errorf("hard1 category = %q, want Hard", byID["hard1"].Category)
	}
}
