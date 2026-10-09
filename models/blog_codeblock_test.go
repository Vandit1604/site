package models

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

func TestCodeBlockHasOnePre(t *testing.T) {
	md := goldmark.New(goldmark.WithRendererOptions(
		renderer.WithNodeRenderers(util.Prioritized(newCodeBlockRenderer(), 100)),
	))
	var out bytes.Buffer
	if err := md.Convert([]byte("```go\nfmt.Println(1)\n```\n"), &out); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(out.String(), "<pre"); n != 1 {
		t.Fatalf("want one <pre>, got %d: %s", n, out.String())
	}
}
