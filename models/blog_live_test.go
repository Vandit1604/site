package models

import (
	"testing"
	"time"

	"github.com/vandit1604/site/types"
)

func TestIsLive(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		post types.BlogPost
		want bool
	}{
		{"past", types.BlogPost{Date: "2026-11-03"}, true},
		{"today", types.BlogPost{Date: "2026-11-10"}, true},
		{"tomorrow", types.BlogPost{Date: "2026-11-11"}, false},
		{"loose format future", types.BlogPost{Date: "2026-12-1"}, false},
		{"unparseable", types.BlogPost{Date: "soon"}, true},
		{"draft past", types.BlogPost{Date: "2026-11-03", Draft: "true"}, false},
	}
	for _, c := range cases {
		if got := isLive(&c.post, now); got != c.want {
			t.Errorf("%s: isLive = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestTransformKeepsFieldsAfterDraftLine(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name  string
		draft string
		date  string
		live  bool
	}{
		{"not draft, future", "false", "2026-12-25", false},
		{"not draft, past", "false", "2026-11-03", true},
		{"draft, past", "true", "2026-11-03", false},
	}
	for _, c := range cases {
		data := "---\ntitle: \"T\"\ndraft: \"" + c.draft + "\"\ndate: \"" + c.date + "\"\n---\nbody text\n"
		post := transformDataToBlog("s", data)
		if post.Title != "T" || post.Date != c.date || post.Draft != c.draft {
			t.Errorf("%s: got title %q date %q draft %q", c.name, post.Title, post.Date, post.Draft)
		}
		if got := isLive(post, now); got != c.live {
			t.Errorf("%s: isLive = %v, want %v", c.name, got, c.live)
		}
	}
}
