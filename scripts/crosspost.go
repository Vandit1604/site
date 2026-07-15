// Command crosspost publishes a site blog post to dev.to straight from
// content/blogs, converting the site's HTML flourishes (callouts, figures,
// source links) to portable markdown and rewriting asset URLs to absolute
// vandit.dev links on the fly.
//
// Safe by default: with no action flag it only PREVIEWS (dry-run). Publishing
// requires an explicit -publish (live) or -draft (dev.to draft), so a new post
// is never pushed to dev.to by accident.
//
// Usage:
//
//	go run ./cmd/crosspost -list
//	go run ./cmd/crosspost -slug <slug> -tags go,devops              # preview only
//	go run ./cmd/crosspost -slug <slug> -tags go,devops -draft       # create dev.to draft
//	go run ./cmd/crosspost -slug <slug> -tags go,devops -publish     # publish live
//
// The dev.to API key comes from .env (or the environment): DEVTO_API_KEY.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	blogsDir = "content/blogs"
	base     = "https://vandit.dev"
)

type post struct {
	Slug        string
	Title       string
	Description string
	Tags        []string
	Canonical   string
	CoverImage  string
	Body        string
}

func main() {
	var (
		slug    = flag.String("slug", "", "post slug (matches content/blogs/<slug>.md)")
		tagsCSV = flag.String("tags", "", "comma-separated tags (max 4, dev.to-valid)")
		publish = flag.Bool("publish", false, "publish LIVE to dev.to (omit to preview)")
		draft   = flag.Bool("draft", false, "create a dev.to draft instead of publishing live")
		list    = flag.Bool("list", false, "list publishable post slugs and exit")
	)
	flag.Parse()
	loadDotenv(".env")

	if *list {
		for _, s := range listSlugs() {
			fmt.Println("  ", s)
		}
		return
	}
	if *slug == "" {
		fatal("usage: -slug <slug> [-tags a,b,c] [-draft|-publish] (see -list)")
	}

	p, err := loadPost(*slug, *tagsCSV)
	if err != nil {
		fatal("%v", err)
	}
	if err := publishDevto(p, *publish, *draft); err != nil {
		fatal("dev.to: %v", err)
	}
}

// ---- load + convert --------------------------------------------------------

func listSlugs() []string {
	files, _ := filepath.Glob(filepath.Join(blogsDir, "*.md"))
	var out []string
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if fm, _, ok := splitFrontmatter(string(raw)); ok && fieldVal(fm, "draft") != "true" {
			out = append(out, strings.TrimSuffix(filepath.Base(f), ".md"))
		}
	}
	return out
}

func loadPost(slug, tagsCSV string) (post, error) {
	raw, err := os.ReadFile(filepath.Join(blogsDir, slug+".md"))
	if err != nil {
		return post{}, fmt.Errorf("read post: %w", err)
	}
	fm, body, ok := splitFrontmatter(string(raw))
	if !ok {
		return post{}, fmt.Errorf("%s: missing front-matter", slug)
	}
	if fieldVal(fm, "draft") == "true" {
		return post{}, fmt.Errorf("%s is a draft; refusing to publish", slug)
	}
	tags := cleanTags(tagsCSV)
	if len(tags) == 0 {
		tags = cleanTags(fieldVal(fm, "tags"))
	}
	return post{
		Slug:        slug,
		Title:       fieldVal(fm, "title"),
		Description: fieldVal(fm, "description"),
		Tags:        tags,
		Canonical:   base + "/blogs/" + slug,
		CoverImage:  base + "/static/images/blog/og/" + slug + ".png",
		Body:        convert(body),
	}, nil
}

