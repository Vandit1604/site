package handlers

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/vandit1604/site/types"
	"gopkg.in/yaml.v2"
)

// readChangelogYAML reads the changelog entries from a YAML file and unmarshals them into a slice of Changelog structs.
func readChangelogYAML(filePath string) ([]types.Changelog, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var pageData types.PageData
	err = yaml.Unmarshal(data, &pageData)
	if err != nil {
		return nil, err
	}

	return pageData.Changelogs, nil
}

// ShowChangelogPage renders the changelog page using the provided Gin context.
func ShowChangelogPage(c *gin.Context) {
	changelogs, err := readChangelogYAML("/home/vandit/codes/site/content/changelog/changelog.yml")
	if err != nil {
		log.Printf("Error reading changelogs: %v", err)
		c.String(http.StatusInternalServerError, "Unable to load changelogs")
		return
	}

	// Render the HTML page using the changelog data
	c.HTML(http.StatusOK, "changelog.html", gin.H{
		"Changelogs": changelogs,
	})
}
