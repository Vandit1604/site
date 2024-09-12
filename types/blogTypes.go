package types

import "html/template"

type BlogPost struct {
	Slug    string
	Title   string
	Content template.HTML // Change type from string to template.HTML
	Date    string
	Tags    []string
}
