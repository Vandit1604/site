package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func ShowNotFoundPage(c *gin.Context) {
	c.HTML(http.StatusNotFound, "404.html", nil)
}

func ShowIndexPage(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", nil)
}
