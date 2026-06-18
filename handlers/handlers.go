package handlers

import (
	"log"
	"net/http"
	"slices"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vandit1604/site/models"
	"github.com/vandit1604/site/spotify"
	"github.com/vandit1604/site/types"
)

var ResumeURL string = "https://drive.google.com/file/d/1PFmsMZC3fvg6W6GsCiglt2b2othhn7S6/view?usp=drive_link"

// blogDateLayouts tolerates loose front-matter dates (zero-padded or not), so a
// post dated "2024-9-26" sorts correctly alongside "2024-11-01".
var blogDateLayouts = []string{"2006-01-02", "2006-1-2", "2006-01-2", "2006-1-02"}

func parseBlogDate(s string) time.Time {
	for _, layout := range blogDateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// sortedBlogs flattens the blog map into a slice ordered newest-first. Ranging a
// Go map is randomized, so templates must be handed an explicitly sorted slice.
func sortedBlogs(blogs map[string]types.BlogPost) []types.BlogPost {
	out := make([]types.BlogPost, 0, len(blogs))
	for _, blog := range blogs {
		out = append(out, blog)
	}
	sort.Slice(out, func(i, j int) bool {
		return parseBlogDate(out[i].Date).After(parseBlogDate(out[j].Date))
	})
	return out
}

func ShowNotFoundPage(c *gin.Context) {
	c.HTML(http.StatusNotFound, "404.html", gin.H{
		"MetaTitle":       "Page not found · Vandit Singh",
		"MetaDescription": "The page you were looking for doesn't exist.",
		"Noindex":         true,
	})
}

func ShowIndexPage(c *gin.Context) {
	blogs := models.ReadBlogs()

	// Newest-first, tolerant of loose date formats in post front-matter.
	blogSlice := sortedBlogs(blogs)

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
			"Vandit Singh — Golang & Distributed Systems Engineer",
			"Golang engineer for distributed systems, storage & p2p. Merged contributor to Kubernetes, Prometheus & Jenkins.",
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
			if slices.Contains(blog.Tags, selectedTag) {
				filteredBlogs[slug] = blog
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
				"blogs":       sortedBlogs(blogs),
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
