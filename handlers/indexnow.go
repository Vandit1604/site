package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// IndexNowKey is the shared secret that proves we own this host to the IndexNow
// protocol. Bing, Yandex, Seznam, Naver and other participants fetch the key
// file at /<IndexNowKey>.txt to validate any URL submission we make. A single
// submission to the aggregator endpoint fans out to every participating engine.
const IndexNowKey = "48c951f560fad1a15bc8bd641833f0be"

// indexNowEndpoint is the neutral aggregator; it forwards submissions to all
// participating search engines, so we do not POST each engine individually.
const indexNowEndpoint = "https://api.indexnow.org/indexnow"

// ShowIndexNowKey serves the key file verbatim so IndexNow endpoints can verify
// host ownership before accepting our URL submissions.
func ShowIndexNowKey(c *gin.Context) {
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(IndexNowKey))
}

// keyFileURL is the public location IndexNow fetches to validate ownership.
func keyFileURL() string { return SiteURL + "/" + IndexNowKey + ".txt" }

// waitForKeyFileLive polls the public key file until it serves our key, so a
// deploy-time submission never fires before the new image is live on the host
// (IndexNow silently rejects submissions whose key file 404s). It gives up
// after ~30s and returns an error rather than submitting into a void.
func waitForKeyFileLive() error {
	client := &http.Client{Timeout: 5 * time.Second}
	var last error
	for attempt := range 6 {
		if attempt > 0 {
			time.Sleep(5 * time.Second)
		}
		resp, err := client.Get(keyFileURL())
		if err != nil {
			last = err
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK && strings.TrimSpace(string(body)) == IndexNowKey {
			return nil
		}
		last = fmt.Errorf("key file not live yet: %s", resp.Status)
	}
	return fmt.Errorf("key file %s never became reachable: %w", keyFileURL(), last)
}

// SubmitToIndexNow pushes every sitemap URL to IndexNow in one batch so search
// engines re-crawl the site promptly instead of waiting for their own schedule.
// Intended to run on deploy via `site -indexnow`. It first confirms the key
// file is live so a submission is never wasted. Returns an error on transport
// failure or a non-2xx response.
func SubmitToIndexNow() error {
	if err := waitForKeyFileLive(); err != nil {
		return err
	}

	host := strings.TrimPrefix(SiteURL, "https://")
	payload := map[string]any{
		"host":        host,
		"key":         IndexNowKey,
		"keyLocation": keyFileURL(),
		"urlList":     AllURLs(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal indexnow payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, indexNowEndpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build indexnow request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post to indexnow: %w", err)
	}
	defer resp.Body.Close()

	// IndexNow returns 200 (accepted) or 202 (accepted, validation pending).
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("indexnow rejected submission: %s %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	return nil
}
