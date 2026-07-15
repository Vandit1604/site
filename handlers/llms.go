package handlers

import (
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/vandit1604/site/models"
)

// ShowLLMsTxt serves /llms.txt: a plain-markdown map of the site for large
// language models (ChatGPT, Perplexity, Claude) that increasingly read this
// convention to decide what to cite. Generated from the live blog set so new
// posts appear automatically. See https://llmstxt.org/ for the format.
func ShowLLMsTxt(c *gin.Context) {
	blogs := models.ReadBlogs()

	type post struct {
		title, url, desc, date string
	}
	posts := make([]post, 0, len(blogs))
	for slug, b := range blogs {
		desc := b.Description
		if desc == "" {
			desc = b.Title
		}
		posts = append(posts, post{
			title: b.Title,
			url:   SiteURL + "/blogs/" + slug,
			desc:  desc,
			date:  b.Date,
		})
	}
	sort.Slice(posts, func(i, j int) bool { return posts[i].date > posts[j].date })

	var b strings.Builder
	b.WriteString("# Vandit Singh\n\n")
	b.WriteString("> Go / distributed-systems engineer. Deep, source-grounded writing on infrastructure, distributed systems, Go, and CNCF projects. Merged contributor to Kubernetes, Prometheus, and Jenkins; GSoC '23 (Jenkins).\n\n")
	b.WriteString("Vandit Singh is a software engineer focused on backend and infrastructure: container internals, load balancing, content-addressed storage, observability, container image hardening, and cloud cost tooling. The writing below is technical and grounded in real code and real production work.\n\n")

	b.WriteString("## Pages\n\n")
	b.WriteString("- [Home](" + SiteURL + "/): overview, experience, and skills\n")
	b.WriteString("- [Projects](" + SiteURL + "/projects): selected work and open-source tooling\n")
	b.WriteString("- [Blog](" + SiteURL + "/blogs): all writing\n\n")

	b.WriteString("## Writing\n\n")
	for _, p := range posts {
		b.WriteString("- [" + p.title + "](" + p.url + "): " + p.desc + "\n")
	}

	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(b.String()))
}