var (
	reFigure  = regexp.MustCompile(`(?s)<figure>.*?</figure>`)
	reImg     = regexp.MustCompile(`<img\s+src="([^"]+)"(?:\s+alt="([^"]*)")?`)
	reFigcap  = regexp.MustCompile(`(?s)<figcaption>(.*?)</figcaption>`)
	reCallout = regexp.MustCompile(`(?s)<aside class="callout[^"]*"\s+data-label="([^"]+)">(.*?)</aside>`)
	reSrcLink = regexp.MustCompile(`(?s)<a\s+class="src-link"[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	reAnchor  = regexp.MustCompile(`(?s)<a\s+[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
	reCode    = regexp.MustCompile(`(?s)<code>(.*?)</code>`)
	reStrong  = regexp.MustCompile(`(?s)<strong>(.*?)</strong>`)
	reEm      = regexp.MustCompile(`(?s)<em>(.*?)</em>`)
	reWS      = regexp.MustCompile(`\s*\n\s*`)
	reSVG     = regexp.MustCompile(`(/static/images/blog/[0-9][^)]*)\.svg\)`)
)

func abs(u string) string {
	if strings.HasPrefix(u, "/") {
		return base + u
	}
	return u
}

func inlineHTML(s string) string {
	s = reCode.ReplaceAllString(s, "`$1`")
	s = reStrong.ReplaceAllString(s, "**$1**")
	s = reEm.ReplaceAllString(s, "_${1}_")
	s = reAnchor.ReplaceAllString(s, "[$2]($1)")
	return s
}

// convert turns the site's raw-HTML flourishes into portable markdown and makes
// every asset/link URL absolute so it renders on an external platform.
func convert(body string) string {
	body = reFigure.ReplaceAllStringFunc(body, func(m string) string {
		img := reImg.FindStringSubmatch(m)
		if img == nil {
			return ""
		}
		src, alt := abs(img[1]), ""
		if len(img) > 2 {
			alt = img[2]
		}
		out := fmt.Sprintf("![%s](%s)", alt, src)
		if cap := reFigcap.FindStringSubmatch(m); cap != nil {
			out += "\n\n*" + inlineHTML(strings.TrimSpace(cap[1])) + "*"
		}
		return out
	})
	body = reCallout.ReplaceAllStringFunc(body, func(m string) string {
		sm := reCallout.FindStringSubmatch(m)
		inner := reWS.ReplaceAllString(inlineHTML(strings.TrimSpace(sm[2])), " ")
		return "> **" + sm[1] + "**\n>\n> " + strings.TrimSpace(inner)
	})
	body = reSrcLink.ReplaceAllString(body, "[$2]($1)")
	body = reAnchor.ReplaceAllString(body, "[$2]($1)")
	body = strings.ReplaceAll(body, "](/static/", "]("+base+"/static/")
	body = strings.ReplaceAll(body, "](/blogs/", "]("+base+"/blogs/")
	body = reSVG.ReplaceAllString(body, "${1}.png)") // diagrams render as PNG off-site
	return strings.TrimSpace(body)
}

// ---- dev.to ----------------------------------------------------------------

func publishDevto(p post, publish, draft bool) error {
	// Preview unless an explicit action flag is set, so nothing goes live by accident.
	if !publish && !draft {
		fmt.Printf("[preview] dev.to (pass -draft or -publish to send)\n"+
			"  title:     %s\n  tags:      %v\n  canonical: %s\n  cover:     %s\n  body:      %d chars\n",
			p.Title, p.Tags, p.Canonical, p.CoverImage, len(p.Body))
		return nil
	}
	key := os.Getenv("DEVTO_API_KEY")
	if key == "" {
		return fmt.Errorf("DEVTO_API_KEY not set")
	}
	article := map[string]any{
		"title":         p.Title,
		"body_markdown": p.Body,
		"published":     publish && !draft,
		"canonical_url": p.Canonical,
		"description":   p.Description,
		"tags":          p.Tags,
		"main_image":    p.CoverImage,
	}
	payload, _ := json.Marshal(map[string]any{"article": article})

	req, _ := http.NewRequest(http.MethodPost, "https://dev.to/api/articles", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal(b, &out)
	state := "published live"
	if draft {
		state = "draft created"
	}
	fmt.Printf("dev.to: %s -> %s\n", state, out.URL)
	return nil
}

// ---- front-matter + helpers ------------------------------------------------

func splitFrontmatter(text string) (fm, body string, ok bool) {
	if !strings.HasPrefix(text, "---\n") {
		return "", text, false
	}
	end := strings.Index(text[4:], "\n---\n")
	if end < 0 {
		return "", text, false
	}
	return text[4 : 4+end], strings.TrimLeft(text[4+end+5:], "\n"), true
}

func fieldVal(fm, key string) string {
	for _, line := range strings.Split(fm, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if ok && strings.TrimSpace(k) == key {
			return strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	return ""
}

// cleanTags normalizes a comma list into up to 4 dev.to-valid tags
// (lowercase, alphanumeric only).
func cleanTags(csv string) []string {
	csv = strings.Trim(strings.TrimSpace(csv), "[]")
	var out []string
	for _, t := range strings.Split(csv, ",") {
		t = strings.Trim(strings.TrimSpace(t), `"`)
		var b strings.Builder
		for _, r := range strings.ToLower(t) {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				b.WriteRune(r)
			}
		}
		if b.Len() > 0 {
			out = append(out, b.String())
		}
		if len(out) == 4 {
			break
		}
	}
	return out
}

func loadDotenv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if k = strings.TrimSpace(k); k != "" {
			if _, set := os.LookupEnv(k); !set {
				os.Setenv(k, strings.TrimSpace(v))
			}
		}
	}
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "crosspost: "+format+"\n", a...)
	os.Exit(1)
}
