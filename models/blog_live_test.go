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
