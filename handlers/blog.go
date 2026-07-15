package handlers

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/vandit1604/site/models"
)

func ShowIndividualBlogPage(c *gin.Context) {
	slug := c.Param("slug")
	blogs := models.ReadBlogs()

	blog, exists := blogs[slug]
	if !exists {
		ShowNotFoundPage(c)
		return
	}

	// Concise, deterministic description derived from the title; the body is
	// already-rendered HTML so we avoid stripping it for the meta tag.
	description := blog.Title + " · a post by Vandit Singh on Go, distributed systems, and engineering."

	// Deep-link the post into ChatGPT / Claude so a reader can hand the article
	// straight to an assistant to summarize or ask questions about.
	articleURL := SiteURL + "/blogs/" + slug
	q := url.QueryEscape("Read this article by Vandit Singh and help me understand it: " + articleURL)

	c.HTML(
		http.StatusOK,
		"blogpost.html",
		merge(
			pageMeta(blog.Title+" · Vandit Singh", description, "/blogs/"+slug),
			gin.H{
				"blog":         blog,
				"OGType":       "article",
				"OGImage":      SiteURL + "/static/images/blog/og/" + slug + ".png",
				"IsArticle":    true,
				"ArticleDate":  blog.Date,
				"ArticleTitle": blog.Title,
				"ChatGPTURL":   "https://chatgpt.com/?q=" + q,
				"ClaudeURL":    "https://claude.ai/new?q=" + q,
			},
		),
	)
}
