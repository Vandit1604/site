package handlers

import (
	"testing"

	"github.com/vandit1604/site/types"
)

func TestRelatedPosts(t *testing.T) {
	blogs := map[string]types.BlogPost{
		"cur":    {Slug: "cur", Date: "2026-01-01", Tags: []string{"go", "k8s"}},
		"both":   {Slug: "both", Date: "2024-01-01", Tags: []string{"go", "k8s"}},
		"one":    {Slug: "one", Date: "2025-01-01", Tags: []string{"go"}},
		"none":   {Slug: "none", Date: "2026-05-01", Tags: []string{"life"}},
		"older1": {Slug: "older1", Date: "2023-01-01", Tags: []string{"go"}},
	}
	got := relatedPosts(blogs, "cur", 3)
	want := []string{"both", "one", "older1"}
	if len(got) != len(want) {
		t.Fatalf("got %d posts, want %d", len(got), len(want))
	}
	for i, b := range got {
		if b.Slug != want[i] {
			t.Errorf("post %d = %s, want %s", i, b.Slug, want[i])
		}
	}
}
