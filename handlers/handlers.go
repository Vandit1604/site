package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vandit1604/site/models"
	"github.com/vandit1604/site/types"
)

func ShowNotFoundPage(c *gin.Context) {
	c.HTML(http.StatusNotFound, "404.html", nil)
}

func ShowIndexPage(c *gin.Context) {
	blogs := models.ReadBlogs()
	c.HTML(http.StatusOK, "index.html", gin.H{
		"blogs": blogs,
	})
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
