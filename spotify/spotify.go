// Package spotify fetches the most recently played track from the Spotify
// Web API for display on the homepage.
//
// Spotify's recently-played endpoint requires a user-scoped OAuth token
// (scope: user-read-recently-played), which expires hourly. We hold a
// long-lived refresh token (obtained once, out of band) and exchange it for a
// short-lived access token on demand. Both the access token and the rendered
// track are cached in-memory so a burst of page loads makes at most one call
// to Spotify per cache window.
//
// Configuration is read from the environment. If any of the three secrets are
// missing the package is a no-op (RecentlyPlayed returns nil), so the site
// runs fine before Spotify is wired up.
package spotify

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Track is the trimmed-down view of a play the template needs.
type Track struct {
	Title    string
	Artist   string
	AlbumArt string
	URL      string
	PlayedAt time.Time
}

const (
	tokenURL    = "https://accounts.spotify.com/api/token"
	recentlyURL = "https://api.spotify.com/v1/me/player/recently-played?limit=1"
	trackTTL    = 60 * time.Second
	tokenLeeway = 30 * time.Second // refresh slightly before actual expiry
	httpTimeout = 5 * time.Second
)

var httpClient = &http.Client{Timeout: httpTimeout}

// state holds the in-memory caches, guarded by mu.
var (
	mu sync.Mutex

	cachedTrack  *Track
	trackFetched time.Time

	accessToken string
	tokenExpiry time.Time
)

// configured reports whether all required secrets are present.
func clientID() string     { return os.Getenv("SPOTIFY_CLIENT_ID") }
func clientSecret() string { return os.Getenv("SPOTIFY_CLIENT_SECRET") }
func refreshToken() string { return os.Getenv("SPOTIFY_REFRESH_TOKEN") }

func configured() bool {
	return clientID() != "" && clientSecret() != "" && refreshToken() != ""
}

// RecentlyPlayed returns the last track played, or nil if Spotify is not
// configured or the call fails. It never returns an error: a missing widget is
// better than a broken homepage. Results are cached for trackTTL.
func RecentlyPlayed() *Track {
	if !configured() {
		return nil
	}

	mu.Lock()
	defer mu.Unlock()

	if cachedTrack != nil && time.Since(trackFetched) < trackTTL {
		return cachedTrack
	}

	track, err := fetchRecentlyPlayed()
	if err != nil {
		// Serve a stale value if we have one; otherwise hide the widget.
		return cachedTrack
	}

	cachedTrack = track
	trackFetched = time.Now()
	return cachedTrack
}

// fetchRecentlyPlayed assumes mu is held.
func fetchRecentlyPlayed() (*Track, error) {
	token, err := ensureAccessToken()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, recentlyURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errStatus(resp.StatusCode)
	}

	var payload struct {
		Items []struct {
			PlayedAt time.Time `json:"played_at"`
			Track    struct {
				Name         string `json:"name"`
				ExternalURLs struct {
					Spotify string `json:"spotify"`
				} `json:"external_urls"`
				Artists []struct {
					Name string `json:"name"`
				} `json:"artists"`
				Album struct {
					Images []struct {
						URL string `json:"url"`
					} `json:"images"`
				} `json:"album"`
			} `json:"track"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if len(payload.Items) == 0 {
		return nil, nil
	}

	item := payload.Items[0]
	artists := make([]string, 0, len(item.Track.Artists))
	for _, a := range item.Track.Artists {
		artists = append(artists, a.Name)
	}

	albumArt := ""
	if len(item.Track.Album.Images) > 0 {
		// Spotify returns images largest-first; take a mid/small one.
		imgs := item.Track.Album.Images
		albumArt = imgs[len(imgs)-1].URL
	}

	return &Track{
		Title:    item.Track.Name,
		Artist:   strings.Join(artists, ", "),
		AlbumArt: albumArt,
		URL:      item.Track.ExternalURLs.Spotify,
		PlayedAt: item.PlayedAt,
	}, nil
}

// ensureAccessToken returns a valid access token, refreshing if needed.
// Assumes mu is held.
func ensureAccessToken() (string, error) {
	if accessToken != "" && time.Now().Before(tokenExpiry.Add(-tokenLeeway)) {
		return accessToken, nil
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken())

	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID(), clientSecret())

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errStatus(resp.StatusCode)
	}

	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", err
	}

	accessToken = tok.AccessToken
	tokenExpiry = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	return accessToken, nil
}

// statusError is a tiny error carrying a non-200 HTTP status.
type statusError int

func (e statusError) Error() string { return "spotify: unexpected status " + strconv.Itoa(int(e)) }

func errStatus(code int) error { return statusError(code) }
