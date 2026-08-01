package natsclient

import (
	"testing"
	"time"

	commonstrings "github.com/gopherust-io/nats-consol/pkg/common/strings"
)

func TestViewCacheGetOrLoadCoalesces(t *testing.T) {
	c := NewViewCache(time.Minute)
	calls := 0
	load := func() (any, error) {
		calls++
		return commonstrings.StringToBytes("ok"), nil
	}
	v1, etag1, err := c.GetOrLoad("k", load)
	if err != nil {
		t.Fatal(err)
	}
	v2, etag2, err := c.GetOrLoad("k", load)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	if commonstrings.BytesToString(v1.([]byte)) != "ok" || commonstrings.BytesToString(v2.([]byte)) != "ok" {
		t.Fatalf("unexpected values")
	}
	if commonstrings.IsEmpty(etag1) || etag1 != etag2 {
		t.Fatalf("etag mismatch %q %q", etag1, etag2)
	}
	c.InvalidateCluster("cluster")
	_, _, err = c.GetOrLoad(ViewCacheKey("cluster", "streams", "0", "10"), load)
	if err != nil {
		t.Fatal(err)
	}
	c.InvalidateCluster("cluster")
	_, _, _ = c.GetOrLoad(ViewCacheKey("cluster", "streams", "0", "10"), load)
	if calls != 3 {
		t.Fatalf("after invalidate calls = %d, want 3", calls)
	}
}
