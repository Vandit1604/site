package models

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/vandit1604/site/types"
)

func ReadBlogs() map[string]types.BlogPost {
	blogs := make(map[string]types.BlogPost)
	dir := "./blogs/"
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
		} else if titleFound && line != "" {
			// Add the remaining lines to content after the title has been found
			contentLines = append(contentLines, fmt.Sprintf("%s\n", line))
		}
	}

	// Join all content lines into a single string
	content := strings.Join(contentLines, "\n")

	// Create and return the BlogPost struct with the title, content, and slug
	blog := &types.BlogPost{
		Slug:    slug,
		Title:   title,
		Content: content,
	}

	return blog
}
