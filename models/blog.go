package models

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/vandit1604/site/types"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	goldmarkhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

func ReadBlogs() map[string]types.BlogPost {
	blogs := make(map[string]types.BlogPost)
	dir := "content/blogs/"
	files, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("Error reading directory: %v", err)
		return blogs
	}

	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
			slug := strings.TrimSuffix(f.Name(), ".md")
			filepath := filepath.Join(dir, f.Name())
			data, err := os.ReadFile(filepath)
			if err != nil {
				log.Printf("Error reading file %s: %v", f.Name(), err)
				continue
			}

			blogPost := transformDataToBlog(slug, string(data))
			blogs[slug] = *blogPost
		}
	}

	return blogs
}

func transformDataToBlog(slug, data string) *types.BlogPost {
	lines := strings.Split(data, "\n")
	var title, date string
	var tags []string
	var contentLines []string
	inFrontMatter := false
	contentStarted := false

	for _, line := range lines {
		if line == "---" {
			if !inFrontMatter {
				inFrontMatter = true
			} else {
				inFrontMatter = false
				contentStarted = true
			}
			continue
		}

		if inFrontMatter {
			if strings.HasPrefix(line, "title:") {
				title = strings.TrimSpace(strings.TrimPrefix(line, "title:"))
				title = strings.Trim(title, "\"")
			} else if strings.HasPrefix(line, "date:") {
				date = strings.TrimSpace(strings.TrimPrefix(line, "date:"))
				date = strings.Trim(date, "\"")
			} else if strings.HasPrefix(line, "tags:") {
				tagString := strings.TrimSpace(strings.TrimPrefix(line, "tags:"))
				tagString = strings.Trim(tagString, "[]")
				tags = strings.Split(tagString, ",")
				for i, tag := range tags {
					tags[i] = strings.TrimSpace(tag)
					tags[i] = strings.Trim(tags[i], "\"")
				}
			}
		} else if contentStarted {
			contentLines = append(contentLines, line)
		}
	}

	markdownContent := strings.Join(contentLines, "\n")

	// Create a new Goldmark Markdown parser with extensions and custom renderer
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Typographer,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			goldmarkhtml.WithUnsafe(),
			goldmarkhtml.WithHardWraps(),
		),
		goldmark.WithRenderer(
			renderer.NewRenderer(
				renderer.WithNodeRenderers(
					util.Prioritized(goldmarkhtml.NewRenderer(), 1000),
					util.Prioritized(newCodeBlockRenderer(), 100),
				),
			),
		),
	)

	// Convert Markdown to HTML
	var buf bytes.Buffer
	if err := md.Convert([]byte(markdownContent), &buf); err != nil {
		log.Printf("Error converting Markdown to HTML for %s: %v", slug, err)
		return nil
	}

	// Wrap the content in a div for styling purposes
	wrappedContent := fmt.Sprintf("<div class=\"markdown-content blog-content\">%s</div>", buf.String())

	blog := &types.BlogPost{
		Slug:    slug,
		Title:   title,
		Date:    date,
		Tags:    tags,
		Content: template.HTML(wrappedContent), // Use template.HTML to prevent escaping
	}

	return blog
}

type codeBlockRenderer struct{}

func newCodeBlockRenderer() renderer.NodeRenderer {
	return &codeBlockRenderer{}
}

func (r *codeBlockRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.renderCodeBlock)
}

func (r *codeBlockRenderer) renderCodeBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	n := node.(*ast.FencedCodeBlock)
	language := string(n.Language(source))
	if language == "" {
		language = "text"
	}

	// Get the code content
	var code string
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		code += string(line.Value(source))
	}

	lexer := lexers.Get(language)
	if lexer == nil {
		lexer = lexers.Fallback
		log.Printf("Using fallback lexer for language: %s", language)
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get("github")
	if style == nil {
		style = styles.Fallback
		log.Printf("Using fallback style")
	}

	formatter := html.New(html.WithClasses(true))

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		log.Printf("Error tokenizing code: %v", err)
		return ast.WalkContinue, err
	}

	w.WriteString("<pre class=\"chroma\"><code class=\"language-")
	w.WriteString(language)
	w.WriteString("\">")

	var formattedCode bytes.Buffer
	err = formatter.Format(&formattedCode, style, iterator)
	if err != nil {
		log.Printf("Error formatting code: %v", err)
		return ast.WalkContinue, err
	}

	w.Write(formattedCode.Bytes())

	w.WriteString("</code></pre>")

	return ast.WalkSkipChildren, nil
}
