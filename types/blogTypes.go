package types

import "html/template"

type BlogPost struct {
	Draft   string
	Slug    string
	Title   string
	Content template.HTML
	Date    string
	Tags    []string
}
