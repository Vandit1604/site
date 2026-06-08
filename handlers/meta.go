package handlers

import "github.com/gin-gonic/gin"

// SiteURL is the canonical production origin, used to build absolute URLs for
// canonicals, Open Graph tags, and the sitemap. No trailing slash.
const SiteURL = "https://vandit.dev"

// defaultOGImage is the fallback social-share image for pages that don't set one.
const defaultOGImage = SiteURL + "/static/images/og.png"

// pageMeta returns the per-page SEO fields consumed by header.html. Keeping this
// in one place stops every page from collapsing onto the homepage's title and
// canonical, which is the most common single-template SEO bug.
//
// path is the route path beginning with "/" (e.g. "/projects"); for the
// homepage pass "/". title and description should be unique per page.
func pageMeta(title, description, path string) gin.H {
	return gin.H{
		"MetaTitle":       title,
		"MetaDescription": description,
		"Canonical":       SiteURL + path,
		"OGType":          "website",
		"OGImage":         defaultOGImage,
	}
}

// merge layers page-specific data on top of the SEO meta without mutating
// either input, so handlers can write `merge(pageMeta(...), gin.H{...})`.
func merge(base, extra gin.H) gin.H {
	out := gin.H{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}
