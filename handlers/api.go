package handlers

import (
	"net/http"
	"os"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/vandit1604/site/github"
	"github.com/vandit1604/site/models"
	"github.com/vandit1604/site/types"
	"github.com/vandit1604/site/views"
	"gopkg.in/yaml.v2"
)

// viewCounter is the site-wide page-view tally. Backed by a file at VIEWS_PATH
// so it persists across deploys when that path is on a mounted volume.
var viewCounter = views.New()

// ShowViews returns the current total WITHOUT incrementing. Used for returning
// visitors (the browser already counted once) so the tally stays unique-per-
// visitor rather than per page load.
func ShowViews(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"count": viewCounter.Count()})
}

// CountView increments the tally by one and returns the new total. The nav
// widget POSTs here exactly once per browser (gated by localStorage) so a
// visitor is only counted the first time they arrive.
func CountView(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"count": viewCounter.Increment()})
}

// ShowGitHub returns the cached GitHub activity snapshot (contribution
// calendar + top repos) rendered by the homepage widget.
func ShowGitHub(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=1800")
	c.JSON(http.StatusOK, github.Get())
}

// searchDoc is one entry in the command-palette index.
type searchDoc struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Section string `json:"section"`         // group header shown in the palette
	Desc    string `json:"desc,omitempty"`  // secondary line
	Ext     bool   `json:"ext,omitempty"`   // opens in a new tab
}

var (
	searchOnce  sync.Once
	searchIndex []searchDoc
)

// ShowSearchIndex serves the full navigable content of the site as a flat JSON
// list the client-side ⌘K palette fuzzy-matches against. Built once and cached;
// the content only changes on redeploy.
func ShowSearchIndex(c *gin.Context) {
	searchOnce.Do(buildSearchIndex)
	c.Header("Cache-Control", "public, max-age=3600")
	c.JSON(http.StatusOK, searchIndex)
}

func buildSearchIndex() {
	docs := []searchDoc{
		{Title: "Home", URL: "/", Section: "Pages"},
		{Title: "Blogs", URL: "/blogs", Section: "Pages"},
		{Title: "Projects", URL: "/projects", Section: "Pages"},
		{Title: "Talks", URL: "/talks", Section: "Pages"},
		{Title: "Library", URL: "/library", Section: "Pages"},
		{Title: "Gallery", URL: "/gallery", Section: "Pages"},
		{Title: "Résumé", URL: ResumeURL, Section: "Pages", Ext: true},
		{Title: "GitHub", URL: "https://github.com/Vandit1604", Section: "Elsewhere", Ext: true},
		{Title: "X / Twitter", URL: "https://x.com/v4nd1t", Section: "Elsewhere", Ext: true},
		{Title: "LinkedIn", URL: "https://www.linkedin.com/in/vandit-singh/", Section: "Elsewhere", Ext: true},
	}

	for _, b := range sortedBlogs(models.ReadBlogs()) {
		docs = append(docs, searchDoc{Title: b.Title, URL: "/blogs/" + b.Slug, Section: "Writing"})
	}

	if projects, err := readProjectYAML("content/projects.yml"); err == nil {
		for _, p := range projects {
			url := p.GithubLink
			if url == "" {
				url = p.DemoLink
			}
			docs = append(docs, searchDoc{Title: p.Name, URL: url, Section: "Projects", Desc: p.Description, Ext: url != ""})
		}
	}

	if talksData := readTalks(); talksData != nil {
		for _, t := range talksData {
			docs = append(docs, searchDoc{Title: t.Title, URL: t.VideoLink, Section: "Talks", Ext: t.VideoLink != ""})
		}
	}

	if lib, err := readLibraryYAML("content/library.yml"); err == nil {
		for _, cat := range lib.Categories {
			for _, l := range cat.Links {
				docs = append(docs, searchDoc{Title: l.Title, URL: l.URL, Section: "Library", Desc: cat.Name, Ext: true})
			}
		}
	}

	searchIndex = docs
}

// readTalks loads the talks list, returning nil on any error (the search index
// simply omits talks rather than failing the whole endpoint).
func readTalks() []types.Talk {
	yamlFile, err := os.ReadFile("content/talks.yml")
	if err != nil {
		return nil
	}
	var data struct {
		Talks []types.Talk `yaml:"talks"`
	}
	if err := yaml.Unmarshal(yamlFile, &data); err != nil {
		return nil
	}
	return data.Talks
}
