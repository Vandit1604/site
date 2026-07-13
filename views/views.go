// Package views is a tiny persistent page-view counter. The count is a single
// integer stored as text at VIEWS_PATH so it survives container restarts when
// that path lives on a mounted volume. Deliberately a flat file, not SQLite:
// one counter needs no schema, no driver, and no CGO (the release image is
// scratch with CGO disabled).
package views

import (
	"os"
	"strconv"
	"strings"
	"sync"
)

// Counter is a goroutine-safe, file-backed view counter.
type Counter struct {
	mu    sync.Mutex
	n     int64
	path  string
	dirty bool // count advanced but the last write to disk failed
}

// New loads the counter from VIEWS_PATH (default ./views.count). A missing or
// unreadable file starts the count at zero; New never fails, so a broken volume
// degrades to an in-memory counter rather than taking the site down.
func New() *Counter {
	path := os.Getenv("VIEWS_PATH")
	if path == "" {
		path = "views.count"
	}
	c := &Counter{path: path}
	if b, err := os.ReadFile(path); err == nil {
		if v, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64); err == nil {
			c.n = v
		}
	}
	return c
}

// Increment bumps the count by one, persists it best-effort, and returns the
// new value. A failed write keeps the in-memory count moving and retries the
// flush on the next call.
func (c *Counter) Increment() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
	c.flushLocked()
	return c.n
}

// Count returns the current value without incrementing.
func (c *Counter) Count() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// flushLocked writes the count via a temp file + atomic rename so a crash mid-
// write can never leave a truncated/garbage count on disk.
func (c *Counter) flushLocked() {
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strconv.FormatInt(c.n, 10)), 0o644); err != nil {
		c.dirty = true
		return
	}
	if err := os.Rename(tmp, c.path); err != nil {
		c.dirty = true
		return
	}
	c.dirty = false
}
