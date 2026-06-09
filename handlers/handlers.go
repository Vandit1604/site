package handlers

import (
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vandit1604/site/models"
	"github.com/vandit1604/site/spotify"
	"github.com/vandit1604/site/types"
)

var ResumeURL string = "https://drive.google.com/file/d/1PFmsMZC3fvg6W6GsCiglt2b2othhn7S6/view?usp=drive_link"

func ShowNotFoundPage(c *gin.Context) {
	c.HTML(http.StatusNotFound, "404.html", gin.H{
		"MetaTitle":       "Page not found · Vandit Singh",
		"MetaDescription": "The page you were looking for doesn't exist.",
		"Noindex":         true,
	})
}

func ShowIndexPage(c *gin.Context) {
	blogs := models.ReadBlogs()

	// Convert map to slice for sorting
	blogSlice := make([]types.BlogPost, 0, len(blogs))
	for _, blog := range blogs {
		blogSlice = append(blogSlice, blog)
	}

	// Sort blogs by date (most recent first)
	sort.Slice(blogSlice, func(i, j int) bool {
		dateI, _ := time.Parse("2006-01-02", blogSlice[i].Date)
		dateJ, _ := time.Parse("2006-01-02", blogSlice[j].Date)
		return dateI.After(dateJ)
	})

	// Get the two most recent blogs
	var recentBlogs []types.BlogPost
	if len(blogSlice) > 2 {
		recentBlogs = blogSlice[:2]
	} else {
		recentBlogs = blogSlice
	}

	// Surface the top projects as "featured work" on the homepage.
	// projects.yml is ordered most-important-first, so we take the leading few.
	featuredProjects, err := readProjectYAML("content/projects.yml")
	if err != nil {
		log.Printf("Error reading projects for homepage: %v", err)
		featuredProjects = nil
	}
	if len(featuredProjects) > 3 {
		featuredProjects = featuredProjects[:3]
	}

	c.HTML(http.StatusOK, "index.html", merge(
		pageMeta(
			"Vandit Singh · Golang Engineer | Distributed Systems & Observability",
			"Go/infra engineer. I build container runtimes and distributed storage, and I'm exploring blockchain node infrastructure. Merged contributor to Kubernetes, Prometheus, and Jenkins.",
			"/",
		),
		gin.H{
			"recentBlogs":      recentBlogs,
			"featuredProjects": featuredProjects,
			// nil when Spotify isn't configured; the template hides the widget.
			"nowPlaying": spotify.RecentlyPlayed(),
		},
	))
}

// ShowSpotifyDebug reports, without leaking secret values, whether the Spotify
// env vars are visible to this process and what a live fetch returns. Temporary
// diagnostic route — remove once the widget is confirmed working in prod.
func ShowSpotifyDebug(c *gin.Context) {
	c.JSON(http.StatusOK, spotify.Diagnose())
}

func RedirectToResume(c *gin.Context) {
	c.Redirect(http.StatusPermanentRedirect, ResumeURL)
}

func ShowBlogPage(c *gin.Context) {
	blogs := models.ReadBlogs()
	selectedTag := c.Query("tag")

	// Get all unique tags
	allTags := getAllTags(blogs)

	// Filter blogs by tag if a tag is selected
	if selectedTag != "" {
		filteredBlogs := make(map[string]types.BlogPost)
		for slug, blog := range blogs {
			for _, tag := range blog.Tags {
				if tag == selectedTag {
					filteredBlogs[slug] = blog
					break
				}
			}
		}
		blogs = filteredBlogs
	}

	title := "Blogs · Vandit Singh"
	description := "Writing on Go, distributed systems, open source contribution, and engineering career notes by Vandit Singh."
	if selectedTag != "" {
		title = selectedTag + " · Blogs by Vandit Singh"
	}

	c.HTML(
		http.StatusOK,
		"allblogs.html",
		merge(
			// Tag-filtered views canonicalize to /blogs so thin near-duplicate
			// tag pages don't compete with the main listing.
			pageMeta(title, description, "/blogs"),
			gin.H{
				"blogs":       blogs,
				"Route":       "/blogs",
				"selectedTag": selectedTag,
				"allTags":     allTags,
				// Don't index tag-filtered permutations; they're thin slices.
				"Noindex": selectedTag != "",
			},
		),
	)
}

// Helper function to get all unique tags
func getAllTags(blogs map[string]types.BlogPost) []string {
	tagSet := make(map[string]bool)
	for _, blog := range blogs {
		for _, tag := range blog.Tags {
			tagSet[tag] = true
		}
	}

	var allTags []string
	for tag := range tagSet {
		allTags = append(allTags, tag)
	}
	return allTags
}
