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
func readProjectYAML(filePath string) ([]types.Project, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var projects types.Projects
	err = yaml.Unmarshal(data, &projects)
	if err != nil {
		return nil, err
	}

	return projects.Projects, nil
}

// ShowChangelogPage renders the changelog page using the provided Gin context.
func ShowProjectsPage(c *gin.Context) {
	projects, err := readProjectYAML("/home/vandit/codes/site/content/projects/projects.yml")
	if err != nil {
		log.Printf("Error reading projects: %v", err)
		c.String(http.StatusInternalServerError, "Unable to load projects")
		return
	}

	// Render the HTML page using the changelog data
	c.HTML(http.StatusOK, "projects.html", gin.H{
		"Projects": projects,
	})
}
