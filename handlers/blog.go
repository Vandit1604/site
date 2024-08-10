package handlers

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/yuin/goldmark"
)

type Blog struct {
	Title   string
	Payload string
}

func ReadFile(slug string) (string, error) {
	f, err := os.Open("../blogs/" + slug + ".md")
	if err != nil {
		return "", err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func ShowIndiviualBlogPage(c *gin.Context) {
	var blog Blog
	slug, _ := c.Params.Get("slug")
	postMarkdown, err := ReadFile(slug)
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}

	var buf bytes.Buffer
	err = goldmark.Convert([]byte(postMarkdown), &buf)
	if err != nil {
		http.Error(c.Writer, "Error converting markdown", http.StatusInternalServerError)
		return
	}

	if err != nil {
		// TODO: Handle different errors in the future
		http.Error(c.Writer, "Post not found", http.StatusNotFound)
		return
	}

	io.Copy(c.Writer, &buf)

	c.HTML(
		http.StatusOK,
		"blogpost.html",
		gin.H{"blog": blog},
	)
}
