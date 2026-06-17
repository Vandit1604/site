package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/vandit1604/site/handlers"
	"github.com/vandit1604/site/models"
	"github.com/vandit1604/site/router"
)

func main() {
	// `site -health` performs an in-process HTTP probe against /healthz and
	// exits 0/1. Used by the Docker HEALTHCHECK since the scratch image has no
	// shell or curl/wget to call.
	healthCheck := flag.Bool("health", false, "probe /healthz and exit 0 (ok) or 1 (fail)")
	// `site -indexnow` submits every sitemap URL to IndexNow and exits. Run on
	// deploy so Bing and other participating engines re-crawl promptly.
	indexNow := flag.Bool("indexnow", false, "submit all sitemap URLs to IndexNow and exit")
	flag.Parse()

	if *healthCheck {
		os.Exit(runHealthCheck())
	}

	if *indexNow {
		models.ReadBlogs() // populate blog URLs before building the submission list
		if err := handlers.SubmitToIndexNow(); err != nil {
			fmt.Fprintln(os.Stderr, "indexnow submission failed:", err)
			os.Exit(1)
		}
		fmt.Println("indexnow: submitted sitemap URLs")
		os.Exit(0)
	}

	models.ReadBlogs()
	router.Run()
}

func runHealthCheck() int {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/healthz", port))
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck failed:", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck unhealthy status:", resp.StatusCode)
		return 1
	}
	return 0
}
