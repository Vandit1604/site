package handlers

import (
	"log"
	"net/http"
	"slices"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/vandit1604/site/models"
	"github.com/vandit1604/site/types"
)

var ResumeURL string = "/resume.pdf"

// sortedBlogs flattens the blog map into a slice ordered newest-first. Ranging a
// Go map is randomized, so templates must be handed an explicitly sorted slice.
func sortedBlogs(blogs map[string]types.BlogPost) []types.BlogPost {
	out := make([]types.BlogPost, 0, len(blogs))
	for _, blog := range blogs {
		out = append(out, blog)
	}
	sort.Slice(out, func(i, j int) bool {
		return models.ParseBlogDate(out[i].Date).After(models.ParseBlogDate(out[j].Date))
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

	recentBlogs := blogSlice[:min(4, len(blogSlice))]

	// Surface the top projects as "featured work" on the homepage.
	// projects.yml is ordered most-important-first, so we take the leading few.
	featuredProjects, err := readProjectYAML("content/projects.yml")
	if err != nil {
		log.Printf("Error reading projects for homepage: %v", err)
		featuredProjects = nil
	}
	// Four, because the template skips Aquanode (already under Experience).
	if len(featuredProjects) > 4 {
		featuredProjects = featuredProjects[:4]
	}

	c.HTML(http.StatusOK, "index.html", merge(
		pageMeta(
			"Vandit Singh | Golang & Distributed Systems Engineer",
			"Go engineer for observability & distributed systems. Merged into Kubernetes, Prometheus, VictoriaMetrics & Lightning Labs.",
			"/",
		),
		gin.H{
			"recentBlogs":      recentBlogs,
			"featuredProjects": featuredProjects,
		},
	))
}

func RedirectToResume(c *gin.Context) {
	c.Redirect(http.StatusFound, ResumeURL)
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
		title = selectedTag + " · Writing by Vandit Singh"
	}

	c.HTML(
		http.StatusOK,
		"allblogs.html",
		merge(
			// Tag-filtered views canonicalize to /blogs so thin near-duplicate
			// tag pages don't compete with the main listing. Deliberately no
			// noindex alongside it: noindex plus a cross-URL canonical are
			// contradictory, and Google may apply the noindex to the canonical
			// target, which would deindex /blogs itself.
			pageMeta(title, description, "/blogs"),
			gin.H{
				"blogs":       sortedBlogs(blogs),
				"Route":       "/blogs",
				"selectedTag": selectedTag,
				"allTags":     allTags,
			},
		),
	)
}

// getAllTags returns each tag once, most-used first (alphabetical on ties), so
// the filter row keeps a stable order instead of Go's random map order.
func getAllTags(blogs map[string]types.BlogPost) []string {
	count := make(map[string]int)
	for _, blog := range blogs {
		for _, tag := range blog.Tags {
			count[tag]++
		}
	}

	allTags := make([]string, 0, len(count))
	for tag := range count {
		allTags = append(allTags, tag)
	}
	slices.SortFunc(allTags, func(x, y string) int {
		if count[x] != count[y] {
			return count[y] - count[x]
		}
		return strings.Compare(x, y)
	})
	return allTags
}
