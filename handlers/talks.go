package handlers

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/vandit1604/site/types"
	"gopkg.in/yaml.v2"
)

func ShowTalksPage(c *gin.Context) {
	yamlFile, err := os.ReadFile("content/talks.yml")
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Failed to read talks data"})
		return
	}

	var talksData struct {
		Talks []types.Talk `yaml:"talks"`
	}

	err = yaml.Unmarshal(yamlFile, &talksData)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Failed to parse talks data"})
		return
	}

	c.HTML(http.StatusOK, "talks.html", gin.H{
		"Talks": talksData.Talks,
	})
}
