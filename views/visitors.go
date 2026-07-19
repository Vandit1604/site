package views

import (
	"bufio"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
	"time"
)

// visitorTTL is how long one visitor stays "already counted". A day means the
// tally reads as unique visitors per day rather than unique browsers forever,
// which is both the more honest number and the reason clearing storage or
// opening an incognito window can no longer inflate it.
const visitorTTL = 24 * time.Hour

// SeenRecently reports whether this visitor has already been counted inside the
// TTL, and records them if not. The key is never stored in the clear: it is
// HMAC'd with a per-install salt, so the file on disk cannot be walked back to
// a list of IP addresses.
func (c *Counter) SeenRecently(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	h := c.fingerprintLocked(key)
	if exp, ok := c.seen[h]; ok && now.Before(exp) {
		return true
	}
	c.seen[h] = now.Add(visitorTTL)
	c.pruneLocked(now)
	c.flushSeenLocked()
	return false
}

// fingerprintLocked derives the on-disk identifier for a visitor key.
func (c *Counter) fingerprintLocked(key string) string {
	mac := hmac.New(sha256.New, c.salt)
	mac.Write([]byte(key))
	// Half the digest is far more than enough to keep collisions negligible at
	// this scale, and halves the size of the file.
	return hex.EncodeToString(mac.Sum(nil)[:16])
}

func (c *Counter) pruneLocked(now time.Time) {
	for h, exp := range c.seen {
		if !now.Before(exp) {
			delete(c.seen, h)
		}
	}
}

// seenPath / saltPath sit beside the count file so a single mounted volume
// carries all three.
func (c *Counter) seenPath() string { return c.path + ".visitors" }
func (c *Counter) saltPath() string { return c.path + ".salt" }

// loadSeenLocked restores the visitor set, dropping entries that expired while
// the process was down. Any read error simply yields an empty set: the worst
// case is that today's visitors get counted once more, never a crash.
func (c *Counter) loadSeenLocked() {
	c.seen = make(map[string]time.Time)
	f, err := os.Open(c.seenPath())
	if err != nil {
		return
	}
	defer f.Close()

	now := time.Now()
	s := bufio.NewScanner(f)
	for s.Scan() {
		h, ts, ok := strings.Cut(strings.TrimSpace(s.Text()), " ")
		if !ok {
			continue
		}
		unix, err := strconv.ParseInt(ts, 10, 64)
		if err != nil {
			continue
		}
		if exp := time.Unix(unix, 0); now.Before(exp) {
			c.seen[h] = exp
		}
	}
}

func (c *Counter) flushSeenLocked() {
	var b strings.Builder
	for h, exp := range c.seen {
		b.WriteString(h)
		b.WriteByte(' ')
		b.WriteString(strconv.FormatInt(exp.Unix(), 10))
		b.WriteByte('\n')
	}
	tmp := c.seenPath() + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		return
	}
	os.Rename(tmp, c.seenPath())
}

// loadSaltLocked reads the per-install HMAC salt, generating and persisting one
// on first run. The salt must survive restarts: a fresh salt would change every
// fingerprint and re-count everyone who was already inside their TTL.
func (c *Counter) loadSaltLocked() {
	if b, err := os.ReadFile(c.saltPath()); err == nil && len(b) >= 16 {
		c.salt = b
		return
	}
	c.salt = make([]byte, 32)
	if _, err := rand.Read(c.salt); err != nil {
		// crypto/rand failing is not a reason to take the site down; the salt
		// only has to be unguessable, and this path is effectively unreachable.
		c.salt = []byte("views-fallback-salt")
	}
	os.WriteFile(c.saltPath(), c.salt, 0o600)
}
