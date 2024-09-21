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
func readExperiencesYAML(filePath string) ([]types.Experience, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var pageData types.Experiences
	err = yaml.Unmarshal(data, &pageData)
	if err != nil {
		return nil, err
	}

	return pageData.Experiences, nil
}

// ShowExperiencesPage renders the changelog page using the provided Gin context.
func ShowExperiencesPage(c *gin.Context) {
	experiences, err := readExperiencesYAML("/home/vandit/codes/site/content/experiences.yml")
	if err != nil {
		log.Printf("Error reading changelogs: %v", err)
		c.String(http.StatusInternalServerError, "Unable to load changelogs")
		return
	}

	// Render the HTML page using the changelog data
	c.HTML(http.StatusOK, "experience.html", gin.H{
		"Experiences": experiences,
	})
}
