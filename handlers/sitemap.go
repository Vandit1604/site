package handlers

import (
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/vandit1604/site/models"
)

// sitemapEntry is one <url> in the sitemap.
type sitemapEntry struct {
	Loc        string
	ChangeFreq string
	Priority   string
}

// staticRoutes are the fixed public pages, in priority order.
var staticRoutes = []sitemapEntry{
	{Loc: "/", ChangeFreq: "weekly", Priority: "1.0"},
	{Loc: "/projects", ChangeFreq: "monthly", Priority: "0.8"},
	{Loc: "/blogs", ChangeFreq: "weekly", Priority: "0.8"},
	{Loc: "/talks", ChangeFreq: "monthly", Priority: "0.6"},
	{Loc: "/library", ChangeFreq: "monthly", Priority: "0.6"},
	{Loc: "/gallery", ChangeFreq: "monthly", Priority: "0.5"},
}

// sitemapEntries returns the full ordered list of sitemap entries: the fixed
// static pages followed by every published blog post. Drafts are excluded
// because ReadBlogs already filters them out. Shared by the XML sitemap and
// IndexNow submission so the two can never drift.
func sitemapEntries() []sitemapEntry {
	entries := make([]sitemapEntry, 0, len(staticRoutes)+8)
	entries = append(entries, staticRoutes...)

	blogs := models.ReadBlogs()
	slugs := make([]string, 0, len(blogs))
	for slug := range blogs {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	for _, slug := range slugs {
		entries = append(entries, sitemapEntry{
			Loc:        "/blogs/" + slug,
			ChangeFreq: "yearly",
			Priority:   "0.7",
		})
	}
	return entries
}

// AllURLs returns every public absolute URL in the sitemap, used by IndexNow
// submission to notify Bing and other participating engines of the full page set.
func AllURLs() []string {
	entries := sitemapEntries()
	urls := make([]string, len(entries))
	for i, e := range entries {
		urls[i] = SiteURL + e.Loc
	}
	return urls
}

// ShowSitemap renders the XML sitemap dynamically so newly published blog posts
// are discoverable without hand-editing a static file.
func ShowSitemap(c *gin.Context) {
	entries := sitemapEntries()

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, e := range entries {
		b.WriteString("  <url>\n")
		b.WriteString("    <loc>" + SiteURL + e.Loc + "</loc>\n")
		b.WriteString("    <changefreq>" + e.ChangeFreq + "</changefreq>\n")
		b.WriteString("    <priority>" + e.Priority + "</priority>\n")
		b.WriteString("  </url>\n")
	}
	b.WriteString("</urlset>\n")

	c.Data(http.StatusOK, "application/xml; charset=utf-8", []byte(b.String()))
}
