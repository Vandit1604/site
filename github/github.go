// Package github fetches a small, cached snapshot of public GitHub activity
// (contribution calendar + top repositories) for the homepage. No token is
// required: the contribution calendar comes from a token-free mirror and the
// repo list from the unauthenticated REST API. Results are cached in-process
// so the homepage never hammers either endpoint (a GITHUB_TOKEN, if set, is
// used to raise the REST rate limit).
package github

import (
	"encoding/json"
	"net/http"
	"os"
	"sort"
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

func fetchContributions() (int, []Day) {
	req, _ := http.NewRequest("GET", "https://github-contributions-api.jogruber.de/v4/"+Username+"?y=last", nil)
	req.Header.Set("User-Agent", "vandit.dev")
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, nil
	}
	var body struct {
		Total         map[string]int `json:"total"`
		Contributions []Day          `json:"contributions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, nil
	}
	total := body.Total["lastYear"]
	if total == 0 {
		for _, d := range body.Contributions {
			total += d.Count
		}
	}
	return total, body.Contributions
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
