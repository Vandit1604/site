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
			byte, err := os.ReadFile(filepath)
			if err != nil {
				log.Printf("Error reading file %s: %v", f.Name(), err)
				continue
			}
			blogPost := transformDataToBlog(string(byte))
			blogs[slug] = *blogPost
		}
	}
	return blogs
}

func transformDataToBlog(data string) *types.BlogPost {
	lines := strings.Split(data, "\n")
	var title string
	var contentLines []string
	titleFound := false

	for _, line := range lines {
		if strings.HasPrefix(line, "# ") && !titleFound {
			title = strings.TrimPrefix(line, "# ")
			titleFound = true
		} else if titleFound && line != "" {
			contentLines = append(contentLines, fmt.Sprintf("%s\n", line))
		}
	}

	content := strings.Join(contentLines, "\n")
	blog := &types.BlogPost{
		Title:   title,
		Content: content,
	}

	return blog
}
