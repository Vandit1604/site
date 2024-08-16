package handlers

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/vandit1604/site/types"
	"github.com/yuin/goldmark"
)

func ReadFile(slug string) (string, string, error) {
	filePath := "/home/vandit/codes/site/blogs/" + slug + ".md"

	f, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error opening file: %v", err)
		return "", "", err
	}
	defer f.Close()

	var title string
	var content strings.Builder

	scanner := bufio.NewScanner(f)
	isFirstLine := true

	for scanner.Scan() {
		line := scanner.Text()

		if isFirstLine {
			// Extract the title from the first line
			if strings.HasPrefix(line, "# ") {
				title = strings.TrimSpace(line[2:])
			} else {
				// If the first line doesn't start with "# ", log an error
				log.Printf("Unexpected format: first line does not start with '# ': %s", line)
				return "", "", fmt.Errorf("invalid format: title not found")
			}
			isFirstLine = false
		} else {
			// Append remaining lines to content
			content.WriteString(line + "\n")
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading file content: %v", err)
		return "", "", err
	}

	return title, content.String(), nil
}

func ShowIndividualBlogPage(c *gin.Context) {
	slug, exists := c.Params.Get("slug")
	if !exists {
		http.Error(c.Writer, "Post not found", http.StatusNotFound)
		return
	}

	// Read the title and content from the file
	title, content, err := ReadFile(slug)
	if err != nil {
		log.Printf("Error reading file: %v", err)
		http.Error(c.Writer, "Error reading file", http.StatusInternalServerError)
		return
	}

	// Convert the markdown content to HTML using Goldmark
	var buf bytes.Buffer
	if err := goldmark.Convert([]byte(content), &buf); err != nil {
		log.Printf("Error converting markdown: %v", err)
		http.Error(c.Writer, "Error converting markdown", http.StatusInternalServerError)
		return
	}

	// Prepare the blog post struct with HTML-safe content
	blog := types.BlogPost{
		Title:   title,
		Content: buf.String(),
	}

	// Render the HTML template with the blog data
	c.HTML(
		http.StatusOK,
		"blogpost.html",
		gin.H{"blog": blog},
	)
}
