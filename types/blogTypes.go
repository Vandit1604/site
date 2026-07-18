package types

import "html/template"

type BlogPost struct {
	Draft   string
	Slug    string
	Title   string
	Content template.HTML
	Date    string
	// Updated is an optional YYYY-MM-DD frontmatter field marking a meaningful
	// revision. It drives sitemap <lastmod> when present; the container runs on
	// scratch with no .git, so commit time isn't available at runtime, and
	// author intent is the better signal anyway (a typo fix shouldn't ask
	// crawlers to revisit).
	Updated     string
	Tags        []string
	Description string // optional one-line summary, used on the featured card + meta
	ReadingTime int    // estimated minutes, computed from word count
}
