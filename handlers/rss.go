package handlers

import (
	"encoding/xml"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vandit1604/site/models"
)

// feedTitle and feedDesc describe the channel to readers and aggregators.
const (
	feedTitle = "Vandit Singh — Blog"
	feedDesc  = "Deep-dives on infrastructure, distributed systems, Go, and CNCF projects."
)

// xmlEsc escapes a string for safe inclusion in XML text/attribute content.
func xmlEsc(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// ShowRSS renders an RSS 2.0 feed of every published blog post, newest first,
// so readers can follow in a feed reader and aggregators (Golang Weekly,
// lobste.rs, etc.) can ingest new posts automatically. Built dynamically from
// the same source as the sitemap so the two never drift.
func ShowRSS(c *gin.Context) {
	blogs := models.ReadBlogs()

	type item struct {
		title, link, desc, date string
	}
	items := make([]item, 0, len(blogs))
	for slug, b := range blogs {
		desc := b.Description
		if desc == "" {
			desc = b.Title
		}
		items = append(items, item{
			title: b.Title,
			link:  SiteURL + "/blogs/" + slug,
			desc:  desc,
			date:  b.Date, // "YYYY-MM-DD"
		})
	}
	// Newest first by ISO date string (lexical sort works for YYYY-MM-DD).
	sort.Slice(items, func(i, j int) bool { return items[i].date > items[j].date })

	// pubDate needs RFC1123Z; parse the ISO date and pin to noon UTC so the
	// value is stable regardless of when the feed is served.
	rfc := func(iso string) string {
		t, err := time.Parse("2006-01-02", iso)
		if err != nil {
			t = time.Unix(0, 0).UTC()
		}
		return t.Add(12 * time.Hour).UTC().Format(time.RFC1123Z)
	}

	lastBuild := time.Unix(0, 0).UTC().Format(time.RFC1123Z)
	if len(items) > 0 {
		lastBuild = rfc(items[0].date)
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">` + "\n")
	b.WriteString("  <channel>\n")
	b.WriteString("    <title>" + xmlEsc(feedTitle) + "</title>\n")
	b.WriteString("    <link>" + SiteURL + "/blogs</link>\n")
	b.WriteString(`    <atom:link href="` + SiteURL + `/rss.xml" rel="self" type="application/rss+xml"/>` + "\n")
	b.WriteString("    <description>" + xmlEsc(feedDesc) + "</description>\n")
	b.WriteString("    <language>en-us</language>\n")
	b.WriteString("    <lastBuildDate>" + lastBuild + "</lastBuildDate>\n")
	for _, it := range items {
		b.WriteString("    <item>\n")
		b.WriteString("      <title>" + xmlEsc(it.title) + "</title>\n")
		b.WriteString("      <link>" + it.link + "</link>\n")
		b.WriteString(`      <guid isPermaLink="true">` + it.link + "</guid>\n")
		b.WriteString("      <pubDate>" + rfc(it.date) + "</pubDate>\n")
		b.WriteString("      <description>" + xmlEsc(it.desc) + "</description>\n")
		b.WriteString("    </item>\n")
	}
	b.WriteString("  </channel>\n")
	b.WriteString("</rss>\n")

	c.Data(http.StatusOK, "application/rss+xml; charset=utf-8", []byte(b.String()))
}
