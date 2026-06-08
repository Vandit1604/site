package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/vandit1604/site/models"
	"github.com/vandit1604/site/router"
)

func main() {
	// `site -health` performs an in-process HTTP probe against /healthz and
	// exits 0/1. Used by the Docker HEALTHCHECK since the scratch image has no
	// shell or curl/wget to call.
	healthCheck := flag.Bool("health", false, "probe /healthz and exit 0 (ok) or 1 (fail)")
	flag.Parse()

	if *healthCheck {
		os.Exit(runHealthCheck())
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
