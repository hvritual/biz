package persistence

import (
	"context"
	"testing"
)

func TestCE06CacheCopiesAndBound(t *testing.T) {
	c := NewMemorySnapshotCache(1)
	ctx := context.Background()
	b := []byte("one")
	_ = c.Put(ctx, "a/1/hash", b)
	b[0] = 'x'
	v, _ := c.Get(ctx, "a/1/hash")
	if string(v) != "one" {
		t.Fatal("input alias")
	}
	v[0] = 'z'
	v, _ = c.Get(ctx, "a/1/hash")
	if string(v) != "one" {
		t.Fatal("output alias")
	}
	_ = c.Put(ctx, "a/2/hash", []byte("two"))
	v, _ = c.Get(ctx, "a/1/hash")
	if len(v) != 0 {
		t.Fatal("unbounded cache")
	}
}
