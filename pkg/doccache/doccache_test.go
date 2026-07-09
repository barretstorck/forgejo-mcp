package doccache

import (
	"fmt"
	"testing"

	"codeberg.org/goern/forgejo-mcp/v2/pkg/extract"
)

func TestCache_PutGet(t *testing.T) {
	c := New(2)
	pages := []extract.PageText{{Page: 1, Text: "x"}}
	c.Put("a|v1", pages)
	got, ok := c.Get("a|v1")
	if !ok || len(got) != 1 || got[0].Text != "x" {
		t.Fatalf("Get = %v %v", got, ok)
	}
	if _, ok := c.Get("missing"); ok {
		t.Fatal("missing key must not hit")
	}
}

func TestCache_EvictsOldest(t *testing.T) {
	c := New(2)
	for i := 0; i < 3; i++ {
		c.Put(fmt.Sprintf("k%d", i), nil)
	}
	if _, ok := c.Get("k0"); ok {
		t.Fatal("k0 should have been evicted")
	}
	if _, ok := c.Get("k2"); !ok {
		t.Fatal("k2 should be present")
	}
}
