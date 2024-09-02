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
	blogs := models.ReadBlogs()
	c.HTML(http.StatusOK, "index.html", gin.H{"blogs": blogs})
}

func ShowBlogPage(c *gin.Context) {
	blogs := models.ReadBlogs()

	c.HTML(
		http.StatusOK,
		"allblogs.html",
		gin.H{"blogs": blogs},
	)
}
