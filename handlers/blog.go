package handlers

import (
	"net/http"
	"net/url"
	"slices"

	"github.com/vandit1604/site/types"

	"github.com/gin-gonic/gin"
	"github.com/vandit1604/site/models"
)

func ShowIndividualBlogPage(c *gin.Context) {
	slug := c.Param("slug")
	blogs := models.ReadBlogs()

	blog, exists := blogs[slug]
	if !exists {
		ShowNotFoundPage(c)
		return
	}

	description := blog.Description
	if description == "" {
		description = blog.Title + " · a post by Vandit Singh on Go, distributed systems, and engineering."
	}

	// Deep-link the post into ChatGPT / Claude so a reader can hand the article
	// straight to an assistant to summarize or ask questions about.
	articleURL := SiteURL + "/blogs/" + slug
	q := url.QueryEscape("Read this article by Vandit Singh and help me understand it: " + articleURL)

	c.HTML(
		http.StatusOK,
		"blogpost.html",
		merge(
			pageMeta(blog.Title+" · Vandit Singh", description, "/blogs/"+slug),
			gin.H{
				"blog":         blog,
				"OGType":       "article",
				"OGImage":      SiteURL + "/static/images/blog/og/" + slug + ".png",
				"IsArticle":    true,
				"ArticleDate":  blog.Date,
				"ArticleMod":   blogLastMod(blog),
				"related":      relatedPosts(blogs, slug, 3),
				"ArticleTitle": blog.Title,
				"ChatGPTURL":   "https://chatgpt.com/?q=" + q,
				"ClaudeURL":    "https://claude.ai/new?q=" + q,
			},
		),
	)
}

// relatedPosts returns up to n other posts, most shared tags first, newest
// first on ties.
func relatedPosts(blogs map[string]types.BlogPost, slug string, n int) []types.BlogPost {
	current := blogs[slug]
	shared := func(b types.BlogPost) int {
		count := 0
		for _, t := range b.Tags {
			if slices.Contains(current.Tags, t) {
				count++
			}
		}
		return count
	}
	posts := slices.DeleteFunc(sortedBlogs(blogs), func(b types.BlogPost) bool { return b.Slug == slug })
	slices.SortStableFunc(posts, func(x, y types.BlogPost) int { return shared(y) - shared(x) })
	return posts[:min(n, len(posts))]
}
