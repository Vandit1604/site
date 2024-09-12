package handlers

import (
	"bytes"
	"html/template"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vandit1604/site/types"
	"github.com/yuin/goldmark"
)

func ConvertMarkdown(content string) (string, error) {
	md := goldmark.New()
	var buf bytes.Buffer
	if err := md.Convert([]byte(content), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ShowIndividualBlogPage serves a blog post
func ShowIndividualBlogPage(c *gin.Context) {
	slug, exists := c.Params.Get("slug")
	if !exists {
		http.Error(c.Writer, "Post not found", http.StatusNotFound)
		return
	}

	frontMatter, content, err := ReadFileWithFrontMatter(slug)
	if err != nil {
		log.Printf("Error reading file: %v", err)
		http.Error(c.Writer, "Error reading file", http.StatusInternalServerError)
		return
	}

	htmlContent, err := ConvertMarkdown(content)
	if err != nil {
		log.Printf("Error converting markdown: %v", err)
		http.Error(c.Writer, "Error converting markdown", http.StatusInternalServerError)
		return
	}

	blog := types.BlogPost{
		Slug:    slug,
		Title:   frontMatter.Title,
		Content: template.HTML(htmlContent), // Now valid with the new type
		Date:    frontMatter.Date,
		Tags:    frontMatter.Tags,
	}

	c.HTML(
		http.StatusOK,
		"blogpost.html",
		gin.H{"blog": blog},
	)
}
