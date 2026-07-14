// Package github fetches a small, cached snapshot of public GitHub activity
// (contribution calendar + top repositories) for the homepage. No token is
// required: the contribution calendar comes from a token-free mirror and the
// repo list from the unauthenticated REST API. Results are cached in-process
// so the homepage never hammers either endpoint (a GITHUB_TOKEN, if set, is
// used to raise the REST rate limit).
package github

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Username is the GitHub account surfaced on the site.
const Username = "Vandit1604"

const cacheTTL = 6 * time.Hour

// Day is one cell of the contribution calendar.
type Day struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
	Level int    `json:"level"` // 0..4, GitHub's intensity bucket
}

// Repo is a single repository card.
type Repo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Language    string `json:"language"`
	Stars       int    `json:"stars"`
}

// Data is the cached snapshot handed to the frontend.
type Data struct {
	User  string `json:"user"`
	Total int    `json:"total"` // contributions in the last year
	Days  []Day  `json:"days"`
	Repos []Repo `json:"repos"`
}

var (
	mu        sync.Mutex
	cached    Data
	fetchedAt time.Time
	client    = &http.Client{Timeout: 6 * time.Second}
)

// Get returns the cached snapshot, refreshing it synchronously when the cache
// is empty or older than the TTL. On a fetch error it returns whatever is
// cached (possibly stale or empty) so the endpoint never fails.
func Get() Data {
	mu.Lock()
	defer mu.Unlock()
	if !fetchedAt.IsZero() && time.Since(fetchedAt) < cacheTTL {
		return cached
	}
	d := Data{User: Username}
	total, days := fetchContributions()
	d.Total, d.Days = total, days
	d.Repos = fetchRepos()
	// Keep a good prior snapshot if this refresh came back empty.
	if len(d.Days) == 0 && len(d.Repos) == 0 && (len(cached.Days) > 0 || len(cached.Repos) > 0) {
		return cached
	}
	cached, fetchedAt = d, time.Now()
	return cached
}

// Contribution-calendar parsing. We scrape GitHub's own public contributions
// endpoint rather than a third-party mirror so private-contribution counts
// (when the account opts into showing them) appear immediately instead of
// waiting on a mirror's cache. The endpoint returns HTML: a <td> per day with
// data-date + data-level, and a <tool-tip for="<cell id>"> carrying the count.
var (
	cellRe    = regexp.MustCompile(`data-date="([0-9-]+)"[^>]*id="(contribution-day-component-[0-9-]+)"[^>]*data-level="([0-4])"`)
	tooltipRe = regexp.MustCompile(`<tool-tip[^>]*for="(contribution-day-component-[0-9-]+)"[^>]*>([^<]*)</tool-tip>`)
	countRe   = regexp.MustCompile(`^([\d,]+) contribution`)
)

func fetchContributions() (int, []Day) {
	req, _ := http.NewRequest("GET", "https://github.com/users/"+Username+"/contributions", nil)
	// GitHub serves the calendar HTML to browser-like clients.
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; vandit.dev/1.0)")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, nil
	}
	html, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil
	}

	// Map cell id -> count from the tooltips ("N contribution(s) on ..." or
	// "No contributions on ..." which yields 0).
	counts := make(map[string]int)
	for _, m := range tooltipRe.FindAllStringSubmatch(string(html), -1) {
		id, text := m[1], m[2]
		n := 0
		if c := countRe.FindStringSubmatch(text); c != nil {
			n, _ = strconv.Atoi(strings.ReplaceAll(c[1], ",", ""))
		}
		counts[id] = n
	}

	cells := cellRe.FindAllStringSubmatch(string(html), -1)
	days := make([]Day, 0, len(cells))
	total := 0
	for _, m := range cells {
		date, id, level := m[1], m[2], m[3]
		lv, _ := strconv.Atoi(level)
		cnt := counts[id]
		total += cnt
		days = append(days, Day{Date: date, Count: cnt, Level: lv})
	}
	return total, days
}

func fetchRepos() []Repo {
	req, _ := http.NewRequest("GET", "https://api.github.com/users/"+Username+"/repos?per_page=100&type=owner&sort=updated", nil)
	req.Header.Set("User-Agent", "vandit.dev")
	req.Header.Set("Accept", "application/vnd.github+json")
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var raw []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		HTMLURL     string `json:"html_url"`
		Language    string `json:"language"`
		Stars       int    `json:"stargazers_count"`
		Fork        bool   `json:"fork"`
		Archived    bool   `json:"archived"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil
	}
	repos := make([]Repo, 0, len(raw))
	for _, r := range raw {
		if r.Fork || r.Archived {
			continue
		}
		repos = append(repos, Repo{r.Name, r.Description, r.HTMLURL, r.Language, r.Stars})
	}
	// Most-starred first; take the top handful for the homepage.
	sort.SliceStable(repos, func(i, j int) bool { return repos[i].Stars > repos[j].Stars })
	if len(repos) > 6 {
		repos = repos[:6]
	}
	return repos
}
