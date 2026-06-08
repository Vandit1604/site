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

// ShowSitemap renders the XML sitemap dynamically so newly published blog posts
// are discoverable without hand-editing a static file. Drafts are excluded
// because ReadBlogs already filters them out.
func ShowSitemap(c *gin.Context) {
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
