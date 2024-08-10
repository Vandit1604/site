package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vandit1604/site/models"
)

func ShowNotFoundPage(c *gin.Context) {
	c.HTML(http.StatusNotFound, "404.html", nil)
}

func ShowIndexPage(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", nil)
}

func ShowBlogPage(c *gin.Context) {
	blogs := models.ReadBlogs()

	c.HTML(
		http.StatusOK,
		"blogs.html",
		gin.H{"blogs": blogs},
	)
}
