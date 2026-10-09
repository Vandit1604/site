package handlers

import (
	"slices"
	"testing"

	"github.com/vandit1604/site/types"
)

func TestGetAllTagsStableOrder(t *testing.T) {
	blogs := map[string]types.BlogPost{
		"a": {Tags: []string{"go", "infra"}},
		"b": {Tags: []string{"go", "web3"}},
		"c": {Tags: []string{"infra", "go"}},
	}
	want := []string{"go", "infra", "web3"}
	for i := 0; i < 20; i++ {
		if got := getAllTags(blogs); !slices.Equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
