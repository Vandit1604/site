package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vandit1604/site/handlers"
)

func Run() {
	router := gin.Default()
	// load the html files under templates
	// Once loaded, these don’t have to be read again on every request making Gin web applications very fast.
	router.LoadHTMLGlob("templates/*")

	setUpRoutes(router)

	router.Run()
}

func setUpRoutes(router *gin.Engine) {
	router.StaticFS("../assets/", http.Dir("assets"))
	// 404 page
	router.NoRoute(handlers.ShowNotFoundPage)

	router.GET("/", handlers.ShowIndexPage)
	router.GET("/changelog", handlers.ShowChangelogPage)
	router.GET("/projects", handlers.ShowProjectsPage)
	router.GET("/blog", handlers.ShowBlogPage)
	router.GET("/blog/:slug", handlers.ShowIndividualBlogPage)
}
