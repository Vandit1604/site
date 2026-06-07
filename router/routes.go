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
	router.Static("/static", "./static")
	router.StaticFS("../assets/", http.Dir("assets"))

	// SEO: serve robots.txt and sitemap.xml from the root
	router.StaticFile("/robots.txt", "./static/robots.txt")
	router.StaticFile("/sitemap.xml", "./static/sitemap.xml")

	// Health check endpoint for container / platform probes.
	router.GET("/healthz", handlers.ShowHealth)

	// 404 page
	router.NoRoute(handlers.ShowNotFoundPage)

	router.GET("/", handlers.ShowIndexPage)
	router.GET("/projects", handlers.ShowProjectsPage)
	router.GET("/blogs", handlers.ShowBlogPage)
	router.GET("/blogs/:slug", handlers.ShowIndividualBlogPage)
	router.GET("/talks", handlers.ShowTalksPage)
	router.GET("/library", handlers.ShowLibraryPage)
	router.GET("/gallery", handlers.ShowGalleryPage)
	router.GET("/resume", handlers.RedirectToResume)
}
