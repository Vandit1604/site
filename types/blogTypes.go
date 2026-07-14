package types

import "html/template"

type BlogPost struct {
	Draft       string
	Slug        string
	Title       string
	Content     template.HTML
	Date        string
	Tags        []string
	Description  string // optional one-line summary, used on the featured card + meta
	ReadingTime int    // estimated minutes, computed from word count
}
