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

	// SEO: serve robots.txt from disk; sitemap.xml is generated dynamically so
	// newly published blog posts are always included.
	router.StaticFile("/robots.txt", "./static/robots.txt")
	router.GET("/sitemap.xml", handlers.ShowSitemap)

	// IndexNow: serve the ownership key file so Bing/Yandex/etc. can validate
	// the URL submissions made by `site -indexnow` on deploy.
	router.GET("/"+handlers.IndexNowKey+".txt", handlers.ShowIndexNowKey)

	// Health check endpoint for container / platform probes.
	router.GET("/healthz", handlers.ShowHealth)

	// 404 page
	router.NoRoute(handlers.ShowNotFoundPage)

	// JSON APIs consumed by the frontend: the ⌘K palette index and the
	// persistent page-view counter shown in the nav.
	router.GET("/api/search-index.json", handlers.ShowSearchIndex)
	// GET reads the total; POST increments it (once per browser, so the count
	// is unique visitors rather than page loads).
	router.GET("/api/views", handlers.ShowViews)
	router.POST("/api/views", handlers.CountView)

	router.GET("/", handlers.ShowIndexPage)
	router.GET("/projects", handlers.ShowProjectsPage)
	router.GET("/blogs", handlers.ShowBlogPage)
	router.GET("/blogs/:slug", handlers.ShowIndividualBlogPage)
	router.GET("/talks", handlers.ShowTalksPage)
	router.GET("/library", handlers.ShowLibraryPage)
	router.GET("/gallery", handlers.ShowGalleryPage)
	router.GET("/resume", handlers.RedirectToResume)
}
