// Command crosspost publishes a site blog post to dev.to (and Hashnode, when a
// Pro publication is available) straight from content/blogs, converting the
// site's HTML flourishes (callouts, figures, source links) to portable
// markdown and rewriting asset URLs to absolute vandit.dev links on the fly.
//
// dev.to works on the free API. Hashnode's GraphQL API now requires a paid Pro
// plan; without it every request 301-redirects to their announcement and the
// tool reports that clearly instead of failing silently.
//
// Usage:
//
//	go run ./cmd/crosspost -list
//	go run ./cmd/crosspost -slug <slug> -devto -dry-run
//	go run ./cmd/crosspost -slug <slug> -devto                    # publish live
//	go run ./cmd/crosspost -slug <slug> -devto -draft             # dev.to draft
//	go run ./cmd/crosspost -slug <slug> -devto -tags go,devops    # override tags
//	go run ./cmd/crosspost -slug <slug> -hashnode -at 2026-07-22T14:00:00Z
//
// Keys come from .env (or the environment): DEVTO_API_KEY, HASHNODE_TOKEN.
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
	"time"
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
		toDevto = flag.Bool("devto", false, "publish to dev.to")
		toHash  = flag.Bool("hashnode", false, "publish to Hashnode (needs Pro)")
		draft   = flag.Bool("draft", false, "create as a draft instead of publishing")
		tagsCSV = flag.String("tags", "", "comma-separated tag override (max 4, dev.to-valid)")
		at      = flag.String("at", "", "Hashnode schedule time, RFC3339 (e.g. 2026-07-22T14:00:00Z)")
		dryRun  = flag.Bool("dry-run", false, "print what would be sent, do not call the API")
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
	if *slug == "" || (!*toDevto && !*toHash) {
		fatal("usage: -slug <slug> and at least one of -devto / -hashnode (see -list)")
	}

	p, err := loadPost(*slug, *tagsCSV)
	if err != nil {
		fatal("%v", err)
	}
	if *toDevto {
		if err := publishDevto(p, *draft, *dryRun); err != nil {
			fatal("dev.to: %v", err)
		}
	}
	if *toHash {
		if err := publishHashnode(p, *at, *dryRun); err != nil {
			fatal("hashnode: %v", err)
		}
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

func publishDevto(p post, draft, dry bool) error {
	key := os.Getenv("DEVTO_API_KEY")
	if key == "" {
		return fmt.Errorf("DEVTO_API_KEY not set")
	}
	article := map[string]any{
		"title":         p.Title,
		"body_markdown": p.Body,
		"published":     !draft,
		"canonical_url": p.Canonical,
		"description":   p.Description,
		"tags":          p.Tags,
		"main_image":    p.CoverImage,
	}
	payload, _ := json.Marshal(map[string]any{"article": article})
	if dry {
		fmt.Printf("[dry-run] dev.to POST /api/articles published=%v tags=%v\n  title: %s\n  canonical: %s\n  cover: %s\n  body: %d chars\n",
			!draft, p.Tags, p.Title, p.Canonical, p.CoverImage, len(p.Body))
		return nil
	}
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
	state := "published"
	if draft {
		state = "draft created"
	}
	fmt.Printf("dev.to: %s -> %s\n", state, out.URL)
	return nil
}

// ---- Hashnode (GraphQL, Pro-gated) -----------------------------------------

var noRedirect = &http.Client{
	Timeout:       30 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func hashnodeGQL(token, query string, vars map[string]any) (json.RawMessage, error) {
	body, _ := json.Marshal(map[string]any{"query": query, "variables": vars})
	req, _ := http.NewRequest(http.MethodPost, "https://gql.hashnode.com/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)
	resp, err := noRedirect.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusMovedPermanently || resp.StatusCode == http.StatusFound {
		return nil, fmt.Errorf("API redirected (%d -> %s). Hashnode's GraphQL API now requires a Pro plan on the publication; free plans cannot use it",
			resp.StatusCode, resp.Header.Get("Location"))
	}
	raw, _ := io.ReadAll(resp.Body)
	if !json.Valid(raw) {
		return nil, fmt.Errorf("non-JSON response (status %d), likely the Pro gate or an auth problem", resp.StatusCode)
	}
	var env struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if len(env.Errors) > 0 {
		return nil, fmt.Errorf("graphql: %s", env.Errors[0].Message)
	}
	return env.Data, nil
}

func publishHashnode(p post, at string, dry bool) error {
	token := os.Getenv("HASHNODE_TOKEN")
	if token == "" {
		return fmt.Errorf("HASHNODE_TOKEN not set")
	}
	data, err := hashnodeGQL(token, `query { me { publications(first:1){ edges { node { id } } } } }`, nil)
	if err != nil {
		return err
	}
	var me struct {
		Me struct {
			Publications struct {
				Edges []struct {
					Node struct{ ID string } `json:"node"`
				} `json:"edges"`
			} `json:"publications"`
		} `json:"me"`
	}
	if err := json.Unmarshal(data, &me); err != nil {
		return err
	}
	if len(me.Me.Publications.Edges) == 0 {
		return fmt.Errorf("no publication found for this token")
	}
	pubID := me.Me.Publications.Edges[0].Node.ID

	tags := make([]map[string]string, 0, len(p.Tags))
	for _, t := range p.Tags {
		tags = append(tags, map[string]string{"slug": t, "name": capitalize(t)})
	}
	input := map[string]any{
		"publicationId":      pubID,
		"title":              p.Title,
		"contentMarkdown":    p.Body,
		"tags":               tags,
		"originalArticleURL": p.Canonical,
		"coverImageOptions":  map[string]string{"coverImageURL": p.CoverImage},
	}
	if at != "" {
		if _, err := time.Parse(time.RFC3339, at); err != nil {
			return fmt.Errorf("bad -at time %q (want RFC3339): %v", at, err)
		}
		input["publishedAt"] = at
	}
	if dry {
		fmt.Printf("[dry-run] hashnode publishPost (pub %s) tags=%v at=%q\n  title: %s\n", pubID, p.Tags, at, p.Title)
		return nil
	}
	mutation := `mutation P($input: PublishPostInput!){ publishPost(input:$input){ post { url } } }`
	out, err := hashnodeGQL(token, mutation, map[string]any{"input": input})
	if err != nil {
		return err
	}
	var res struct {
		PublishPost struct {
			Post struct{ URL string } `json:"post"`
		} `json:"publishPost"`
	}
	_ = json.Unmarshal(out, &res)
	fmt.Printf("hashnode: published -> %s\n", res.PublishPost.Post.URL)
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

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
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
