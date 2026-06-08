package handlers

import (
	"net/http"

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

	c.HTML(
		http.StatusOK,
		"blogpost.html",
		merge(
			pageMeta(blog.Title+" · Vandit Singh", description, "/blogs/"+slug),
			gin.H{
				"blog":         blog,
				"OGType":       "article",
				"IsArticle":    true,
				"ArticleDate":  blog.Date,
				"ArticleTitle": blog.Title,
			},
		),
	)
}
