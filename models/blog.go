package models

import (
	"html/template"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/russross/blackfriday/v2"
	"github.com/vandit1604/site/types"
)

func ReadBlogs() map[string]types.BlogPost {
	blogs := make(map[string]types.BlogPost)
	dir := "content/blogs/"
	files, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("Error reading directory: %v", err)
		return blogs
	}

	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
			slug := strings.TrimSuffix(f.Name(), ".md")
			filepath := filepath.Join(dir, f.Name())
			data, err := os.ReadFile(filepath)
			if err != nil {
				log.Printf("Error reading file %s: %v", f.Name(), err)
				continue
			}

			blogPost := transformDataToBlog(slug, string(data))
			blogs[slug] = *blogPost
		}
	}

	return blogs
}

func transformDataToBlog(slug, data string) *types.BlogPost {
	lines := strings.Split(data, "\n")
	var title string
	var contentLines []string
	titleFound := false

	for _, line := range lines {
		if strings.HasPrefix(line, "# ") && !titleFound {
			// Extract the title from the line starting with "# "
			title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			titleFound = true
		}
		// Add all lines to content, including title
		contentLines = append(contentLines, line)
	}

	// Join all content lines into a single string
	markdownContent := strings.Join(contentLines, "\n")

	// Convert markdown content to HTML using Blackfriday
	htmlContent := string(blackfriday.Run([]byte(markdownContent)))

	// Convert the string HTML content to template.HTML
	// to safely render it as HTML in the template
	blog := &types.BlogPost{
		Slug:    slug,
		Title:   title,
		Content: template.HTML(htmlContent), // Convert string to template.HTML here
	}

	return blog
}
