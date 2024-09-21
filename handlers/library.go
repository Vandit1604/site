package handlers

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/vandit1604/site/types"
	"gopkg.in/yaml.v2"
)

func readLibraryYAML(filePath string) (types.Library, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return types.Library{}, err
	}

	var library types.Library
	err = yaml.Unmarshal(data, &library)
	if err != nil {
		return types.Library{}, err
	}

	return library, nil
}

func ShowLibraryPage(c *gin.Context) {
	library, err := readLibraryYAML("content/library.yml")
	if err != nil {
		log.Printf("Error reading library: %v", err)
		c.String(http.StatusInternalServerError, "Unable to load library")
		return
	}

	c.HTML(http.StatusOK, "library.html", gin.H{
		"Categories": library.Categories,
	})
}
