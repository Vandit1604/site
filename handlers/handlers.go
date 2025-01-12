package handlers

import (
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vandit1604/site/models"
	"github.com/vandit1604/site/types"
)

var ResumeURL string = "https://drive.google.com/drive/u/0/folders/14PeLd2rs6EOKoeUjCvK6IqvLt8AVh4Y4"

func ShowNotFoundPage(c *gin.Context) {
	c.HTML(http.StatusNotFound, "404.html", nil)
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

	c.HTML(http.StatusOK, "index.html", gin.H{
		"recentBlogs": recentBlogs,
	})
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

	c.HTML(
		http.StatusOK,
		"allblogs.html",
		gin.H{
			"blogs":       blogs,
			"Route":       "/blogs",
			"selectedTag": selectedTag,
			"allTags":     allTags,
		},
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
