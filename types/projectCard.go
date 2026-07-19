package types

import (
	"net/url"
	"strings"
)

// Every project has exactly one destination — a live site or a repo, never
// both — which is why the card can be a single link the way the indie cards
// are. PrimaryURL is empty only if a project has neither, in which case the
// card renders as a plain block.
func (p Project) PrimaryURL() string {
	if p.DemoLink != "" {
		return p.DemoLink
	}
	return p.GithubLink
}

// PrimaryLabel is what the card prints next to its arrow. A product shows the
// domain it lives at; a repo shows owner/name, which says more than the word
// "GitHub" repeated across six cards.
func (p Project) PrimaryLabel() string {
	raw := p.PrimaryURL()
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	host := strings.TrimPrefix(u.Host, "www.")
	path := strings.Trim(u.Path, "/")
	if host == "github.com" && path != "" {
		return path
	}
	return host
}
