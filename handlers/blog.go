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
		c.HTML(http.StatusNotFound, "404.html", nil)
		return
	}

	c.HTML(
		http.StatusOK,
		"blogpost.html",
		gin.H{"blog": blog},
	)
}
